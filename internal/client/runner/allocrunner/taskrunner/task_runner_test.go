package taskrunner

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/umbracle/vesta/internal/client/runner/docker"
	"github.com/umbracle/vesta/internal/client/runner/state"
	proto "github.com/umbracle/vesta/internal/client/runner/structs"
	"github.com/umbracle/vesta/internal/testutil"
)

func destroyRunner(tr *TaskRunner) {
	tr.Kill(context.Background(), proto.NewTaskEvent(""))
}

func testWaitForTaskToDie(t *testing.T, tr *TaskRunner) {
	testutil.WaitForResult(func() (bool, error) {
		ts := tr.TaskState()
		return ts.State == proto.TaskState_Dead, fmt.Errorf("expected task to be dead, got %v", ts.State)
	}, func(err error) {
		require.NoError(t, err)
	})
}

func testWaitForTaskToStart(t *testing.T, tr *TaskRunner) {
	testutil.WaitForResult(func() (bool, error) {
		ts := tr.TaskState()
		return ts.State == proto.TaskState_Running, fmt.Errorf("expected task to be running, got %v", ts.State)
	}, func(err error) {
		require.NoError(t, err)
	})
}

func setupTaskRunner(t *testing.T, task *proto.Task) *Config {
	task.Name = "test-task"
	logger := hclog.New(&hclog.LoggerOptions{Level: hclog.Debug})

	driver := docker.NewTestDockerDriver(t)

	alloc := &proto.Allocation{
		Deployment: &proto.Deployment{
			Name: "test-alloc",
			Tasks: []*proto.Task{
				task,
			},
		},
	}

	tmpDir, err := ioutil.TempDir("/tmp", "task-runner-")
	assert.NoError(t, err)

	state, err := state.NewBoltdbStore(filepath.Join(tmpDir, "my.db"))
	assert.NoError(t, err)

	assert.NoError(t, state.PutAllocation(alloc))

	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	cfg := &Config{
		Logger:           logger,
		Task:             task,
		Allocation:       alloc,
		Driver:           driver,
		State:            state,
		TaskStateUpdated: func() {},
	}

	return cfg
}

func TestTaskRunner_Stop_ExitCode(t *testing.T) {
	tt := &proto.Task{
		Image: "busybox",
		Tag:   "1.29.3",
		Args:  []string{"sleep", "3"},
	}
	r := NewTaskRunner(setupTaskRunner(t, tt))
	go r.Run()

	testWaitForTaskToStart(t, r)

	err := r.Kill(context.Background(), proto.NewTaskEvent("drop"))
	require.NoError(t, err)

	terminatedEvent := r.TaskState().Events[1]
	require.Equal(t, terminatedEvent.Type, proto.TaskTerminated)
	require.Equal(t, terminatedEvent.Details["exit_code"], "137")
}

func TestTaskRunner_Restore_AlreadyRunning(t *testing.T) {
	// Restoring a running task should not re run the task
	tt := &proto.Task{
		Image: "busybox",
		Tag:   "1.29.3",
		Args:  []string{"sleep", "3"},
		Batch: true,
	}
	cfg := setupTaskRunner(t, tt)

	oldRunner := NewTaskRunner(cfg)
	go oldRunner.Run()

	testWaitForTaskToStart(t, oldRunner)

	// stop the task runner
	oldRunner.Shutdown()

	// start another task runner with the same state
	newRunner := NewTaskRunner(cfg)

	// restore the task
	require.NoError(t, newRunner.Restore())
	defer destroyRunner(newRunner)

	go newRunner.Run()

	// wait for the process to finish
	testWaitForTaskToDie(t, newRunner)

	// assert the process only started once
	state := newRunner.TaskState()

	started := 0
	for _, ev := range state.Events {
		if ev.Type == proto.TaskStarted {
			started++
		}
	}
	assert.Equal(t, 1, started)
}

func TestTaskRunner_Restore_RequiresRestart(t *testing.T) {
	// Restore a running task that was dropped should restart
	// the task.
	tt := &proto.Task{
		Image: "busybox",
		Tag:   "1.29.3",
		Args:  []string{"sleep", "6"},
	}
	cfg := setupTaskRunner(t, tt)

	oldRunner := NewTaskRunner(cfg)
	go oldRunner.Run()

	testWaitForTaskToStart(t, oldRunner)

	// stop runner and stop the instance
	oldRunner.Shutdown()
	require.NoError(t, oldRunner.driver.DestroyTask(oldRunner.handle.Id, true))

	// restart (and restore) the runner
	newRunner := NewTaskRunner(cfg)
	require.NoError(t, newRunner.Restore())
	defer destroyRunner(newRunner)

	go newRunner.Run()

	// wait for the task to start again
	testWaitForTaskToStart(t, newRunner)

	events := newRunner.TaskState().Events
	require.Equal(t, events[0].Type, proto.TaskStarted)    // initial start
	require.Equal(t, events[1].Type, proto.TaskTerminated) // emitted during newRunner.Run
	require.Equal(t, events[2].Type, proto.TaskRestarting)
	require.Equal(t, events[3].Type, proto.TaskStarted)
}

func TestTaskRunner_Shutdown(t *testing.T) {
	// A task can be shutdown and it notifies with the
	// wait channel
	tt := &proto.Task{
		Image: "busybox",
		Tag:   "1.29.3",
		Args:  []string{"sleep", "6"},
	}
	cfg := setupTaskRunner(t, tt)

	runner := NewTaskRunner(cfg)
	go runner.Run()

	testWaitForTaskToStart(t, runner)

	runner.Shutdown()

	select {
	case <-runner.WaitCh():
	case <-time.After(5 * time.Second):
		t.Fatal("it did not notify shutdown")
	}
}

func TestTaskRunner_MountData(t *testing.T) {
	mountData := map[string]string{
		"/var/xx.txt": "data",
		"/var/yy.txt": "data2",
	}

	// we can create a task with mount data and deploy it
	tt := &proto.Task{
		Image: "busybox",
		Tag:   "1.29.3",
		Args:  []string{"sleep", "6"},
		Data:  mountData,
	}
	cfg := setupTaskRunner(t, tt)

	runner := NewTaskRunner(cfg)
	go runner.Run()

	testWaitForTaskToStart(t, runner)

	for fileName, content := range mountData {
		res, err := cfg.Driver.ExecTask(runner.handle.Id, []string{"cat", fileName})
		require.NoError(t, err)

		require.Zero(t, res.ExitCode)
		require.Equal(t, content, string(res.Stdout))
	}
}

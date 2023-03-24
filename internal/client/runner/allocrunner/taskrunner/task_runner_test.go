package taskrunner

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/umbracle/vesta/internal/client/runner/docker"
	"github.com/umbracle/vesta/internal/client/runner/state"
	"github.com/umbracle/vesta/internal/server/proto"
	"github.com/umbracle/vesta/internal/testutil"
)

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

func setupTaskRunner(t *testing.T, task *proto.Task1) *Config {
	task.Name = "test-task"
	logger := hclog.New(&hclog.LoggerOptions{Level: hclog.Debug})

	driver, err := docker.NewDockerDriver(logger)
	assert.NoError(t, err)

	alloc := &proto.Allocation1{
		Deployment: &proto.Deployment1{
			Name: "test-alloc",
			Tasks: []*proto.Task1{
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
	tt := &proto.Task1{
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
	tt := &proto.Task1{
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
	oldRunner.Close()

	// start another task runner with the same state
	newRunner := NewTaskRunner(cfg)

	// restore the task
	require.NoError(t, newRunner.Restore())

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
	tt := &proto.Task1{
		Image: "busybox",
		Tag:   "1.29.3",
		Args:  []string{"sleep", "6"},
	}
	cfg := setupTaskRunner(t, tt)

	oldRunner := NewTaskRunner(cfg)
	go oldRunner.Run()

	testWaitForTaskToStart(t, oldRunner)

	// stop runner and stop the instance
	oldRunner.Close()
	require.NoError(t, oldRunner.driver.DestroyTask(oldRunner.handle.Id, true))

	// restart (and restore) the runner
	newRunner := NewTaskRunner(cfg)
	require.NoError(t, newRunner.Restore())
	go newRunner.Run()

	// wait for the task to start again
	testWaitForTaskToStart(t, newRunner)

	events := newRunner.TaskState().Events
	require.Equal(t, events[0].Type, proto.TaskStarted)    // initial start
	require.Equal(t, events[1].Type, proto.TaskTerminated) // emitted during newRunner.Run
	require.Equal(t, events[2].Type, proto.TaskRestarting)
	require.Equal(t, events[3].Type, proto.TaskStarted)
}

func TestTaskRunner_Restart(t *testing.T) {
	// Task should restart if failed
	t.Skip("TODO")
}

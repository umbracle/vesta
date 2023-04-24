package allocrunner

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/umbracle/vesta/internal/client/runner/docker"
	"github.com/umbracle/vesta/internal/client/runner/mock"
	"github.com/umbracle/vesta/internal/client/runner/state"
	proto "github.com/umbracle/vesta/internal/client/runner/structs"
	"github.com/umbracle/vesta/internal/testutil"
)

func destroy(ar *AllocRunner) {
	ar.Destroy()
	<-ar.DestroyCh()
}

func testAllocRunnerConfig(t *testing.T, alloc *proto.Allocation) *Config {
	alloc.Deployment.Name = "mock-dep"
	logger := hclog.New(&hclog.LoggerOptions{Level: hclog.Debug})

	driver := docker.NewTestDockerDriver(t)

	tmpDir, err := ioutil.TempDir("/tmp", "task-runner-")
	assert.NoError(t, err)

	state, err := state.NewBoltdbStore(filepath.Join(tmpDir, "my.db"))
	assert.NoError(t, err)

	assert.NoError(t, state.PutAllocation(alloc))

	volumeDir, err := ioutil.TempDir("/tmp", "task-runner-volume-dir")
	require.NoError(t, err)

	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
		os.RemoveAll(volumeDir)
	})

	cfg := &Config{
		Logger:          logger,
		Driver:          driver,
		Alloc:           alloc,
		State:           state,
		StateUpdater:    &mockUpdater{},
		ClientVolumeDir: volumeDir,
	}

	return cfg
}

func TestAllocRunner_Create(t *testing.T) {
	// Deploy an allocation with multiple tasks
	alloc := mock.ServiceAlloc()
	cfg := testAllocRunnerConfig(t, alloc)

	allocRunner, err := NewAllocRunner(cfg)
	require.NoError(t, err)

	go allocRunner.Run()
	defer destroy(allocRunner)

	// wait for the allocation to be running
	updater := cfg.StateUpdater.(*mockUpdater)
	testutil.WaitForResult(func() (bool, error) {
		last := updater.Last()
		if last == nil {
			return false, errors.New("last update nil")
		}

		if last.Status != proto.Allocation_Running {
			return false, fmt.Errorf("got client status %v; want running", last.Status)
		}

		running := map[string]struct{}{}
		for taskName, s := range last.TaskStates {
			for _, e := range s.Events {
				if e.Type == proto.TaskStarted {
					running[taskName] = struct{}{}
				}
			}
		}
		if len(running) != 2 {
			return false, fmt.Errorf("two task expected running")
		}

		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})
}

func TestAllocRunner_Destroy(t *testing.T) {
	// After an allocation gets deployed, we send a terminal execution
	// and all the task should be destroyed.
	alloc := mock.ServiceAlloc()
	task := alloc.Deployment.Tasks[0]

	cfg := testAllocRunnerConfig(t, alloc)

	allocRunner, err := NewAllocRunner(cfg)
	require.NoError(t, err)

	go allocRunner.Run()

	// wait for the alloc to be running
	waitForRunningAlloc(t, cfg)

	// assert state was stored
	ts, th, err := cfg.State.GetTaskState(alloc.Deployment.Name, task.Name)
	require.NoError(t, err)
	require.NotNil(t, th)
	require.NotNil(t, ts)

	// destroy the task
	allocRunner.Destroy()

	// wait for the destroy
	select {
	case <-allocRunner.DestroyCh():
	case <-time.After(10 * time.Second):
		t.Fatal("failed to destroy")
	}

	// alloc status shoulld be dead
	state := allocRunner.AllocStatus()
	require.Equal(t, state, proto.Allocation_Complete)

	// state is cleaned
	_, _, err = cfg.State.GetTaskState(alloc.Deployment.Name, task.Name)
	require.Error(t, err)
}

func TestAllocRunner_Restore(t *testing.T) {
	// If after a restore the tasks are not running, the runner
	// has to start them again
	alloc := mock.ServiceAlloc()
	cfg := testAllocRunnerConfig(t, alloc)

	oldAllocRunner, err := NewAllocRunner(cfg)
	require.NoError(t, err)

	go oldAllocRunner.Run()

	// wait for the alloc to be running
	waitForRunningAlloc(t, cfg)

	// shutdown the alloc runner
	oldAllocRunner.Shutdown()

	select {
	case <-oldAllocRunner.ShutdownCh():
	case <-time.After(10 * time.Second):
		t.Fatal("alloc runner did not shutdown")
	}

	// destroy the tasks
	for _, task := range oldAllocRunner.tasks {
		require.NoError(t, cfg.Driver.DestroyTask(task.Handle().Id, true))
	}

	// reattach with a new alloc runner. The tasks have been destroyed
	// but the desired state is running. Thus, the alloc runner should start
	// the containers again
	newAllocRunner, err := NewAllocRunner(cfg)
	require.NoError(t, err)

	// restore the task
	require.NoError(t, newAllocRunner.Restore())

	go newAllocRunner.Run()
	defer destroy(newAllocRunner)

	updater := cfg.StateUpdater.(*mockUpdater)
	testutil.WaitForResult(func() (bool, error) {
		last := updater.Last()
		if last == nil {
			return false, fmt.Errorf("no updates")
		}
		if last.Status != proto.Allocation_Running {
			return false, fmt.Errorf("alloc not complete")
		}

		started := 0
		restarting := 0
		terminated := 0

		for _, task := range last.TaskStates {
			for _, ev := range task.Events {
				if ev.Type == proto.TaskStarted {
					started++
				}
				if ev.Type == proto.TaskRestarting {
					restarting++
				}
				if ev.Type == proto.TaskTerminated {
					terminated++
				}
			}
		}

		if started != 4 {
			return false, fmt.Errorf("expected 4 started events but found %d", started)
		}
		if restarting != 2 {
			return false, fmt.Errorf("expected 2 restarting events but found %d", restarting)
		}
		if terminated != 2 {
			return false, fmt.Errorf("expected 2 terminated events but found %d", terminated)
		}

		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})
}

func TestAllocRunner_Reattach(t *testing.T) {
	// The allocation runner has to reattach to the running tasks if
	// it gets reconnected.
	alloc := mock.ServiceAlloc()
	cfg := testAllocRunnerConfig(t, alloc)

	oldAllocRunner, err := NewAllocRunner(cfg)
	require.NoError(t, err)

	go oldAllocRunner.Run()

	// wait for the alloc to be running
	waitForRunningAlloc(t, cfg)

	// shutdown the alloc runner
	oldAllocRunner.Shutdown()

	select {
	case <-oldAllocRunner.ShutdownCh():
	case <-time.After(10 * time.Second):
		t.Fatal("alloc runner did not shutdown")
	}

	// restart the tasks in another allocRunner
	newAllocRunner, err := NewAllocRunner(cfg)
	require.NoError(t, err)

	// restore the task
	require.NoError(t, newAllocRunner.Restore())

	go newAllocRunner.Run()
	defer destroy(newAllocRunner)

	// it should not create any new tasks
	updater := cfg.StateUpdater.(*mockUpdater)
	testutil.WaitForResult(func() (bool, error) {
		last := updater.Last()
		if last == nil {
			return false, fmt.Errorf("no updates")
		}
		if last.Status != proto.Allocation_Running {
			return false, fmt.Errorf("alloc not complete")
		}

		running := 0
		for _, task := range last.TaskStates {
			for _, ev := range task.Events {
				if ev.Type == proto.TaskStarted {
					running++
				}
			}
		}
		if running != 2 {
			return false, fmt.Errorf("expected only two started events")
		}

		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})
}

func TestAllocRunner_Update_CreateTask(t *testing.T) {
	// from a running allocation, create an extra task
	alloc := mock.ServiceAlloc()
	cfg := testAllocRunnerConfig(t, alloc)

	allocRunner, err := NewAllocRunner(cfg)
	require.NoError(t, err)

	go allocRunner.Run()
	defer destroy(allocRunner)

	// wait for the alloc to be running
	waitForRunningAlloc(t, cfg)

	// create an extra task
	dep := alloc.Deployment.Copy()
	newTask := dep.Tasks[0].Copy()
	newTask.Name = "c"
	dep.Tasks = append(dep.Tasks, newTask)

	allocRunner.Update(dep)

	// wait until the allocation is complete
	updater := cfg.StateUpdater.(*mockUpdater)
	testutil.WaitForResult(func() (bool, error) {
		last := updater.Last()
		if last == nil {
			return false, fmt.Errorf("no updates")
		}
		if last.Status != proto.Allocation_Running {
			return false, fmt.Errorf("alloc not complete")
		}

		// there has to be 3 running tasks with Started events
		running := map[string]struct{}{}
		for taskName, s := range last.TaskStates {
			if s.State != proto.TaskState_Running {
				return false, fmt.Errorf("task '%s' not running", taskName)
			}

			for _, e := range s.Events {
				if e.Type == proto.TaskStarted {
					running[taskName] = struct{}{}
				}
			}
		}
		if len(running) != 3 {
			return false, fmt.Errorf("two task expected running")
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestAllocRunner_Update_UpdateTask(t *testing.T) {
	// update one of the tasks in the deployment
	alloc := mock.ServiceAlloc()
	cfg := testAllocRunnerConfig(t, alloc)

	allocRunner, err := NewAllocRunner(cfg)
	require.NoError(t, err)

	go allocRunner.Run()
	defer destroy(allocRunner)

	var taskAId string

	// wait for the alloc to be running
	updater := cfg.StateUpdater.(*mockUpdater)
	testutil.WaitForResult(func() (bool, error) {
		last := updater.Last()
		if last == nil {
			return false, fmt.Errorf("no updates")
		}
		if last.Status != proto.Allocation_Running {
			return false, fmt.Errorf("alloc not running")
		}

		// record the id for the "a" task which will get updated
		taskAId = last.TaskStates["a"].Id

		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})

	// drop the tasks
	dep := alloc.Deployment.Copy()
	dep.Tasks[0].Args = []string{"sleep", "40"}

	allocRunner.Update(dep)

	// wait until the allocation is running and there
	// is a new task "a"
	testutil.WaitForResult(func() (bool, error) {
		last := updater.Last()
		if last == nil {
			return false, fmt.Errorf("no updates")
		}
		if last.Status != proto.Allocation_Running {
			return false, fmt.Errorf("alloc not running")
		}

		updatedTask, ok := last.TaskStates["a"]
		if !ok {
			return false, fmt.Errorf("updated task 'a' not found")
		}
		if updatedTask.Id == taskAId {
			return false, fmt.Errorf("updated task 'a' not replaced")
		}
		if updatedTask.State != proto.TaskState_Running {
			return false, fmt.Errorf("updated task 'a' not running")
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestAllocRunner_Update_DestroyTask(t *testing.T) {
	// with multiple deployments one is deleted
	alloc := mock.ServiceAlloc()
	cfg := testAllocRunnerConfig(t, alloc)

	allocRunner, err := NewAllocRunner(cfg)
	require.NoError(t, err)

	go allocRunner.Run()
	defer destroy(allocRunner)

	// wait for the alloc to be running
	waitForRunningAlloc(t, cfg)

	// drop the tasks
	dep := alloc.Deployment.Copy()
	dep.Tasks = []*proto.Task{dep.Tasks[0]}

	allocRunner.Update(dep)

	// wait until the allocation is complete
	updater := cfg.StateUpdater.(*mockUpdater)
	testutil.WaitForResult(func() (bool, error) {
		last := updater.Last()
		if last == nil {
			return false, fmt.Errorf("no updates")
		}
		if last.Status != proto.Allocation_Running {
			return false, fmt.Errorf("alloc not running")
		}

		// only task 'a' is running
		if len(last.TaskStates) != 1 {
			return false, fmt.Errorf("only one task expected but found %d", len(last.TaskStates))
		}
		state, ok := last.TaskStates["a"]
		if !ok {
			return false, fmt.Errorf("task 'a' not found")
		}
		if state.State != proto.TaskState_Running {
			return false, fmt.Errorf("task 'a' not running %s", state.State)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestAllocRunner_UpdateDestroyAllocation(t *testing.T) {
	// if we update the desired status of the allocation to 'stop',
	// the allocation and all the tasks should stop gracefully

	alloc := mock.ServiceAlloc()
	cfg := testAllocRunnerConfig(t, alloc)

	allocRunner, err := NewAllocRunner(cfg)
	require.NoError(t, err)

	go allocRunner.Run()
	// defer destroy(allocRunner)

	// wait for the alloc to be running
	waitForRunningAlloc(t, cfg)

	stopDeployment := alloc.Copy().Deployment
	stopDeployment.DesiredStatus = proto.Deployment_Stop

	allocRunner.Update(stopDeployment)
	<-allocRunner.waitCh

	// Wait for all tasks to stop
	updater := cfg.StateUpdater.(*mockUpdater)
	testutil.WaitForResult(func() (bool, error) {
		last := updater.Last()

		if last.Status != proto.Allocation_Complete {
			return false, fmt.Errorf("alloc not completed")
		}

		for name, t := range last.TaskStates {
			if t.State != proto.TaskState_Dead {
				return false, fmt.Errorf("task '%s' is not dead", name)
			}
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("error waiting for initial state:\n%v", err)
	})
}

func TestAllocRunner_PortConflict(t *testing.T) {
	// if two tasks on the same deployment try to listen on
	// the same port, one will fail since they share the
	// same networking spec.
	bindArgs := []string{"nc", "-l", "-p", "3000", "0.0.0.0"}

	alloc := mock.ServiceAlloc()
	alloc.Deployment.Tasks[0].Args = bindArgs
	alloc.Deployment.Tasks[1].Args = bindArgs

	cfg := testAllocRunnerConfig(t, alloc)

	allocRunner, err := NewAllocRunner(cfg)
	require.NoError(t, err)

	go allocRunner.Run()
	defer destroy(allocRunner)

	updater := cfg.StateUpdater.(*mockUpdater)
	testutil.WaitForResult(func() (bool, error) {
		last := updater.Last()
		if last == nil {
			return false, fmt.Errorf("no updates")
		}

		// wait for one of the task to restart
		restarting := 0
		for _, s := range last.TaskStates {
			for _, e := range s.Events {
				if e.Type == proto.TaskRestarting {
					restarting++
				}
			}
		}
		if restarting != 1 {
			return false, fmt.Errorf("restarting expected")
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestAllocRunner_VolumeMount(t *testing.T) {
	// one of the tasks mounts a volume
	// 1. Deploy the tasks (volume not found, create it)
	// 2. Touch a file in the folder
	// 3. Restart the task and check the touched file

	alloc := mock.ServiceAlloc()
	alloc.Deployment.Tasks[0].Volumes = map[string]*proto.Task_Volume{
		"data": {
			Path: "/data1",
		},
	}

	cfg := testAllocRunnerConfig(t, alloc)

	oldAllocRunner, err := NewAllocRunner(cfg)
	require.NoError(t, err)

	go oldAllocRunner.Run()

	// wait for the alloc to be running
	waitForRunningAlloc(t, cfg)

	handleID := oldAllocRunner.tasks["a"].Handle().Id
	res, err := cfg.Driver.ExecTask(handleID, []string{
		"touch", "/data1/file.txt",
	})
	require.NoError(t, err)
	require.Zero(t, res.ExitCode, 0)

	oldAllocRunner.Shutdown()
	<-oldAllocRunner.ShutdownCh()

	// destroy the tasks
	for _, task := range oldAllocRunner.tasks {
		require.NoError(t, cfg.Driver.DestroyTask(task.Handle().Id, true))
	}

	// restart with a new allocation
	newAllocRunner, err := NewAllocRunner(cfg)
	require.NoError(t, err)

	// restore the task
	require.NoError(t, newAllocRunner.Restore())

	go newAllocRunner.Run()
	defer destroy(newAllocRunner)

	// wait for the other two tasks to be started since when
	// it gets restored it starts in Running state (its old state before stopping)
	updater := cfg.StateUpdater.(*mockUpdater)
	testutil.WaitForResult(func() (bool, error) {
		last := updater.Last()
		if last == nil {
			return false, fmt.Errorf("no updates")
		}
		if last.Status != proto.Allocation_Running {
			return false, fmt.Errorf("alloc not complete")
		}

		started := 0

		for _, task := range last.TaskStates {
			for _, ev := range task.Events {
				if ev.Type == proto.TaskStarted {
					started++
				}
			}
		}

		if started != 4 {
			return false, fmt.Errorf("expected 4 started events but found %d", started)
		}

		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})

	handleID = newAllocRunner.tasks["a"].Handle().Id
	fmt.Println(handleID)

	res, err = cfg.Driver.ExecTask(handleID, []string{"cat", "/data1/file.txt"})
	require.NoError(t, err)
	require.Empty(t, res.Stderr)
	require.Empty(t, res.Stdout)
}

type mockUpdater struct {
	alloc *proto.Allocation
	lock  sync.Mutex
}

func (m *mockUpdater) AllocStateUpdated(alloc *proto.Allocation) {
	m.lock.Lock()
	m.alloc = alloc
	m.lock.Unlock()
}

func (m *mockUpdater) Last() *proto.Allocation {
	m.lock.Lock()
	alloc := m.alloc
	m.lock.Unlock()
	return alloc
}

func TestClientStatus(t *testing.T) {
	cases := []struct {
		states map[string]*proto.TaskState
		status proto.Allocation_Status
	}{
		{
			map[string]*proto.TaskState{
				"a": {State: proto.TaskState_Running},
			},
			proto.Allocation_Running,
		},
		{
			map[string]*proto.TaskState{
				"a": {State: proto.TaskState_Pending},
				"b": {State: proto.TaskState_Running},
			},
			proto.Allocation_Pending,
		},
	}

	for _, c := range cases {
		require.Equal(t, c.status, getClientStatus(c.states))
	}
}

func waitForRunningAlloc(t *testing.T, cfg *Config) {
	updater := cfg.StateUpdater.(*mockUpdater)
	testutil.WaitForResult(func() (bool, error) {
		last := updater.Last()
		if last == nil {
			return false, fmt.Errorf("no updates")
		}
		if last.Status != proto.Allocation_Running {
			return false, fmt.Errorf("alloc not running")
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

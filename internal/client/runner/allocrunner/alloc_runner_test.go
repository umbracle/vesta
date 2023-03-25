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
	"github.com/umbracle/vesta/internal/client/runner/state"
	"github.com/umbracle/vesta/internal/mock"
	"github.com/umbracle/vesta/internal/server/proto"
	"github.com/umbracle/vesta/internal/testutil"
)

func destroy(ar *AllocRunner) {
	ar.Destroy()
	<-ar.DestroyCh()
}

func testAllocRunnerConfig(t *testing.T, alloc *proto.Allocation1) *Config {
	alloc.Deployment.Name = "mock-dep"
	logger := hclog.New(&hclog.LoggerOptions{Level: hclog.Debug})

	driver, err := docker.NewDockerDriver(logger)
	assert.NoError(t, err)

	tmpDir, err := ioutil.TempDir("/tmp", "task-runner-")
	assert.NoError(t, err)

	state, err := state.NewBoltdbStore(filepath.Join(tmpDir, "my.db"))
	assert.NoError(t, err)

	assert.NoError(t, state.PutAllocation(alloc))

	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	cfg := &Config{
		Logger:       logger,
		Driver:       driver,
		Alloc:        alloc,
		State:        state,
		StateUpdater: &mockUpdater{},
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

	// wait for the allocation to be running
	updater := cfg.StateUpdater.(*mockUpdater)
	testutil.WaitForResult(func() (bool, error) {
		last := updater.Last()
		if last == nil {
			return false, errors.New("last update nil")
		}

		if last.Status != proto.Allocation1_Running {
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
	testutil.WaitForResult(func() (bool, error) {
		state := allocRunner.AllocStatus()

		return state == proto.Allocation1_Running,
			fmt.Errorf("got client status %v; want running", state)
	}, func(err error) {
		require.NoError(t, err)
	})

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
	require.Equal(t, state, proto.Allocation1_Complete)

	// state is cleaned
	_, _, err = cfg.State.GetTaskState(alloc.Deployment.Name, task.Name)
	require.Error(t, err)
}

func TestAllocRunner_Restore(t *testing.T) {
	// The allocation runner has to restore the running tasks if
	// it gets reconnected.
	alloc := mock.ServiceAlloc()
	cfg := testAllocRunnerConfig(t, alloc)

	oldAllocRunner, err := NewAllocRunner(cfg)
	require.NoError(t, err)

	go oldAllocRunner.Run()

	// wait for the alloc to be running
	testutil.WaitForResult(func() (bool, error) {
		state := oldAllocRunner.AllocStatus()

		return state == proto.Allocation1_Running,
			fmt.Errorf("got client status %v; want running", state)
	}, func(err error) {
		require.NoError(t, err)
	})

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

	// it should not create any new tasks
	updater := cfg.StateUpdater.(*mockUpdater)
	testutil.WaitForResult(func() (bool, error) {
		last := updater.Last()
		if last == nil {
			return false, fmt.Errorf("no updates")
		}
		if last.Status != proto.Allocation1_Running {
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

	// wait for the alloc to be running
	testutil.WaitForResult(func() (bool, error) {
		state := allocRunner.AllocStatus()

		return state == proto.Allocation1_Running,
			fmt.Errorf("got client status %v; want running", state)
	}, func(err error) {
		require.NoError(t, err)
	})

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
		if last.Status != proto.Allocation1_Running {
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

	var taskAId string

	// wait for the alloc to be running
	updater := cfg.StateUpdater.(*mockUpdater)
	testutil.WaitForResult(func() (bool, error) {
		last := updater.Last()
		if last == nil {
			return false, fmt.Errorf("no updates")
		}
		if last.Status != proto.Allocation1_Running {
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
		if last.Status != proto.Allocation1_Running {
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

	// wait for the alloc to be running
	testutil.WaitForResult(func() (bool, error) {
		state := allocRunner.AllocStatus()

		return state == proto.Allocation1_Running,
			fmt.Errorf("got client status %v; want running", state)
	}, func(err error) {
		require.NoError(t, err)
	})

	// drop the tasks
	dep := alloc.Deployment.Copy()
	dep.Tasks = []*proto.Task1{dep.Tasks[0]}

	allocRunner.Update(dep)

	// wait until the allocation is complete
	updater := cfg.StateUpdater.(*mockUpdater)
	testutil.WaitForResult(func() (bool, error) {
		last := updater.Last()
		if last == nil {
			return false, fmt.Errorf("no updates")
		}
		if last.Status != proto.Allocation1_Running {
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

type mockUpdater struct {
	alloc *proto.Allocation1
	lock  sync.Mutex
}

func (m *mockUpdater) AllocStateUpdated(alloc *proto.Allocation1) {
	m.lock.Lock()
	m.alloc = alloc
	m.lock.Unlock()
}

func (m *mockUpdater) Last() *proto.Allocation1 {
	m.lock.Lock()
	alloc := m.alloc
	m.lock.Unlock()
	return alloc
}

func TestClientStatus(t *testing.T) {
	cases := []struct {
		states map[string]*proto.TaskState
		status proto.Allocation1_Status
	}{
		{
			map[string]*proto.TaskState{
				"a": {State: proto.TaskState_Running},
			},
			proto.Allocation1_Running,
		},
		{
			map[string]*proto.TaskState{
				"a": {State: proto.TaskState_Pending},
				"b": {State: proto.TaskState_Running},
			},
			proto.Allocation1_Pending,
		},
	}

	for _, c := range cases {
		require.Equal(t, c.status, getClientStatus(c.states))
	}
}

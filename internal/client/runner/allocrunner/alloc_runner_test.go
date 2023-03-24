package allocrunner

import (
	"fmt"
	"io/ioutil"
	"math/rand"
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
	"github.com/umbracle/vesta/internal/server/proto"
	"github.com/umbracle/vesta/internal/testutil"
	"github.com/umbracle/vesta/internal/uuid"
)

func destroy(ar *AllocRunner) {
	ar.Shutdown()
	<-ar.ShutdownCh()
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

func TestAllocRunner_SimpleRun(t *testing.T) {
	alloc := &proto.Allocation1{
		Deployment: &proto.Deployment1{
			Tasks: []*proto.Task1{
				{
					Image: "busybox",
					Tag:   "1.29.3",
					Args:  []string{"sleep", "10"},
				},
			},
		},
	}
	cfg := testAllocRunnerConfig(t, alloc)

	allocRunner, err := NewAllocRunner(cfg)
	require.NoError(t, err)

	defer destroy(allocRunner)
	go allocRunner.Run()

	updater := cfg.StateUpdater.(*mockUpdater)
	testutil.WaitForResult(func() (bool, error) {
		last := updater.Last()
		if last == nil {
			return false, fmt.Errorf("no updates")
		}
		if last.Status != proto.Allocation1_Running {
			return false, fmt.Errorf("alloc not running")
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestAllocRunner_MultipleRun(t *testing.T) {
	// An allocation can run more than one tasks
	alloc := &proto.Allocation1{
		Deployment: &proto.Deployment1{
			Tasks: []*proto.Task1{
				{
					Name:  "a",
					Image: "busybox",
					Tag:   "1.29.3",
					Args:  []string{"sleep", "30"},
				},
				{
					Name:  "b",
					Image: "busybox",
					Tag:   "1.29.3",
					Args:  []string{"sleep", "30"},
				},
			},
		},
	}

	cfg := testAllocRunnerConfig(t, alloc)

	allocRunner, err := NewAllocRunner(cfg)
	require.NoError(t, err)

	go allocRunner.Run()

	// wait for the allocation to be running
	updater := cfg.StateUpdater.(*mockUpdater)
	testutil.WaitForResult(func() (bool, error) {
		last := updater.Last()
		if last == nil {
			return false, fmt.Errorf("no updates")
		}
		if last.Status != proto.Allocation1_Running {
			return false, fmt.Errorf("alloc not running")
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// drop the tasks
	go allocRunner.Destroy()

	// wait until the allocation is complete
	testutil.WaitForResult(func() (bool, error) {
		last := updater.Last()
		if last == nil {
			return false, fmt.Errorf("no updates")
		}
		if last.Status != proto.Allocation1_Complete {
			return false, fmt.Errorf("alloc not complete")
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestAllocRunner_Update(t *testing.T) {
	alloc := &proto.Allocation1{
		Deployment: &proto.Deployment1{
			Tasks: []*proto.Task1{
				{
					Image: "busybox",
					Tag:   "1.29.3",
					Args:  []string{"sleep", "30"},
				},
			},
		},
	}
	cfg := testAllocRunnerConfig(t, alloc)

	allocRunner, err := NewAllocRunner(cfg)
	require.NoError(t, err)

	go allocRunner.Run()

	updater := cfg.StateUpdater.(*mockUpdater)
	testutil.WaitForResult(func() (bool, error) {
		last := updater.Last()
		if last == nil {
			return false, fmt.Errorf("no updates")
		}
		if last.Status != proto.Allocation1_Running {
			return false, fmt.Errorf("alloc not running")
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// update the args
	dep1 := alloc.Deployment.Copy()
	dep1.Tasks[0].Args = []string{"sleep", "35"}

	allocRunner.Update(dep1)

	/*
		testutil.WaitForResult(func() (bool, error) {
			last := updater.Last()
			if last == nil {
				return false, fmt.Errorf("no updates")
			}
			if last.Status != proto.Allocation1_Running {
				return false, fmt.Errorf("alloc not running")
			}

			events := last.TaskStates["a"]
			fmt.Println(events)

			return true, nil
		}, func(err error) {
			t.Fatalf("err: %v", err)
		})
	*/

	time.Sleep(10 * time.Second)

	last := updater.Last()
	fmt.Println(last.TaskStates[""])

	//fmt.Println("- post -")
	//fmt.Println(allocRunner.tasks)
}

func TestAllocRunner_TerminalState(t *testing.T) {
	// After an allocation gets deployed, we send a terminal execution
	// and all the task should be destroyed.
	t.Skip("TODO") // kind of done.
}

func TestAllocRunner_Restore(t *testing.T) {
	// The allocation runner has to restore the running tasks if
	// it gets reconnected.
	t.Skip("TODO")
}

func TestAllocRunner_PartialCreate(t *testing.T) {
	// with multiple deployments create a new one

	alloc := &proto.Allocation1{
		Deployment: &proto.Deployment1{
			Name: "mock-dep",
			Tasks: []*proto.Task1{
				{
					Name:  "a",
					Image: "busybox",
					Tag:   "1.29.3",
					Args:  []string{"sleep", "30"},
				},
				{
					Name:  "b",
					Image: "busybox",
					Tag:   "1.29.3",
					Args:  []string{"sleep", "30"},
				},
			},
		},
	}

	cfg := testAllocRunnerConfig(t, alloc)

	allocRunner, err := NewAllocRunner(cfg)
	require.NoError(t, err)

	go allocRunner.Run()

	// wait for the allocation to be running
	updater := cfg.StateUpdater.(*mockUpdater)
	testutil.WaitForResult(func() (bool, error) {
		last := updater.Last()
		if last == nil {
			return false, fmt.Errorf("no updates")
		}
		if last.Status != proto.Allocation1_Running {
			return false, fmt.Errorf("alloc not running")
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// drop the tasks
	dep1 := alloc.Deployment
	dep1.Tasks = append(dep1.Tasks, &proto.Task1{
		Name:  "c",
		Image: "busybox",
		Tag:   "1.29.3",
		Args:  []string{"sleep", "30"},
	})

	allocRunner.Update(dep1)

	/*
		// wait until the allocation is complete
		testutil.WaitForResult(func() (bool, error) {
			last := updater.Last()
			if last == nil {
				return false, fmt.Errorf("no updates")
			}
			if last.Status != proto.Allocation1_Complete {
				return false, fmt.Errorf("alloc not complete")
			}

			return true, nil
		}, func(err error) {
			t.Fatalf("err: %v", err)
		})
	*/

	time.Sleep(10 * time.Second)
}

func TestAllocRunner_PartialUpdate(t *testing.T) {
	// with multiple deployments one is updated

	alloc := &proto.Allocation1{
		Deployment: &proto.Deployment1{
			Name: "mock-dep",
			Tasks: []*proto.Task1{
				{
					Name:  "a",
					Image: "busybox",
					Tag:   "1.29.3",
					Args:  []string{"sleep", "30"},
				},
				{
					Name:  "b",
					Image: "busybox",
					Tag:   "1.29.3",
					Args:  []string{"sleep", "30"},
				},
			},
		},
	}

	cfg := testAllocRunnerConfig(t, alloc)

	allocRunner, err := NewAllocRunner(cfg)
	require.NoError(t, err)

	go allocRunner.Run()

	// wait for the allocation to be running
	updater := cfg.StateUpdater.(*mockUpdater)
	testutil.WaitForResult(func() (bool, error) {
		last := updater.Last()
		if last == nil {
			return false, fmt.Errorf("no updates")
		}
		if last.Status != proto.Allocation1_Running {
			return false, fmt.Errorf("alloc not running")
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// drop the tasks
	dep1 := alloc.Deployment
	dep1.Tasks[0].Args = []string{"sleep", "40"}

	allocRunner.Update(dep1)

	/*
		// wait until the allocation is complete
		testutil.WaitForResult(func() (bool, error) {
			last := updater.Last()
			if last == nil {
				return false, fmt.Errorf("no updates")
			}
			if last.Status != proto.Allocation1_Complete {
				return false, fmt.Errorf("alloc not complete")
			}

			return true, nil
		}, func(err error) {
			t.Fatalf("err: %v", err)
		})
	*/

	time.Sleep(10 * time.Second)
}

func TestAllocRunner_PartialDestroy(t *testing.T) {
	// with multiple deployments one is deleted
	alloc := &proto.Allocation1{
		Deployment: &proto.Deployment1{
			Name: "mock-dep",
			Tasks: []*proto.Task1{
				{
					Name:  "a",
					Image: "busybox",
					Tag:   "1.29.3",
					Args:  []string{"sleep", "30"},
				},
				{
					Name:  "b",
					Image: "busybox",
					Tag:   "1.29.3",
					Args:  []string{"sleep", "30"},
				},
			},
		},
	}

	cfg := testAllocRunnerConfig(t, alloc)

	allocRunner, err := NewAllocRunner(cfg)
	require.NoError(t, err)

	go allocRunner.Run()

	// wait for the allocation to be running
	updater := cfg.StateUpdater.(*mockUpdater)
	testutil.WaitForResult(func() (bool, error) {
		last := updater.Last()
		if last == nil {
			return false, fmt.Errorf("no updates")
		}
		if last.Status != proto.Allocation1_Running {
			return false, fmt.Errorf("alloc not running")
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// drop the tasks
	dep1 := &proto.Deployment1{
		Name: "mock-dep",
		Tasks: []*proto.Task1{
			alloc.Deployment.Tasks[0],
		},
	}
	allocRunner.Update(dep1)

	/*
		// wait until the allocation is complete
		testutil.WaitForResult(func() (bool, error) {
			last := updater.Last()
			if last == nil {
				return false, fmt.Errorf("no updates")
			}
			if last.Status != proto.Allocation1_Complete {
				return false, fmt.Errorf("alloc not complete")
			}

			return true, nil
		}, func(err error) {
			t.Fatalf("err: %v", err)
		})
	*/

	time.Sleep(5 * time.Second)
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

func TestAllocRunner_Fuzz(t *testing.T) {
	deps := genFuzzActions(2)

	fmt.Println("######")

	// initial deployment
	cfg := testAllocRunnerConfig(t, &proto.Allocation1{
		Deployment: deps[0],
	})

	allocRunner, err := NewAllocRunner(cfg)
	require.NoError(t, err)

	go allocRunner.Run()
	time.Sleep(1 * time.Second)

	defer func() {
		fmt.Println("=>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>> WAITING FOR SHUTDOWN")
		// destroy
		allocRunner.Shutdown()
		<-allocRunner.ShutdownCh()
	}()

	timer := time.NewTimer(2 * time.Second)

	// initial alloc
	for index, dep := range deps[1:] {
		fmt.Println("ITEM => ", index, dep)

		timer.Reset(2 * time.Second)
		go func() {
			<-timer.C
			panic("cxx")
		}()

		allocRunner.Update(dep)

		time.Sleep(1 * time.Second)

	}
}

func randomInt(min, max int) int {
	return min + rand.Intn(max-min)
}

func randomType(typs ...string) string {
	return typs[rand.Intn(len(typs))]
}

func genFuzzActions(num int) []*proto.Deployment1 {
	dep := &proto.Deployment1{
		Name: "mock-dep",
		Tasks: []*proto.Task1{
			longRunningTask(),
		},
	}

	deps := []*proto.Deployment1{
		dep,
	}

	// rand.Seed(time.Now().UTC().UnixNano())

	for i := 0; i < num; i++ {
		newDep := dep.Copy()

	RETRY:
		typ := randomType("add", "del", "update")
		if typ == "add" {
			// add new item to the deployment
			tt := longRunningTask()
			newDep.Tasks = append(newDep.Tasks, tt)

			fmt.Println("ADD => ", tt)

		} else if typ == "del" {
			// remove item from the deployment
			if len(newDep.Tasks) == 1 {
				goto RETRY
			}

			indx := rand.Intn(len(newDep.Tasks))
			newDep.Tasks = append(newDep.Tasks[:indx], newDep.Tasks[indx+1:]...)

			fmt.Println("DEL => ", indx)

		} else if typ == "update" {
			// pick a random item and update
			if len(newDep.Tasks) == 0 {
				goto RETRY
			}

			indx := rand.Intn(len(newDep.Tasks))

			newTask := newDep.Tasks[indx].Copy()
			newTask.Args = []string{
				"sleep", fmt.Sprintf("%d3000", i),
			}

			fmt.Println("UPDATE => ", newTask)

			newDep.Tasks[indx] = newTask
		}

		fmt.Println(newDep)

		deps = append(deps, newDep)
		dep = newDep
	}

	return deps
}

func longRunningTask() *proto.Task1 {
	return &proto.Task1{
		Name:  uuid.Generate(),
		Image: "busybox",
		Tag:   "1.29.3",
		Args:  []string{"sleep", "3000"},
	}
}

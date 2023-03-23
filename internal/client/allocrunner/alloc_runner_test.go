package allocrunner

import (
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
	"github.com/umbracle/vesta/internal/client/allocrunner/docker"
	"github.com/umbracle/vesta/internal/client/allocrunner/state"
	"github.com/umbracle/vesta/internal/server/proto"
	"github.com/umbracle/vesta/internal/testutil"
)

func testAllocRunnerConfig(t *testing.T, alloc *proto.Allocation) *Config {
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
	alloc := &proto.Allocation{
		Id: "a",
		Deployment: &proto.Deployment{
			Tasks: map[string]*proto.Task{
				"a": {
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
		if last.Status != proto.Allocation_Running {
			return false, fmt.Errorf("alloc not running")
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestAllocRunner_Update(t *testing.T) {
	alloc := &proto.Allocation{
		Id: "a",
		Deployment: &proto.Deployment{
			Tasks: map[string]*proto.Task{
				"a": {
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
		if last.Status != proto.Allocation_Running {
			return false, fmt.Errorf("alloc not running")
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// update the args
	alloc1 := alloc.Copy()
	alloc1.Sequence += 1
	alloc1.Deployment.Tasks["a"].Args = []string{"sleep", "35"}

	allocRunner.Update(alloc1)

	/*
		testutil.WaitForResult(func() (bool, error) {
			last := updater.Last()
			if last == nil {
				return false, fmt.Errorf("no updates")
			}
			if last.Status != proto.Allocation_Running {
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

	fmt.Println("- post -")
	fmt.Println(allocRunner.tasks)
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

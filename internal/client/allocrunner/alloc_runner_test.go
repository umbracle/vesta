package allocrunner

import (
	"fmt"
	"sync"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/umbracle/vesta/internal/client/state"
	"github.com/umbracle/vesta/internal/docker"
	"github.com/umbracle/vesta/internal/server/proto"
	"github.com/umbracle/vesta/internal/testutil"
	"github.com/umbracle/vesta/internal/uuid"
)

func destroy(ar *AllocRunner) {
	//ar.Destroy()
	//<-ar.DestroyCh()
}

func TestAllocRunner_(t *testing.T) {
	alloc := mockAllocation()

	alloc.Deployment.Tasks["first"] = &proto.Task{
		Image: "busybox",
		Tag:   "1.29.3",
		Args:  []string{"nc", "-l", "-p", "3000", "127.0.0.1"},
	}
	alloc.Deployment.Tasks["second"] = &proto.Task{
		Image: "busybox",
		Tag:   "1.29.3",
		Args:  []string{"nc", "-l", "-p", "3001", "127.0.0.1"},
	}

	config := testAllocRunnerConfig(t, alloc)

	ar, err := NewAllocRunner(config)
	require.NoError(t, err)
	go ar.Run()
	defer destroy(ar)

	upd := config.StateUpdater.(*MockStateUpdater)
	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}
		for name, state := range last.TaskStates {
			if state.State != proto.TaskState_Running {
				return false, fmt.Errorf("Task %q is not running yet (it's %q)", name, state.State)
			}
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func testAllocRunnerConfig(t *testing.T, alloc *proto.Allocation) *Config {
	driver, err := docker.NewDockerDriver(hclog.NewNullLogger())
	if err != nil {
		t.Fatal(err)
	}

	state := state.NewInmemStore(t)
	assert.NoError(t, state.PutAllocation(alloc))

	c := &Config{
		Logger:       hclog.NewNullLogger(),
		Alloc:        alloc,
		StateUpdater: &MockStateUpdater{},
		Driver:       driver,
		State:        state,
	}
	return c
}

func mockAllocation() *proto.Allocation {
	a := &proto.Allocation{
		Id:         uuid.Generate(),
		Deployment: &proto.Deployment{},
	}
	return a
}

type MockStateUpdater struct {
	Updates []*proto.Allocation
	mu      sync.Mutex
}

func (m *MockStateUpdater) AllocStateUpdated(alloc *proto.Allocation) {
	m.mu.Lock()
	m.Updates = append(m.Updates, alloc)
	m.mu.Unlock()
}

func (m *MockStateUpdater) Last() *proto.Allocation {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := len(m.Updates)
	if n == 0 {
		return nil
	}
	return m.Updates[n-1]
}

func (m *MockStateUpdater) Reset() {
	m.mu.Lock()
	m.Updates = nil
	m.mu.Unlock()
}

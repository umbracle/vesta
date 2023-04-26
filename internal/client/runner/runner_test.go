package runner

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
	"github.com/umbracle/vesta/internal/client/runner/mock"
	"github.com/umbracle/vesta/internal/client/runner/state"
	proto "github.com/umbracle/vesta/internal/client/runner/structs"
	"github.com/umbracle/vesta/internal/testutil"
)

func newTestRunner(t *testing.T) *Runner {
	config := &Config{
		Logger:            hclog.New(&hclog.LoggerOptions{Level: hclog.Info}),
		State:             state.NewInmemStore(t),
		AllocStateUpdated: &updater{},
	}

	r, err := NewRunner(config)
	require.NoError(t, err)

	time.Sleep(1 * time.Second)
	return r
}

type updater struct {
	alloc *proto.Allocation
}

func (u *updater) AllocStateUpdated(alloc *proto.Allocation) {
	u.alloc = alloc
}

func TestRunner_SimpleRun(t *testing.T) {
	r := newTestRunner(t)
	updater := r.config.AllocStateUpdated.(*updater)

	dep := mock.ServiceAlloc().Deployment
	dep.Name = "a"

	r.UpsertDeployment(dep)

	testutil.WaitForResult(func() (bool, error) {
		last := updater.alloc
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

	dep1 := dep.Copy()
	dep1.Sequence = dep.Sequence + 1
	dep1.Tasks = []*proto.Task{dep.Tasks[0]}

	r.UpsertDeployment(dep1)

	testutil.WaitForResult(func() (bool, error) {
		last := updater.alloc
		if last == nil {
			return false, fmt.Errorf("no updates")
		}
		if last.Status != proto.Allocation_Running {
			return false, fmt.Errorf("alloc not running")
		}
		if len(last.TaskStates) != 1 {
			return false, fmt.Errorf("only one task expected")
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	dep2 := dep1.Copy()
	dep2.Sequence = dep1.Sequence + 1
	dep2.DesiredStatus = proto.Deployment_Stop

	r.UpsertDeployment(dep2)

	testutil.WaitForResult(func() (bool, error) {
		last := updater.alloc
		if last == nil {
			return false, fmt.Errorf("no updates")
		}
		if last.Status != proto.Allocation_Complete {
			return false, fmt.Errorf("alloc not complete")
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

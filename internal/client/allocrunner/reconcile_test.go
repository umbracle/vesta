package allocrunner

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/umbracle/vesta/internal/mock"
	"github.com/umbracle/vesta/internal/server/proto"
)

func TestReconcile(t *testing.T) {
	t0 := &proto.Task{
		Id: "id0",
	}
	t1 := &proto.Task{
		Id: "id1",
	}

	alloc := &proto.Allocation{
		Deployment: &proto.Deployment{
			Tasks: map[string]*proto.Task{
				"t0": t0,
				"t1": t1,
			},
		},
	}

	tasks := map[string]*proto.Task{}

	taskState := map[string]*proto.TaskState{}

	pendingDelete := map[string]struct{}{}

	r := newAllocReconciler(alloc, tasks, taskState, pendingDelete)

	res := r.Compute()
	fmt.Println(res)
}

func TestTaskUpdated(t *testing.T) {
	t1 := mock.Task()
	t2 := mock.Task()

	require.False(t, tasksUpdated(t1, t2))

	t2.Args = []string{"c"}
	require.True(t, tasksUpdated(t1, t2))

	t2 = mock.Task()
	t2.Env = map[string]string{"c": "d"}
	require.True(t, tasksUpdated(t1, t2))
}

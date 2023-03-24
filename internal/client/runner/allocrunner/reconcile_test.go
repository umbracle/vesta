package allocrunner

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/umbracle/vesta/internal/mock"
	"github.com/umbracle/vesta/internal/server/proto"
)

func TestAllocResultsEmpty(t *testing.T) {
	t.Skip("TODO")
}

func TestReconcile(t *testing.T) {
	alloc := &proto.Allocation1{
		Deployment: &proto.Deployment1{
			Tasks: []*proto.Task1{
				{Name: "t0"},
				{Name: "t1"},
			},
		},
	}

	tasks := map[string]*proto.Task1{}

	taskState := map[string]*proto.TaskState{}

	pendingDelete := map[string]struct{}{}

	r := newAllocReconciler(alloc, tasks, taskState, pendingDelete)

	res := r.Compute()
	fmt.Println(res)
}

func TestTaskUpdated(t *testing.T) {
	t1 := mock.Task1()
	t2 := mock.Task1()

	require.False(t, tasksUpdated(t1, t2))

	t2.Args = []string{"c"}
	require.True(t, tasksUpdated(t1, t2))

	t2 = mock.Task1()
	t2.Env = map[string]string{"c": "d"}
	require.True(t, tasksUpdated(t1, t2))
}

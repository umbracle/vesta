package allocrunner

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/umbracle/vesta/internal/mock"
)

func TestAllocResultsEmpty(t *testing.T) {
	cases := []struct {
		res   *allocResults
		empty bool
	}{
		{
			&allocResults{},
			true,
		},
		{
			&allocResults{
				removeTasks: []string{"a", "b"},
			},
			false,
		},
	}

	for _, c := range cases {
		require.Equal(t, c.empty, c.res.Empty())
	}
}

type expectedResults struct {
	newTasks int
	delTasks int
}

func checkReconcile(t *testing.T, res *allocResults, expected expectedResults) {
	t.Helper()

	require.Len(t, res.newTasks, expected.newTasks)
	require.Len(t, res.removeTasks, expected.delTasks)
}

func TestTaskUpdated(t *testing.T) {
	t1 := mock.Task1()
	t2 := mock.Task1()

	require.False(t, tasksUpdated(t1, t2))

	t2 = mock.Task1()
	t2.Image = "bad-image"
	require.True(t, tasksUpdated(t1, t2))

	t2 = mock.Task1()
	t2.Tag = "bad-tag"
	require.True(t, tasksUpdated(t1, t2))

	t2 = mock.Task1()
	t2.Args = []string{"c"}
	require.True(t, tasksUpdated(t1, t2))

	t2 = mock.Task1()
	t2.Env = map[string]string{"c": "d"}
	require.True(t, tasksUpdated(t1, t2))

	t2 = mock.Task1()
	t2.Labels = map[string]string{"c": "d"}
	require.True(t, tasksUpdated(t1, t2))
}

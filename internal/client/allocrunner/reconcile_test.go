package allocrunner

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/umbracle/vesta/internal/mock"
)

func TestReconcile(t *testing.T) {
	// TODO
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

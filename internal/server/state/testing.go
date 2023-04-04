package state

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func NewInmemStore(t *testing.T) *StateStore {
	tmpDir, err := ioutil.TempDir("/tmp", "task-runner-")
	require.NoError(t, err)

	state, err := NewStateStore(filepath.Join(tmpDir, "my.db"))
	require.NoError(t, err)

	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	return state
}

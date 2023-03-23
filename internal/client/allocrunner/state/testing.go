package state

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func NewInmemStore(t *testing.T) *BoltdbStore {
	tmpDir, err := ioutil.TempDir("/tmp", "task-runner-")
	assert.NoError(t, err)

	state, err := NewBoltdbStore(filepath.Join(tmpDir, "my.db"))
	assert.NoError(t, err)

	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	return state
}

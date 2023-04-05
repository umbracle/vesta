package allocdir

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllocDir(t *testing.T) {
	tmpDir, err := ioutil.TempDir("/tmp", "alloc-dir-test-")
	assert.NoError(t, err)

	a0 := NewAllocDir(tmpDir, "alloc")
	t0 := a0.NewTaskDir("a")

	require.NoError(t, t0.Build())
	require.NoError(t, a0.Build())

	volDir := filepath.Join(tmpDir, "alloc", "a", "volumes")
	require.DirExists(t, volDir)

	// write a file and build again, it should
	// not override any files
	err = ioutil.WriteFile(filepath.Join(volDir, "file.txt"), []byte{}, 0655)
	require.NoError(t, err)

	a1 := NewAllocDir(tmpDir, "alloc")
	a1.NewTaskDir("a")
	require.NoError(t, a1.Build())

	require.FileExists(t, filepath.Join(volDir, "file.txt"))
}

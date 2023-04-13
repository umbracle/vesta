package server

import (
	"path/filepath"
	"testing"

	"github.com/boltdb/bolt"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
	"github.com/umbracle/vesta/internal/server/proto"
)

type dummyCatalog struct {
	tasks map[string]*proto.Task
}

func (d *dummyCatalog) Build(req *proto.ApplyRequest, input map[string]interface{}) (map[string]*proto.Task, error) {
	// this is enough to generate an allocation
	return d.tasks, nil
}

func TestCreate(t *testing.T) {
	// test that we can create and update an allocation
	tmpDir := t.TempDir()

	db, err := bolt.Open(filepath.Join(tmpDir, "path.db"), 0755, nil)
	require.NoError(t, err)

	cfg := &Config{PersistentDB: db}
	catalog := &dummyCatalog{}

	srv, _ := NewServer(hclog.NewNullLogger(), cfg)
	srv.catalog = catalog

	catalog.tasks = map[string]*proto.Task{}
	catalog.tasks["a"] = &proto.Task{
		Args: []string{"a"},
	}

	allocid, err := srv.Create(&proto.ApplyRequest{}, nil)
	require.NoError(t, err)

	// the allocation should be on the state with sequence=0
	alloc, err := srv.state.GetAllocation(allocid)
	require.NoError(t, err)
	require.Equal(t, alloc.Id, allocid)
	require.Equal(t, alloc.Sequence, int64(0))
	require.Len(t, alloc.Tasks, 1)

	// update the allocation
	catalog.tasks["b"] = &proto.Task{
		Args: []string{"b"},
	}

	allocid2, err := srv.Create(&proto.ApplyRequest{AllocationId: allocid}, nil)
	require.NoError(t, err)
	require.Equal(t, allocid, allocid2)

	// the allocation shoulld be on the state with sequence=1
	alloc, err = srv.state.GetAllocation(allocid)
	require.NoError(t, err)
	require.Equal(t, alloc.Sequence, int64(1))
	require.Len(t, alloc.Tasks, 2)
}

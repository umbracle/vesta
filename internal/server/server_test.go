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
	createTask *proto.Task
}

func (d *dummyCatalog) Build(req *proto.ApplyRequest) (map[string]*proto.Task, error) {
	// this is enough to generate an allocation
	return map[string]*proto.Task{"task": d.createTask}, nil
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

	catalog.createTask = &proto.Task{
		Args: []string{"a"},
	}

	allocid, err := srv.Create(&proto.ApplyRequest{})
	require.NoError(t, err)

	// the allocation should be on the state with sequence=0
	alloc, err := srv.state.GetAllocation(allocid)
	require.NoError(t, err)
	require.Equal(t, alloc.Id, allocid)
	require.Equal(t, alloc.Sequence, int64(0))
	require.Equal(t, alloc.Tasks["task"].Args, []string{"a"})

	// update the allocation
	catalog.createTask = &proto.Task{
		Args: []string{"b"},
	}

	allocid2, err := srv.Create(&proto.ApplyRequest{AllocationId: allocid})
	require.NoError(t, err)
	require.Equal(t, allocid, allocid2)

	// the allocation shoulld be on the state with sequence=1
	alloc, err = srv.state.GetAllocation(allocid)
	require.NoError(t, err)
	require.Equal(t, alloc.Sequence, int64(1))
	require.Equal(t, alloc.Tasks["task"].Args, []string{"b"})
}

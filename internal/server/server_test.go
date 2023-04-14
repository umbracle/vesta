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
	prev       []byte
	createTask *proto.Task
}

func (d *dummyCatalog) Build(prev []byte, req *proto.ApplyRequest) ([]byte, map[string]*proto.Task, error) {
	// this is enough to generate an allocation
	d.prev = prev
	return req.Input, map[string]*proto.Task{"task": d.createTask}, nil
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

	input := []byte{0x1, 0x2, 0x3}

	allocid, err := srv.Create(&proto.ApplyRequest{Input: input})
	require.NoError(t, err)

	// 'prev' is empty since there was no previous state
	require.Empty(t, catalog.prev)

	// the allocation should be on the state with sequence=0
	alloc, err := srv.state.GetAllocation(allocid)
	require.NoError(t, err)
	require.Equal(t, alloc.Id, allocid)
	require.Equal(t, alloc.Sequence, int64(0))
	require.Equal(t, alloc.Tasks["task"].Args, []string{"a"})
	require.Equal(t, alloc.InputState, input)

	// update the allocation
	catalog.createTask = &proto.Task{
		Args: []string{"b"},
	}

	input2 := []byte{0x4, 0x5, 0x6}

	allocid2, err := srv.Create(&proto.ApplyRequest{Input: input2, AllocationId: allocid})
	require.NoError(t, err)
	require.Equal(t, allocid, allocid2)
	require.Equal(t, catalog.prev, input)

	// the allocation shoulld be on the state with sequence=1
	alloc, err = srv.state.GetAllocation(allocid)
	require.NoError(t, err)
	require.Equal(t, alloc.Sequence, int64(1))
	require.Equal(t, alloc.Tasks["task"].Args, []string{"b"})
	require.Equal(t, alloc.InputState, input2)
}

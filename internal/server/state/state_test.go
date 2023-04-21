package state

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/go-memdb"
	"github.com/stretchr/testify/require"
	"github.com/umbracle/vesta/internal/server/proto"
)

func TestState_Allocation_InsertGet(t *testing.T) {
	state := NewInmemStore(t)

	alloc := &proto.Allocation{
		Id:     "a",
		NodeId: "local",
	}
	require.NoError(t, state.UpsertAllocation(alloc))

	alloc1, err := state.GetAllocation("a")
	require.NoError(t, err)
	require.NotNil(t, alloc1)
}

func TestState_Allocation_GetByNode(t *testing.T) {
	state := NewInmemStore(t)

	ids := []string{"a", "b", "c"}
	for _, id := range ids {
		alloc := &proto.Allocation{
			Id:     id,
			NodeId: "local",
		}
		require.NoError(t, state.UpsertAllocation(alloc))
	}

	ws := memdb.NewWatchSet()
	allocs, err := state.AllocationListByNodeId("local", ws)
	require.NoError(t, err)
	require.Len(t, allocs, 3)
}

func TestState_Allocation_Destroy(t *testing.T) {
	state := NewInmemStore(t)

	alloc := &proto.Allocation{
		Id:     "a",
		NodeId: "local",
	}

	require.NoError(t, state.UpsertAllocation(alloc))

	// destroy allocation increases the sequence and
	// updates the desired state. Note, we could abstract this
	// on the server itself?
	require.NoError(t, state.DestroyAllocation("a"))

	alloc1, err := state.GetAllocation("a")
	require.NoError(t, err)
	require.Equal(t, alloc1.DesiredStatus, proto.Allocation_Stop)
	require.Equal(t, alloc1.Sequence, alloc.Sequence+1)
}

func TestState_Allocation_Persistence(t *testing.T) {
	tmpDir, err := ioutil.TempDir("/tmp", "task-runner-")
	require.NoError(t, err)

	boldDBPath := filepath.Join(tmpDir, "my.db")

	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	state1, err := NewStateStore(boldDBPath)
	require.NoError(t, err)

	alloc := &proto.Allocation{
		Id:     "a",
		NodeId: "local",
	}
	require.NoError(t, state1.UpsertAllocation(alloc))

	// mount the state again
	state1.Close()

	state2, err := NewStateStore(boldDBPath)
	require.NoError(t, err)

	alloc1, err := state2.GetAllocation("a")
	require.NoError(t, err)
	require.NotNil(t, alloc1)
}

func TestStateStore_AllocationByIDPrefix(t *testing.T) {
	state := NewInmemStore(t)

	alloc0 := &proto.Allocation{
		Id:     "ab",
		NodeId: "local",
	}
	alloc1 := &proto.Allocation{
		Id:     "ac",
		NodeId: "local",
	}

	require.NoError(t, state.UpsertAllocation(alloc0))
	require.NoError(t, state.UpsertAllocation(alloc1))

	allocs, err := state.AllocationsByIDPrefix("a")
	require.NoError(t, err)
	require.Len(t, allocs, 2)
}

func TestStateStore_InsertWithAlias(t *testing.T) {
	state := NewInmemStore(t)

	// cannot get an empty alias
	obj, err := state.AllocationByAlias("a")
	require.NoError(t, err)
	require.Empty(t, obj)

	// multiple allocations can have an empty alias
	alloc0 := &proto.Allocation{Id: "ab", NodeId: "b"}
	alloc1 := &proto.Allocation{Id: "ac", NodeId: "b"}

	require.NoError(t, state.UpsertAllocation(alloc0))
	require.NoError(t, state.UpsertAllocation(alloc1))

	// update the alias of the allocation
	alloc0.Alias = "a"
	require.NoError(t, state.UpsertAllocation(alloc0))

	obj, err = state.AllocationByAlias("a")
	require.NoError(t, err)
	require.Equal(t, obj.Id, "ab")

	// cannot be two allocations with the same alias
	alloc1.Alias = "a"
	require.Error(t, state.UpsertAllocation(alloc1))
}

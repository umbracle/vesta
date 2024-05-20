package server

import (
	"fmt"
	"testing"

	"github.com/umbracle/vesta/internal/catalog"
	"github.com/umbracle/vesta/internal/server/proto"
	"github.com/umbracle/vesta/internal/server/state"
)

/*
import (
	"path/filepath"
	"testing"

	"github.com/boltdb/bolt"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
	"github.com/umbracle/vesta/internal/framework"
	"github.com/umbracle/vesta/internal/server/proto"
)

type dummyCatalog struct {
	prev       []byte
	createTask *proto.Task
}

func (d *dummyCatalog) Build(prev []byte, req *proto.ApplyRequest) ([]byte, *proto.Service, error) {
	// this is enough to generate an allocation
	d.prev = prev
	return req.Input, &proto.Service{Tasks: map[string]*proto.Task{"task": d.createTask}}, nil
}

func (d *dummyCatalog) ListPlugins() []string {
	return nil
}

func (d *dummyCatalog) GetFields(id string, input []byte) (*framework.FieldData, error) {
	return nil, nil
}

func (d *dummyCatalog) GetPlugin(name string) (*proto.Item, error) {
	return nil, nil
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
*/

func TestServer_Deploy(t *testing.T) {
	s := &Server{
		state: state.NewInmemStore(t),
		catalog: &mockBackend{
			item: &proto.Item{
				Name: "test",
				Fields: []*proto.Item_Field{
					{
						Name: "key",
						Type: "string",
					},
				},
			},
			buildFn: func(data *catalog.FieldData) *proto.Service {
				return &proto.Service{
					Tasks: map[string]*proto.Task{
						"task": {
							Image: data.GetString("key"),
							Args:  []string{"a"},
						},
					},
				}
			},
		},
	}

	_, err := s.Create(&proto.ApplyRequest{
		Action: "",
		Input:  map[string]interface{}{"key": "value"},
	})
	fmt.Println(err)
}

type mockBackend struct {
	item    *proto.Item
	buildFn func(data *catalog.FieldData) *proto.Service
}

func (m *mockBackend) GetFields(id string, input []byte) (*catalog.FieldData, error) {
	panic("TODO: implement")
}

func (m *mockBackend) Build2(name string, data *catalog.FieldData) *proto.Service {
	return m.buildFn(data)
}

func (m *mockBackend) Build(prev []byte, req *proto.ApplyRequest) (*catalog.FieldData, *proto.Service, error) {
	panic("TODO: implement")
}

func (m *mockBackend) ListPlugins() []string {
	panic("TODO: implement")
}

func (m *mockBackend) GetPlugin(name string) (*proto.Item, error) {
	return m.item, nil
}

func (m *mockBackend) ValidateFn(plugin string, validationFn string, config, obj interface{}) bool {
	panic("TODO: implement")
}

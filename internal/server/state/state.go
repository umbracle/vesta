package state

import (
	"github.com/hashicorp/go-memdb"
	"github.com/umbracle/vesta/internal/server/proto"
)

type StateStore struct {
	db *memdb.MemDB
}

func NewStateStore() *StateStore {
	db, err := memdb.NewMemDB(schema)
	if err != nil {
		panic(err)
	}
	s := &StateStore{
		db: db,
	}
	return s
}

func (s *StateStore) AllocationList(nodeId string, ws memdb.WatchSet) ([]*proto.Allocation, error) {
	txn := s.db.Txn(false)
	defer txn.Abort()

	iter, err := txn.Get("allocations", "nodeId", nodeId)
	if err != nil {
		return nil, err
	}
	tasks := []*proto.Allocation{}
	for obj := iter.Next(); obj != nil; obj = iter.Next() {
		tasks = append(tasks, obj.(*proto.Allocation))
	}

	ws.Add(iter.WatchCh())
	return tasks, nil
}

func (s *StateStore) UpsertAllocation(t *proto.Allocation) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	if err := txn.Insert("allocations", t); err != nil {
		return err
	}

	txn.Commit()
	return nil
}

func (s *StateStore) InsertDeployment(n *proto.Deployment) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	if err := txn.Insert("deployments", n); err != nil {
		return err
	}

	txn.Commit()
	return nil
}

func (s *StateStore) GetDeployment(id string) (*proto.Deployment, error) {
	txn := s.db.Txn(false)
	defer txn.Abort()

	item, err := txn.First("node", "id", id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, nil
	}
	return item.(*proto.Deployment), nil
}

func (s *StateStore) DeploymentsList(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)
	defer txn.Abort()

	iter, err := txn.Get("deployments", "id")
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())
	return iter, nil
}

func (s *StateStore) UpsertCatalog(item *proto.Item) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	if err := txn.Insert("item", item); err != nil {
		return err
	}

	txn.Commit()
	return nil
}

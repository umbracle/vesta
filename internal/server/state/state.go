package state

import (
	"fmt"

	"github.com/boltdb/bolt"
	"github.com/hashicorp/go-memdb"
	"github.com/umbracle/vesta/internal/server/proto"
	gproto "google.golang.org/protobuf/proto"
)

// list of buckets
var (
	allocationBucket = []byte("allocation")
)

type StateStore struct {
	memDb *memdb.MemDB

	// db is the persistence layer
	db *bolt.DB
}

func NewStateStore(path string) (*StateStore, error) {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("could not open db, %v", err)
	}

	// init buckets
	err = db.Update(func(tx *bolt.Tx) error {
		bkts := [][]byte{
			allocationBucket,
		}

		for _, b := range bkts {
			if _, err := tx.CreateBucketIfNotExists(b); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	memDb, err := memdb.NewMemDB(schema)
	if err != nil {
		panic(err)
	}
	s := &StateStore{
		db:    db,
		memDb: memDb,
	}

	if err := s.reIndex(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *StateStore) Close() error {
	return s.db.Close()
}

func (s *StateStore) reIndex() error {
	memTxn := s.memDb.Txn(true)
	defer memTxn.Abort()

	// re-index allocations
	err := s.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(allocationBucket)

		return bkt.ForEach(func(k, v []byte) error {
			var alloc proto.Allocation
			if err := dbGet(bkt, k, &alloc); err != nil {
				return err
			}

			if err := memTxn.Insert("allocations", &alloc); err != nil {
				return err
			}
			return nil
		})
	})
	if err != nil {
		return err
	}

	memTxn.Commit()
	return nil
}

func (s *StateStore) GetAllocation(id string) (*proto.Allocation, error) {
	txn := s.memDb.Txn(false)
	defer txn.Abort()

	item, err := txn.First("allocations", "id", id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, nil
	}
	return item.(*proto.Allocation), nil
}

func (s *StateStore) AllocationList(ws memdb.WatchSet) ([]*proto.Allocation, error) {
	txn := s.memDb.Txn(false)
	defer txn.Abort()

	iter, err := txn.Get("allocations", "id")
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

func (s *StateStore) AllocationListByNodeId(nodeId string, ws memdb.WatchSet) ([]*proto.Allocation, error) {
	txn := s.memDb.Txn(false)
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

func (s *StateStore) DestroyAllocation(id string) error {
	memTxn := s.memDb.Txn(true)
	defer memTxn.Abort()

	// get the allocation
	item, err := memTxn.First("allocations", "id", id)
	if err != nil {
		return err
	}
	if item == nil {
		return fmt.Errorf("allocation not found")
	}

	alloc := item.(*proto.Allocation).Copy()
	alloc.Sequence++
	alloc.DesiredStatus = proto.Allocation_Stop

	err = s.db.Update(func(dbTxn *bolt.Tx) error {
		return s.putAllocation(dbTxn, memTxn, alloc)
	})
	if err == nil {
		memTxn.Commit()
	}

	return nil
}

func (s *StateStore) UpsertAllocation(alloc *proto.Allocation) error {
	memTxn := s.memDb.Txn(true)
	defer memTxn.Abort()

	err := s.db.Update(func(dbTxn *bolt.Tx) error {
		return s.putAllocation(dbTxn, memTxn, alloc)
	})
	if err == nil {
		memTxn.Commit()
	}

	return err
}

func (s *StateStore) putAllocation(dbTxn *bolt.Tx, memTxn *memdb.Txn, alloc *proto.Allocation) error {
	bkt := dbTxn.Bucket(allocationBucket)

	if err := dbPut(bkt, []byte(alloc.Id), alloc); err != nil {
		return err
	}
	if err := memTxn.Insert("allocations", alloc); err != nil {
		return err
	}
	return nil
}

func dbPut(b *bolt.Bucket, id []byte, msg gproto.Message) error {
	enc, err := gproto.Marshal(msg)
	if err != nil {
		return err
	}

	if err := b.Put(id, enc); err != nil {
		return err
	}
	return nil
}

func dbGet(b *bolt.Bucket, id []byte, msg gproto.Message) error {
	raw := b.Get(id)
	if raw == nil {
		return fmt.Errorf("record not found")
	}

	if err := gproto.Unmarshal(raw, msg); err != nil {
		return fmt.Errorf("failed to decode data: %v", err)
	}
	return nil
}

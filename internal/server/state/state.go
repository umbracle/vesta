package state

import (
	"database/sql"
	"fmt"

	"github.com/boltdb/bolt"
	"github.com/hashicorp/go-memdb"
	"github.com/umbracle/vesta/internal/server/proto"
	gproto "google.golang.org/protobuf/proto"

	_ "github.com/mattn/go-sqlite3"
)

// list of buckets
var (
	allocationBucket = []byte("allocation")
	deploymentBucket = []byte("deployment")
	volumesBkt       = []byte("volumes")
)

type StateStore struct {
	memDb *memdb.MemDB

	// db is the persistence layer
	db *bolt.DB

	db2 *sql.DB
}

func NewStateStore(path string) (*StateStore, error) {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("could not open db, %v", err)
	}

	return NewStateStoreWithBoltDB(db)
}

func NewStateStoreWithBoltDB(db *bolt.DB) (*StateStore, error) {
	// init buckets
	err := db.Update(func(tx *bolt.Tx) error {
		bkts := [][]byte{
			allocationBucket,
			deploymentBucket,
			volumesBkt,
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

	/*
		// start sqlite db with foreign index checks
		db2, err := sql.Open("sqlite3", ":memory:")
		if err != nil {
			return nil, err
		}
		s.db2 = db2

		// apply the schema
		if _, err := db2.Exec(schemaDB); err != nil {
			return nil, err
		}
	*/

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

func (s *StateStore) AllocationByAlias(alias string) (*proto.Allocation, error) {
	txn := s.memDb.Txn(false)
	defer txn.Abort()

	return s.allocationByAliasImpl(txn, alias)
}

func (s *StateStore) allocationByAliasImpl(txn *memdb.Txn, alias string) (*proto.Allocation, error) {
	val, err := txn.First("allocations", "alias", alias)
	if err != nil {
		return nil, err
	}
	if val == nil {
		return nil, nil
	}
	return val.(*proto.Allocation), nil
}

func (s *StateStore) AllocationByAliasOrIDOrPrefix(id string) (*proto.Allocation, error) {
	// try to resolve first by alias
	obj, err := s.AllocationByAlias(id)
	if err != nil {
		return nil, err
	}
	if obj != nil {
		return obj, nil
	}

	// try to resolve by id or prefix
	allocs, err := s.AllocationsByIDPrefix(id)
	if err != nil {
		return nil, err
	}
	if len(allocs) == 0 {
		return nil, fmt.Errorf("no allocations found with id or prefix '%s'", id)
	}
	if len(allocs) != 1 {
		return nil, fmt.Errorf("more than one allocation found with prefix")
	}
	return allocs[0], nil
}

func (s *StateStore) AllocationsByIDPrefix(prefix string) ([]*proto.Allocation, error) {
	txn := s.memDb.Txn(false)
	defer txn.Abort()

	iter, err := txn.Get("allocations", "id_prefix", prefix)
	if err != nil {
		return nil, err
	}

	allocs := []*proto.Allocation{}
	for obj := iter.Next(); obj != nil; obj = iter.Next() {
		allocs = append(allocs, obj.(*proto.Allocation))
	}
	return allocs, nil
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

func (s *StateStore) GetVolume(id string) (*proto.Volume, error) {
	var volume proto.Volume
	err := s.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(volumesBkt)

		return dbGet(bkt, []byte(id), &volume)
	})
	if err != nil {
		return nil, err
	}
	return &volume, nil
}

func (s *StateStore) PutVolume(dep *proto.Volume) error {
	err := s.db.Update(func(dbTxn *bolt.Tx) error {
		return dbPut(dbTxn.Bucket(volumesBkt), []byte(dep.Id), dep)
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *StateStore) GetDeployment(id string) (*proto.Service, error) {
	var service proto.Service
	err := s.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(deploymentBucket)

		return dbGet(bkt, []byte(id), &service)
	})
	if err != nil {
		return nil, err
	}
	return &service, nil
}

func (s *StateStore) PutDeployment(dep *proto.Service) error {
	err := s.db.Update(func(dbTxn *bolt.Tx) error {
		if err := dbPut(dbTxn.Bucket(deploymentBucket), []byte(dep.Name), dep); err != nil {
			return err
		}
		for _, vol := range dep.Volumes {
			fmt.Println("-- write volume --", vol.Id)
			if err := dbPut(dbTxn.Bucket(volumesBkt), []byte(vol.Id), vol); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
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

	// validate that if the alloc is being updated, the alias is available
	if alloc.Alias != "" {
		obj, err := s.allocationByAliasImpl(memTxn, alloc.Alias)
		if err != nil {
			return err
		}
		if obj != nil && obj.Id != alloc.Id {
			return fmt.Errorf("alias already in use")
		}
	}

	err := s.db.Update(func(dbTxn *bolt.Tx) error {
		return s.putAllocation(dbTxn, memTxn, alloc)
	})
	if err == nil {
		memTxn.Commit()
	}

	return err
}

func (s *StateStore) SubscribeEvents(service string, ws memdb.WatchSet) memdb.ResultIterator {
	txn := s.memDb.Txn(false)
	defer txn.Abort()

	iter, err := txn.Get("events", "service", service)
	if err != nil {
		return nil
	}
	ws.Add(iter.WatchCh())
	return iter
}

func (s *StateStore) InsertEvent(event *proto.Event) error {
	memTxn := s.memDb.Txn(true)
	defer memTxn.Abort()

	if err := memTxn.Insert("events", event); err != nil {
		return err
	}

	memTxn.Commit()
	return nil
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

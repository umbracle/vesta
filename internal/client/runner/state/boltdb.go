package state

import (
	"fmt"

	"github.com/boltdb/bolt"
	gproto "github.com/golang/protobuf/proto"
	proto "github.com/umbracle/vesta/internal/client/runner/structs"
)

var _ State = &BoltdbStore{}

var (
	allocsBucket = []byte("allocs")

	allocKey = []byte("alloc")

	taskSpecKey = []byte("task-spec")

	taskStateKey = []byte("task-state")

	taskHandleKey = []byte("task-handle")
)

func taskKey(name string) []byte {
	return []byte("task-" + name)
}

type BoltdbStore struct {
	db *bolt.DB
}

func NewBoltdbStore(path string) (*BoltdbStore, error) {
	db, err := bolt.Open(path, 0755, &bolt.Options{})
	if err != nil {
		return nil, err
	}

	return NewBoltdbStoreWithDB(db)
}

func NewBoltdbStoreWithDB(db *bolt.DB) (*BoltdbStore, error) {
	err := db.Update(func(tx *bolt.Tx) error {
		buckets := [][]byte{
			allocsBucket,
		}
		for _, bkt := range buckets {
			if _, err := tx.CreateBucketIfNotExists(bkt); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	s := &BoltdbStore{
		db: db,
	}
	return s, nil
}

func (s *BoltdbStore) DeleteAllocationBucket(allocID string) error {
	err := s.db.Update(func(tx *bolt.Tx) error {
		allocsBkt := tx.Bucket(allocsBucket)

		return allocsBkt.DeleteBucket([]byte(allocID))
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *BoltdbStore) PutAllocation(a *proto.Allocation) error {
	err := s.db.Update(func(tx *bolt.Tx) error {
		allocsBkt := tx.Bucket(allocsBucket)

		bkt, err := allocsBkt.CreateBucketIfNotExists([]byte(a.Deployment.Name))
		if err != nil {
			return err
		}
		if err := dbPut(bkt, allocKey, a); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *BoltdbStore) GetAllocations() ([]*proto.Allocation, error) {
	allocs := []*proto.Allocation{}
	s.db.View(func(tx *bolt.Tx) error {
		allocsBkt := tx.Bucket(allocsBucket)

		c := allocsBkt.Cursor()
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			allocBkt := allocsBkt.Bucket(k)

			alloc := proto.Allocation{}
			if err := dbGet(allocBkt, allocKey, &alloc); err != nil {
				return err
			}
			allocs = append(allocs, &alloc)
		}
		return nil
	})
	return allocs, nil
}

func (s *BoltdbStore) GetTaskState(allocID, taskName string) (*proto.TaskState, *proto.TaskHandle, error) {
	state := proto.TaskState{}
	handle := proto.TaskHandle{}

	err := s.db.View(func(tx *bolt.Tx) error {
		allocsBkt := tx.Bucket(allocsBucket)

		allocBkt := allocsBkt.Bucket([]byte(allocID))
		if allocBkt == nil {
			return fmt.Errorf("alloc '%s' not found", allocID)
		}

		taskBkt := allocBkt.Bucket(taskKey(taskName))
		if taskBkt == nil {
			return fmt.Errorf("task '%s' not found", taskName)
		}
		if err := dbGet(taskBkt, taskStateKey, &state); err != nil {
			return fmt.Errorf("failed to get task state '%s' '%s'", allocID, taskName)
		}
		if err := dbGet(taskBkt, taskHandleKey, &handle); err != nil {
			return fmt.Errorf("failed to get handle '%s' '%s'", allocID, taskName)
		}
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get task state %v", err)
	}

	return &state, &handle, nil
}

func (s *BoltdbStore) PutTaskLocalState(allocID string, taskName string, handle *proto.TaskHandle) error {
	err := s.db.Update(func(tx *bolt.Tx) error {
		allocsBkt := tx.Bucket(allocsBucket)

		allocBkt := allocsBkt.Bucket([]byte(allocID))
		if allocBkt == nil {
			return fmt.Errorf("alloc '%s' not found", allocID)
		}

		taskBkt, err := allocBkt.CreateBucketIfNotExists(taskKey(taskName))
		if err != nil {
			return err
		}
		if err := dbPut(taskBkt, taskHandleKey, handle); err != nil {
			return fmt.Errorf("failed to get handle %s %s", allocID, taskName)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *BoltdbStore) PutTaskState(allocID string, taskName string, state *proto.TaskState) error {
	err := s.db.Update(func(tx *bolt.Tx) error {
		allocsBkt := tx.Bucket(allocsBucket)

		allocBkt := allocsBkt.Bucket([]byte(allocID))
		if allocBkt == nil {
			return fmt.Errorf("alloc '%s' not found", allocID)
		}

		taskBkt, err := allocBkt.CreateBucketIfNotExists(taskKey(taskName))
		if err != nil {
			return err
		}
		if err := dbPut(taskBkt, taskStateKey, state); err != nil {
			return fmt.Errorf("failed to get task state '%s' '%s'", allocID, taskName)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to put task state: %v", err)
	}
	return nil
}

func (s *BoltdbStore) Close() error {
	return s.db.Close()
}

func dbPut(bkt *bolt.Bucket, key []byte, obj gproto.Message) error {
	raw, err := gproto.Marshal(obj)
	if err != nil {
		return err
	}
	if err := bkt.Put(key, raw); err != nil {
		return err
	}
	return nil
}

func dbGet(bkt *bolt.Bucket, key []byte, obj gproto.Message) error {
	raw := bkt.Get(key)
	if raw == nil {
		return fmt.Errorf("not exists")
	}
	if err := gproto.Unmarshal(raw, obj); err != nil {
		return err
	}
	return nil
}

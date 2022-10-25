package state

import (
	"fmt"

	"github.com/boltdb/bolt"
	gproto "github.com/golang/protobuf/proto"
	"github.com/umbracle/vesta/internal/server/proto"
)

var (
	allocsBucket = []byte("allocs")

	allocKey = []byte("alloc")

	taskStateKey = []byte("task-state")

	taskHandleKey = []byte("task-handle")
)

func taskKey(name string) []byte {
	return []byte("task-" + name)
}

type State struct {
	db *bolt.DB
}

func NewState(path string) (*State, error) {
	db, err := bolt.Open(path, 0755, &bolt.Options{})
	if err != nil {
		return nil, err
	}

	err = db.Update(func(tx *bolt.Tx) error {
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

	s := &State{
		db: db,
	}
	return s, nil
}

func (s *State) PutAllocation(a *proto.Allocation) error {
	err := s.db.Update(func(tx *bolt.Tx) error {
		allocsBkt := tx.Bucket(allocsBucket)

		bkt, err := allocsBkt.CreateBucketIfNotExists([]byte(a.Id))
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

func (s *State) GetAllocations() ([]*proto.Allocation, error) {
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

func (s *State) GetTaskState(allocID, taskName string) (*proto.TaskState, *proto.TaskHandle, error) {
	state := proto.TaskState{}
	handle := proto.TaskHandle{}

	err := s.db.View(func(tx *bolt.Tx) error {
		allocsBkt := tx.Bucket(allocsBucket)

		allocBkt := allocsBkt.Bucket([]byte(allocID))
		if allocBkt == nil {
			return fmt.Errorf("not found")
		}

		taskBkt := allocBkt.Bucket(taskKey(taskName))
		if taskBkt == nil {
			return fmt.Errorf("task not found")
		}
		if err := dbGet(taskBkt, taskStateKey, &state); err != nil {
			return fmt.Errorf("failed to get task state %s %s", allocID, taskName)
		}
		if err := dbGet(taskBkt, taskHandleKey, &handle); err != nil {
			return fmt.Errorf("failed to get handle %s %s", allocID, taskName)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	return &state, &handle, nil
}

func (s *State) PutTaskLocalState(allocID string, taskName string, handle *proto.TaskHandle) error {
	err := s.db.Update(func(tx *bolt.Tx) error {
		allocsBkt := tx.Bucket(allocsBucket)

		allocBkt := allocsBkt.Bucket([]byte(allocID))
		if allocBkt == nil {
			return fmt.Errorf("not found")
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

func (s *State) PutTaskState(allocID string, taskName string, state *proto.TaskState) error {
	err := s.db.Update(func(tx *bolt.Tx) error {
		allocsBkt := tx.Bucket(allocsBucket)

		allocBkt := allocsBkt.Bucket([]byte(allocID))
		if allocBkt == nil {
			return fmt.Errorf("not found")
		}

		taskBkt, err := allocBkt.CreateBucketIfNotExists(taskKey(taskName))
		if err != nil {
			return err
		}
		if err := dbPut(taskBkt, taskStateKey, state); err != nil {
			return fmt.Errorf("failed to get task state %s %s", allocID, taskName)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
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

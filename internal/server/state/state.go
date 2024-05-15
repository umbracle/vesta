package state

import (
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
		return nil, err
	}
	s := &StateStore{
		db:    db,
		memDb: memDb,
	}

	return s, nil
}

func (s *StateStore) Close() error {
	return s.db.Close()
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

func (s *StateStore) GetVolumes() ([]*proto.Volume, error) {
	var volumes []*proto.Volume

	err := s.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(volumesBkt)

		return bkt.ForEach(func(k, v []byte) error {
			var volume proto.Volume
			if err := dbGet(bkt, k, &volume); err != nil {
				return err
			}

			volumes = append(volumes, &volume)
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	return volumes, nil
}

func (s *StateStore) GetDeployments() ([]*proto.Service, error) {
	var services []*proto.Service

	err := s.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(deploymentBucket)

		return bkt.ForEach(func(k, v []byte) error {
			var service proto.Service
			if err := dbGet(bkt, k, &service); err != nil {
				return err
			}

			services = append(services, &service)
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	return services, nil
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

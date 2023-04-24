package client

import (
	"github.com/hashicorp/go-memdb"
	babel "github.com/umbracle/babel/sdk"
	"github.com/umbracle/vesta/internal/server/proto"
)

type ControlPlane interface {
	// Pull pulls the configs assigned to the agent when updated
	Pull(nodeId string, ws memdb.WatchSet) ([]*proto.Allocation, error)

	// UpdateAlloc updates an allocation state
	UpdateAlloc(alloc *proto.Allocation) error

	// UpdateSyncStatus updates the sync status of an allocation
	UpdateSyncStatus(alloc, task string, status *babel.SyncStatus) error
}

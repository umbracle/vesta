package client

import (
	"github.com/hashicorp/go-memdb"
	"github.com/umbracle/vesta/internal/server/proto"
)

type ControlPlane interface {
	// Pull pulls the configs assigned to the agent when updated
	Pull(nodeId string, ws memdb.WatchSet) ([]*proto.Allocation, error)

	// UpdateAlloc updates an allocation state
	UpdateAlloc(alloc *proto.Allocation) error
}

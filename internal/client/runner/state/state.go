package state

import proto "github.com/umbracle/vesta/internal/client/runner/structs"

type State interface {
	PutTaskLocalState(allocID string, taskName string, handle *proto.TaskHandle) error
	GetTaskState(allocID, taskName string) (*proto.TaskState, *proto.TaskHandle, error)
	PutTaskState(allocID string, taskName string, state *proto.TaskState) error
	GetAllocations() ([]*proto.Allocation, error)
	PutAllocation(a *proto.Allocation) error
	DeleteAllocationBucket(allocId string) error
	Close() error
}

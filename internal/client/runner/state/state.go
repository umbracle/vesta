package state

import "github.com/umbracle/vesta/internal/server/proto"

type State interface {
	PutTaskLocalState(allocID string, taskName string, handle *proto.TaskHandle) error
	GetTaskState(allocID, taskName string) (*proto.TaskState, *proto.TaskHandle, error)
	PutTaskState(allocID string, taskName string, state *proto.TaskState) error
	GetAllocations() ([]*proto.Allocation1, error)
	PutAllocation(a *proto.Allocation1) error
}

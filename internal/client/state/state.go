package state

import "github.com/umbracle/vesta/internal/server/proto"

type State interface {
	PutTaskLocalState(allocID string, taskName string, handle *proto.TaskHandle) error
	GetTaskState(allocID, taskName string) (*proto.TaskState, *proto.TaskHandle, error)
	PutTaskState(allocID string, taskName string, state *proto.TaskState) error
	GetAllocationTasks(allocID string) ([]*proto.Task, error)
	PutTaskSpec(allocID string, task *proto.Task) error
	PutAllocation(a *proto.Allocation) error
}

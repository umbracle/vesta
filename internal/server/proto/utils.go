package proto

import (
	"google.golang.org/protobuf/proto"
)

const (
	TaskStarted       = "Started"
	TaskTerminated    = "Terminated"
	TaskRestarting    = "Restarting"
	TaskNotRestarting = "Not-restarting"
)

func (a *Allocation) Copy() *Allocation {
	return proto.Clone(a).(*Allocation)
}

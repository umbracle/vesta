package proto

import (
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func NewTaskState() *TaskState {
	return &TaskState{
		State: TaskState_Pending,
	}
}

func (r *ExitResult) Successful() bool {
	return r.ExitCode == 0 && r.Signal == 0 && r.Err == ""
}

func NewTaskEvent(typ string) *TaskState_Event {
	return &TaskState_Event{
		Type:    typ,
		Time:    timestamppb.Now(),
		Details: map[string]string{},
	}
}

func (t *TaskState_Event) SetExitCode(c int64) *TaskState_Event {
	t.Details["exit_code"] = fmt.Sprintf("%d", c)
	return t
}

func (t *TaskState_Event) SetSignal(s int64) *TaskState_Event {
	t.Details["signal"] = fmt.Sprintf("%d", s)
	return t
}

func (t *TaskState_Event) FailsTask() bool {
	_, ok := t.Details["fails_task"]
	return ok
}

func (t *TaskState_Event) SetFailsTask() *TaskState_Event {
	t.Details["fails_task"] = "true"
	return t
}

const (
	TaskStarted       = "Started"
	TaskTerminated    = "Terminated"
	TaskRestarting    = "Restarting"
	TaskNotRestarting = "Not-restarting"
)

func (a *Allocation) Copy() *Allocation {
	return proto.Clone(a).(*Allocation)
}

package hooks

import (
	"context"

	"github.com/hashicorp/go-hclog"
	proto "github.com/umbracle/vesta/internal/client/runner/structs"
)

type TaskHook interface {
	Name() string
}

type TaskHookFactory func(logger hclog.Logger, alloc *proto.Allocation, task *proto.Task) TaskHook

type TaskPoststartHookRequest struct {
	Spec *proto.TaskHandle_Network
}

type TaskPoststartHook interface {
	TaskHook

	Poststart(context.Context, *TaskPoststartHookRequest) error
}

type TaskPrestartHookRequest struct {
}

type TaskPrestartHook interface {
	TaskHook

	Prestart(context.Context, *TaskPrestartHookRequest) error
}

type TaskStopRequest struct {
}

type TaskStopHook interface {
	TaskHook

	Stop(context.Context, *TaskStopRequest) error
}

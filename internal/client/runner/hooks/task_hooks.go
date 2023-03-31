package hooks

import (
	"github.com/hashicorp/go-hclog"
	proto "github.com/umbracle/vesta/internal/client/runner/structs"
)

type TaskHook interface {
	Name() string
}

type TaskHookFactory func(logger hclog.Logger, task *proto.Task) TaskHook

type TaskPoststartHookRequest struct {
	Spec *proto.TaskHandle_Network
}

type TaskPoststartHook interface {
	TaskHook

	Poststart(chan struct{}, *TaskPoststartHookRequest) error
}

type TaskPrestartHookRequest struct {
}

type TaskPrestartHook interface {
	TaskHook

	Prestart(chan struct{}, *TaskPrestartHookRequest) error
}

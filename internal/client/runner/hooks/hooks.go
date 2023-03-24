package hooks

import (
	"github.com/hashicorp/go-hclog"
	"github.com/umbracle/vesta/internal/server/proto"
)

type TaskHook interface {
	Name() string
}

type TaskHookFactory func(logger hclog.Logger, task *proto.Task1) TaskHook

type TaskPoststartHookRequest struct {
}

type TaskPoststartHook interface {
	TaskHook

	Poststart(chan struct{}, *TaskPoststartHookRequest) error
}

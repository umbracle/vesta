package hooks

import (
	"github.com/hashicorp/go-hclog"
	"github.com/umbracle/vesta/internal/client/runner/proto"
)

type TaskHook interface {
	Name() string
}

type TaskHookFactory func(logger hclog.Logger, task *proto.Task) TaskHook

type TaskPoststartHookRequest struct {
}

type TaskPoststartHook interface {
	TaskHook

	Poststart(chan struct{}, *TaskPoststartHookRequest) error
}

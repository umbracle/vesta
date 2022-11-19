package driver

import (
	"context"
	"time"

	"github.com/umbracle/vesta/internal/server/proto"
)

type Driver interface {
	StartTask(task *proto.Task, allocDir string) (*proto.TaskHandle, error)
	RecoverTask(taskID string, task *proto.TaskHandle) error
	WaitTask(ctx context.Context, taskID string) (<-chan *proto.ExitResult, error)
	StopTask(taskID string, timeout time.Duration) error
	DestroyTask(taskID string, force bool) error
}

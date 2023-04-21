package driver

import (
	"context"
	"time"

	proto "github.com/umbracle/vesta/internal/client/runner/structs"
)

type Driver interface {
	StartTask(task *Task) (*proto.TaskHandle, error)
	RecoverTask(taskID string, task *proto.TaskHandle) error
	WaitTask(ctx context.Context, taskID string) (<-chan *proto.ExitResult, error)
	StopTask(taskID string, timeout time.Duration) error
	DestroyTask(taskID string, force bool) error
	CreateNetwork(allocID string, dnsAlias []string, hostname string) (*proto.NetworkSpec, bool, error)
	DestroyNetwork(spec *proto.NetworkSpec) error
	ExecTask(taskID string, cmd []string) (*ExecTaskResult, error)
}

type Task struct {
	Id string

	AllocID string

	Network *proto.NetworkSpec

	Mounts []*MountConfig

	*proto.Task
}

type MountConfig struct {
	TaskPath string
	HostPath string
}

type ExecTaskResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode uint64
}

package taskrunner

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-getter"
	"github.com/hashicorp/go-hclog"
	"github.com/umbracle/vesta/internal/client/runner/allocrunner/allocdir"
	"github.com/umbracle/vesta/internal/client/runner/hooks"
	proto "github.com/umbracle/vesta/internal/client/runner/structs"
)

var _ hooks.TaskHook = &artifactsHook{}
var _ hooks.TaskPrestartHook = &artifactsHook{}

type artifactsHook struct {
	logger  hclog.Logger
	task    *proto.Task
	taskDir *allocdir.TaskDir
}

func newArtifactsHook(logger hclog.Logger, alloc *proto.Allocation, taskDir *allocdir.TaskDir, task *proto.Task) *artifactsHook {
	h := &artifactsHook{
		task:    task,
		taskDir: taskDir,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (a *artifactsHook) Name() string {
	return "artifacts-dir"
}

func (a *artifactsHook) Prestart(ctx context.Context, req *hooks.TaskPrestartHookRequest) error {
	if len(a.task.Artifacts) == 0 {
		return nil
	}

	for _, artifact := range a.task.Artifacts {
		dst, ok := a.taskDir.ResolvePath(artifact.Destination)
		if !ok {
			return fmt.Errorf("could not resolve local destination: %s", artifact.Destination)
		}

		fmt.Println("ssss", artifact.Destination, artifact.Source, dst)

		client := &getter.Client{
			Ctx:  context.Background(),
			Src:  artifact.Source,
			Dst:  dst,
			Mode: getter.ClientModeFile,
		}
		if err := client.Get(); err != nil {
			panic(err)
		}
	}

	return nil
}

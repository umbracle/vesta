package docker

import (
	"context"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/hashicorp/go-hclog"
	"github.com/umbracle/vesta/internal/server/proto"
)

type taskHandle struct {
	client      *client.Client
	logger      hclog.Logger
	containerID string

	waitCh chan struct{}

	exitResult     *proto.ExitResult
	exitResultLock sync.Mutex
}

func (t *taskHandle) run() {
	t.logger.Info("handle running", "id", t.containerID)

	statusCh, errCh := t.client.ContainerWait(context.Background(), t.containerID, container.WaitConditionNotRunning)

	var status container.ContainerWaitOKBody
	select {
	case err := <-errCh:
		if err != nil {
			// TODO: unit test
			t.logger.Error("failed to wait container", "id", t.containerID, "err", err)
		}
	case status = <-statusCh:
	}

	t.exitResultLock.Lock()
	t.exitResult = &proto.ExitResult{
		ExitCode: status.StatusCode,
		Signal:   0,
	}
	t.exitResultLock.Unlock()
	close(t.waitCh)
}

func (t *taskHandle) Kill(killTimeout time.Duration) error {
	if err := t.client.ContainerStop(context.Background(), t.containerID, &killTimeout); err != nil {
		return err
	}
	return nil
}

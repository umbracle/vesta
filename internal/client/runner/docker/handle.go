package docker

import (
	"bytes"
	"context"
	"io/ioutil"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/hashicorp/go-hclog"
	"github.com/umbracle/vesta/internal/client/runner/driver"
	proto "github.com/umbracle/vesta/internal/client/runner/structs"
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

func (h *taskHandle) Exec(ctx context.Context, args []string) (*driver.ExecTaskResult, error) {
	config := types.ExecConfig{
		AttachStderr: true,
		AttachStdout: true,
		Cmd:          args,
	}

	exec, err := h.client.ContainerExecCreate(ctx, h.containerID, config)
	if err != nil {
		return nil, err
	}

	resp, err := h.client.ContainerExecAttach(ctx, exec.ID, types.ExecStartCheck{})
	if err != nil {
		return nil, err
	}
	defer resp.Close()

	// read the output
	var outBuf, errBuf bytes.Buffer
	outputDone := make(chan error)

	go func() {
		// StdCopy demultiplexes the stream into two buffers
		_, err = stdcopy.StdCopy(&outBuf, &errBuf, resp.Reader)
		outputDone <- err
	}()

	select {
	case err := <-outputDone:
		if err != nil {
			return nil, err
		}
		break

	case <-ctx.Done():
		return nil, ctx.Err()
	}

	stdout, err := ioutil.ReadAll(&outBuf)
	if err != nil {
		return nil, err
	}
	stderr, err := ioutil.ReadAll(&errBuf)
	if err != nil {
		return nil, err
	}

	res, err := h.client.ContainerExecInspect(ctx, exec.ID)
	if err != nil {
		return nil, err
	}

	execResult := &driver.ExecTaskResult{
		ExitCode: uint64(res.ExitCode),
		Stdout:   stdout,
		Stderr:   stderr,
	}
	return execResult, nil
}

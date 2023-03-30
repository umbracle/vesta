package docker

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/client"
	"github.com/hashicorp/go-hclog"
	"github.com/umbracle/vesta/internal/client/runner/driver"
	proto "github.com/umbracle/vesta/internal/client/runner/structs"
)

var _ driver.Driver = &Docker{}

var ErrTaskNotFound = fmt.Errorf("task not found")

var networkName = "vesta"

type Docker struct {
	logger      hclog.Logger
	client      *client.Client
	coordinator *dockerImageCoordinator
	store       *taskStore
}

func NewDockerDriver(logger hclog.Logger) (*Docker, error) {
	if logger == nil {
		logger = hclog.NewNullLogger()
	}

	client, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}

	// initialize the vesta private network. This is required to have
	// DNS discovery by docker.
	networks, err := client.NetworkList(context.Background(), types.NetworkListOptions{})
	if err != nil {
		return nil, err
	}
	found := false
	for _, net := range networks {
		if net.Name == networkName {
			found = true
			break
		}
	}
	if !found {
		if _, err := client.NetworkCreate(context.Background(), networkName, types.NetworkCreate{CheckDuplicate: true}); err != nil {
			return nil, err
		}
	}

	d := &Docker{
		logger:      hclog.NewNullLogger(),
		client:      client,
		coordinator: newDockerImageCoordinator(client),
		store:       newTaskStore(),
	}
	return d, nil
}

func (d *Docker) DestroyTask(taskID string, force bool) error {
	h, ok := d.store.Get(taskID)
	if !ok {
		return ErrTaskNotFound
	}

	c, err := d.client.ContainerInspect(context.Background(), h.containerID)
	if err != nil {
		return err
	} else {
		if c.State.Running {
			if !force {
				return fmt.Errorf("cannot destroy if force not set to true")
			}

			timeout := 1 * time.Second
			if err := d.client.ContainerStop(context.Background(), h.containerID, &timeout); err != nil {
				h.logger.Warn("failed to stop container", "err", err)
			}
		}
	}

	d.store.Delete(taskID)
	return nil
}

func (d *Docker) StopTask(taskID string, timeout time.Duration) error {
	h, ok := d.store.Get(taskID)
	if !ok {
		return ErrTaskNotFound
	}

	return h.Kill(timeout)
}

func (d *Docker) WaitTask(ctx context.Context, taskID string) (<-chan *proto.ExitResult, error) {
	handle, ok := d.store.Get(taskID)
	if !ok {
		return nil, ErrTaskNotFound
	}
	ch := make(chan *proto.ExitResult)
	go func() {
		defer close(ch)

		select {
		case <-handle.waitCh:
			ch <- handle.exitResult
		case <-ctx.Done():
			ch <- &proto.ExitResult{
				Err: ctx.Err().Error(),
			}
		}
	}()
	return ch, nil
}

func (d *Docker) RecoverTask(taskID string, task *proto.TaskHandle) error {
	if _, ok := d.store.Get(taskID); ok {
		return nil
	}

	if _, err := d.client.ContainerInspect(context.Background(), task.ContainerID); err != nil {
		return err
	}

	h := &taskHandle{
		logger:      d.logger.Named(taskID),
		client:      d.client,
		containerID: task.ContainerID,
		waitCh:      make(chan struct{}),
	}

	d.store.Set(taskID, h)
	go h.run()

	return nil
}

func (d *Docker) ExecTask(taskID string, cmd []string) (*driver.ExecTaskResult, error) {
	h, ok := d.store.Get(taskID)
	if !ok {
		return nil, fmt.Errorf("task not found")
	}

	return h.Exec(context.Background(), cmd)
}

func (d *Docker) StartTask(task *driver.Task) (*proto.TaskHandle, error) {
	d.logger.Info("Create task", "image", task.Image, "tag", task.Tag)

	if err := d.createImage(task.Image + ":" + task.Tag); err != nil {
		return nil, err
	}

	opts, err := d.createContainerOptions(task)
	if err != nil {
		return nil, err
	}
	body, err := d.client.ContainerCreate(context.Background(), opts.config, opts.host, opts.network, nil, "")
	if err != nil {
		return nil, err
	}

	if err := d.client.ContainerStart(context.Background(), body.ID, types.ContainerStartOptions{}); err != nil {
		return nil, err
	}

	h := &taskHandle{
		logger:      d.logger.Named(task.Id),
		client:      d.client,
		containerID: body.ID,
		waitCh:      make(chan struct{}),
	}

	d.store.Set(task.Id, h)
	go h.run()

	handle := &proto.TaskHandle{
		Id:          task.Id,
		ContainerID: body.ID,
		Network:     &proto.TaskHandle_Network{
			// Ip: ip,
		},
	}
	return handle, nil
}

type createContainerOptions struct {
	name    string
	config  *container.Config
	host    *container.HostConfig
	network *network.NetworkingConfig
}

func (d *Docker) createContainerOptions(task *driver.Task) (*createContainerOptions, error) {
	labels := map[string]string{}
	for k, v := range task.Labels {
		labels[k] = v
	}
	// append system wide labels
	labels["vesta"] = "true"

	config := &container.Config{
		Image:  task.Image + ":" + task.Tag,
		Cmd:    strslice.StrSlice(task.Args),
		Labels: labels,
	}
	for k, v := range task.Env {
		config.Env = append(config.Env, k+"="+v)
	}

	hostConfig := &container.HostConfig{
		Binds: []string{},
	}
	for _, mount := range task.Mounts {
		// both volume and bind get mounted the same way
		hostConfig.Binds = append(hostConfig.Binds, mount.HostPath+":"+mount.TaskPath)
	}

	if task.Network != nil {
		// connect network and pid to the network start container
		hostConfig.NetworkMode = container.NetworkMode("container:init-" + task.AllocID)
		hostConfig.PidMode = container.PidMode("container:init-" + task.AllocID)
	}

	opts := &createContainerOptions{
		config: config,
		host:   hostConfig,
	}
	return opts, nil
}

func (d *Docker) createImage(image string) error {

	_, dockerImageRaw, _ := d.client.ImageInspectWithRaw(context.Background(), image)
	if dockerImageRaw != nil {
		// already available
		return nil
	}
	if _, err := d.coordinator.PullImage(image); err != nil {
		return err
	}
	return nil
}

func (d *Docker) containerByName(name string) (*types.ContainerJSON, error) {
	containers, err := d.client.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		return nil, err
	}

	var (
		bodyID string
		found  bool
	)

	containerName := "/" + name

OUTER:
	for _, c := range containers {
		for _, cName := range c.Names {
			if cName == containerName {
				bodyID = c.ID
				found = true
				break OUTER
			}
		}
	}

	if !found {
		return nil, nil
	}

	container, err := d.client.ContainerInspect(context.Background(), bodyID)
	if err != nil {
		return nil, err
	}

	return &container, nil
}

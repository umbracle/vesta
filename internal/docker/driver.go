package docker

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/client"
	"github.com/hashicorp/go-hclog"
	"github.com/umbracle/vesta/internal/server/proto"
)

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
			if err := d.client.ContainerStop(context.Background(), h.containerID, nil); err != nil {
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

func (d *Docker) StartTask(task *proto.Task, allocDir string) (*proto.TaskHandle, error) {
	d.logger.Info("Create task", "image", task.Image, "tag", task.Tag)

	if err := d.createImage(task.Image + ":" + task.Tag); err != nil {
		return nil, err
	}

	opts, err := d.createContainerOptions(task, allocDir)
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

	container, err := d.client.ContainerInspect(context.Background(), body.ID)
	if err != nil {
		return nil, err
	}
	net, ok := container.NetworkSettings.Networks[networkName]
	if !ok {
		return nil, fmt.Errorf("network settings not found in container")
	}
	ip := net.IPAddress

	h := &taskHandle{
		logger:      d.logger.Named(task.Id),
		client:      d.client,
		containerID: body.ID,
		waitCh:      make(chan struct{}),
	}

	d.store.Set(task.Id, h)
	go h.run()

	handle := &proto.TaskHandle{
		ContainerID: body.ID,
		Network: &proto.TaskHandle_Network{
			Ip: ip,
		},
	}
	return handle, nil
}

type createContainerOptions struct {
	config  *container.Config
	host    *container.HostConfig
	network *network.NetworkingConfig
}

func (d *Docker) createContainerOptions(task *proto.Task, allocDir string) (*createContainerOptions, error) {
	// build any mount path
	mountMap := map[string]string{}
	for _, mount := range []string{"/var"} {
		tmpDir, err := ioutil.TempDir("/tmp", "vesta-")
		if err != nil {
			return nil, err
		}
		mountMap[mount] = tmpDir
	}

	for dest, data := range task.Data {
		// --- write data ---
		path := dest

		// find the mount match
		var mount, local string
		var found bool

	MOUNT:
		for mount, local = range mountMap {
			if strings.HasPrefix(path, mount) {
				found = true
				break MOUNT
			}
		}
		if !found {
			return nil, fmt.Errorf("mount match for '%s' not found", path)
		}

		relPath := strings.TrimPrefix(path, mount)
		localPath := filepath.Join(local, relPath)

		// create all the directory paths required
		parentDir := filepath.Dir(localPath)
		if err := os.MkdirAll(parentDir, 0700); err != nil {
			return nil, err
		}
		if err := ioutil.WriteFile(localPath, []byte(data), 0644); err != nil {
			return nil, err
		}
	}

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
	for dest, src := range mountMap {
		hostConfig.Binds = append(hostConfig.Binds, src+":"+dest)
	}

	if allocDir != "" {
		// for each volume, create an entry in alloc dir and mount it
		for name, vol := range task.Volumes {
			path := filepath.Join(allocDir, name)
			if err := os.Mkdir(path, 0755); err != nil {
				return nil, fmt.Errorf("failed to create volume dir '%s': %v", name, vol)
			}
			hostConfig.Binds = append(hostConfig.Binds, path+":"+vol.Path)
		}
	}

	opts := &createContainerOptions{
		config: config,
		host:   hostConfig,
		network: &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				networkName: {
					Aliases: []string{task.AllocId},
				},
			},
		},
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

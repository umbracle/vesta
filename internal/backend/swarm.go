package backend

import (
	"context"
	"fmt"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/client"
	"github.com/umbracle/vesta/internal/server/proto"
	"github.com/umbracle/vesta/internal/uuid"
)

type Swarm struct {
	client  *client.Client
	updater Updater
}

type Updater interface {
	UpdateEvent(event *proto.Event2)
}

func NewSwarm(updater Updater) *Swarm {
	client, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		panic(err)
	}
	client.NegotiateAPIVersion(context.Background())

	s := &Swarm{
		client:  client,
		updater: updater,
	}
	go s.trackEventUpdates()

	return s
}

func (s *Swarm) trackEventUpdates() {
	filters := filters.NewArgs()
	filters.Add("label", "vesta=true")

	msgCh, errCh := s.client.Events(context.Background(), types.EventsOptions{Filters: filters})

	for {
		select {
		case msg := <-msgCh:
			if msg.Actor.Attributes["role"] == "init-container" {
				// we do not want to return attributes from this container
				continue
			}

			deployment, ok := msg.Actor.Attributes["deployment"]
			if !ok {
				panic("??")
			}
			event := &proto.Event2{
				Id:         uuid.Generate(),
				Deployment: deployment,
				Task:       msg.Actor.Attributes["name"],
				Type:       msg.Action,
			}
			s.updater.UpdateEvent(event)

		case err := <-errCh:
			fmt.Println("err", err)
		}
	}
}

func (s *Swarm) Deploy(name string, tasks map[string]*proto.Task) error {
	// create the network reference
	initRes := s.createNetworkContainer(name)

	for name, t := range tasks {
		opts, err := s.createContainerOptions(t, initRes)
		if err != nil {
			return err
		}
		body, err := s.client.ContainerCreate(context.Background(), opts.config, opts.host, opts.network, nil, name)
		if err != nil {
			return err
		}
		if err := s.client.ContainerStart(context.Background(), body.ID, types.ContainerStartOptions{}); err != nil {
			return err
		}
	}

	return nil
}

var (
	networkInfraImage = "gcr.io/google_containers/pause-amd64:3.1"
)

type createContainerOptions struct {
	name    string
	config  *container.Config
	host    *container.HostConfig
	network *network.NetworkingConfig
}

func (s *Swarm) createNetworkContainer(name string) string {
	initContainerName := "init-" + name

	opts := &createContainerOptions{
		name: initContainerName,
		config: &container.Config{
			Image:    networkInfraImage,
			Hostname: "",
			Labels: map[string]string{
				"vesta": "true",
				"role":  "init-container",
			},
		},
		host: &container.HostConfig{
			NetworkMode: container.NetworkMode("vesta"),
		},
		network: &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{},
		},
	}

	body, err := s.client.ContainerCreate(context.Background(), opts.config, opts.host, opts.network, nil, opts.name)
	if err != nil {
		panic(err)
	}
	if err := s.client.ContainerStart(context.Background(), body.ID, types.ContainerStartOptions{}); err != nil {
		panic(err)
	}

	_, err = s.client.ContainerInspect(context.Background(), body.ID)
	if err != nil {
		panic(err)
	}

	return initContainerName
}

func (s *Swarm) createContainerOptions(task *proto.Task, network string) (*createContainerOptions, error) {
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
		Binds:       []string{},
		NetworkMode: container.NetworkMode("container:" + network),
		PidMode:     container.PidMode("container:" + network),
	}

	for folder, data := range task.Data {
		f, err := os.CreateTemp("", "vesta")
		if err != nil {
			return nil, err
		}
		if _, err := f.Write([]byte(data)); err != nil {
			return nil, err
		}
		if err := f.Close(); err != nil {
			return nil, err
		}
		hostConfig.Binds = append(hostConfig.Binds, f.Name()+":"+folder)
	}

	opts := &createContainerOptions{
		config: config,
		host:   hostConfig,
	}
	return opts, nil
}

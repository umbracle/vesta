package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/umbracle/vesta/internal/client/runner/structs"
)

var (
	networkInfraImage = "gcr.io/google_containers/pause-amd64:3.1"
)

func (d *Docker) CreateNetwork(allocID string, hostname string) (*structs.NetworkSpec, bool, error) {
	if err := d.createImage(networkInfraImage); err != nil {
		return nil, false, err
	}

	nets, err := d.client.NetworkList(context.Background(), types.NetworkListOptions{})
	if err != nil {
		panic(err)
	}
	for _, net := range nets {
		fmt.Println(net.Name, net.ID, net.Created)
	}

	opts := &createContainerOptions{
		name: "init-" + allocID,
		config: &container.Config{
			Image:    networkInfraImage,
			Hostname: hostname,
		},
		host: &container.HostConfig{
			NetworkMode: container.NetworkMode(networkName),
		},
		network: &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				networkName: {
					Aliases: []string{
						allocID,
					},
				},
			},
		},
	}

	specFromContainer := func(container *types.ContainerJSON) *structs.NetworkSpec {
		// resolve the ip
		var ip string
		for _, net := range container.NetworkSettings.Networks {
			if net.IPAddress == "" {
				// Ignore networks without an IP address
				continue
			}

			ip = net.IPAddress
			break
		}
		return &structs.NetworkSpec{Id: container.ID, Ip: ip}
	}

	container, err := d.containerByName(opts.name)
	if err != nil {
		return nil, false, err
	}
	if container != nil {
		return specFromContainer(container), false, nil
	}

	body, err := d.client.ContainerCreate(context.Background(), opts.config, opts.host, opts.network, nil, opts.name)
	if err != nil {
		return nil, false, err
	}
	if err := d.client.ContainerStart(context.Background(), body.ID, types.ContainerStartOptions{}); err != nil {
		return nil, false, err
	}

	res, err := d.client.ContainerInspect(context.Background(), body.ID)
	if err != nil {
		return nil, false, err
	}
	return specFromContainer(&res), true, nil
}

func (d *Docker) DestroyNetwork(spec *structs.NetworkSpec) error {
	if err := d.client.ContainerRemove(context.Background(), spec.Id, types.ContainerRemoveOptions{Force: true}); err != nil {
		return err
	}
	return nil
}

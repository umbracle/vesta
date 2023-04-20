package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/umbracle/vesta/internal/client/runner/driver"
	proto "github.com/umbracle/vesta/internal/client/runner/structs"
	"github.com/umbracle/vesta/internal/uuid"
)

func TestNetwork_CreateSameNetwork(t *testing.T) {
	allocName := "test-same-network"

	d := NewTestDockerDriver(t)

	spec, created, err := d.CreateNetwork(allocName, []string{}, "b")
	require.NoError(t, err)
	require.True(t, created)

	spec2, created, err := d.CreateNetwork(allocName, []string{}, "b")
	require.NoError(t, err)
	require.False(t, created)

	require.Equal(t, spec.Id, spec2.Id)

	d.DestroyNetwork(spec)
}

func TestNetwork_Destroy(t *testing.T) {
	allocName := "test-destroy"

	d := NewTestDockerDriver(t)

	spec, created, err := d.CreateNetwork(allocName, []string{}, "b")
	require.NoError(t, err)
	require.True(t, created)

	err = d.DestroyNetwork(spec)
	require.NoError(t, err)

	spec2, created, err := d.CreateNetwork(allocName, []string{}, "b")
	require.NoError(t, err)
	require.True(t, created)

	require.NotEqual(t, spec.Id, spec2.Id)

	d.DestroyNetwork(spec2)
}

func TestNetwork_MultipleContainers(t *testing.T) {
	// test that we can deploy a container in the network and has
	// the same network ip.
	allocName := "test-multiple"

	d := NewTestDockerDriver(t)

	spec, _, err := d.CreateNetwork(allocName, []string{}, "b")
	require.NoError(t, err)

	t0 := &driver.Task{
		AllocID: allocName,
		Network: spec,
		Task: &proto.Task{
			Image: "busybox",
			Tag:   "1.29.3",
			Args:  []string{"nc", "-l", "-p", "3000", "0.0.0.0"},
		},
	}

	handle, err := d.StartTask(t0)
	assert.NoError(t, err)

	d.DestroyTask(handle.Id, true)
	d.DestroyNetwork(spec)

	// ip of the handle and the network
	require.Equal(t, spec.Ip, handle.Network.Ip)
}

func TestNetwork_ResolveDNS(t *testing.T) {
	// test that we can deploy two networks and connect them with dns

	// deploy two allocations of nginx, one will work as the target for the dns
	// queries while the other one will generate the dns requests.
	// We will deploy nginx twice since it has both an http server and curl installed

	tasks := []struct {
		allocName string
		extraDNS  []string
	}{
		{
			"target-alloc",
			[]string{"extra"},
		},
		{
			"source-alloc",
			[]string{},
		},
	}

	d := NewTestDockerDriver(t)

	var sourceID string
	for _, tt := range tasks {
		// create target nginx allocation
		spec, _, err := d.CreateNetwork(tt.allocName, tt.extraDNS, "b")
		require.NoError(t, err)

		t0 := &driver.Task{
			Id:      uuid.Generate(),
			AllocID: tt.allocName,
			Network: spec,
			Task:    &proto.Task{Image: "nginx", Tag: "1.24.0"},
		}

		handle, err := d.StartTask(t0)
		require.NoError(t, err)

		defer d.DestroyTask(handle.Id, true)
		defer d.DestroyNetwork(spec)

		if tt.allocName == "source-alloc" {
			sourceID = handle.Id
		}
	}

	paths := []string{
		"http://extra",
		"http://target-alloc",
	}
	for _, path := range paths {
		res, err := d.ExecTask(sourceID, []string{"curl", path})
		require.NoError(t, err)
		require.NotEmpty(t, res.Stdout)
	}
}

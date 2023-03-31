package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/umbracle/vesta/internal/client/runner/driver"
	proto "github.com/umbracle/vesta/internal/client/runner/structs"
)

func TestNetwork_CreateSameNetwork(t *testing.T) {
	allocName := "test-same-network"

	d, _ := NewDockerDriver(nil)

	spec, created, err := d.CreateNetwork(allocName, "b")
	require.NoError(t, err)
	require.True(t, created)

	spec2, created, err := d.CreateNetwork(allocName, "b")
	require.NoError(t, err)
	require.False(t, created)

	require.Equal(t, spec.Id, spec2.Id)

	d.DestroyNetwork(spec)
}

func TestNetwork_Destroy(t *testing.T) {
	allocName := "test-destroy"

	d, _ := NewDockerDriver(nil)

	spec, created, err := d.CreateNetwork(allocName, "b")
	require.NoError(t, err)
	require.True(t, created)

	err = d.DestroyNetwork(spec)
	require.NoError(t, err)

	spec2, created, err := d.CreateNetwork(allocName, "b")
	require.NoError(t, err)
	require.True(t, created)

	require.NotEqual(t, spec.Id, spec2.Id)

	d.DestroyNetwork(spec2)
}

func TestNetwork_MultipleContainers(t *testing.T) {
	allocName := "test-multiple"

	d, _ := NewDockerDriver(nil)

	spec, _, err := d.CreateNetwork(allocName, "b")
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

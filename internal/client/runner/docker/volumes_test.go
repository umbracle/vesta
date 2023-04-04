package docker

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVolumes_Create(t *testing.T) {
	d := NewTestDockerDriver(t)

	created, err := d.CreateVolume("abc")
	require.NoError(t, err)
	require.True(t, created)

	created, err = d.CreateVolume("abc")
	require.NoError(t, err)
	require.False(t, created)

	d.DeleteVolume("abc")
}

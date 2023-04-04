package docker

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
	"github.com/umbracle/vesta/internal/uuid"
)

func NewTestDockerDriver(t *testing.T) *Docker {
	t.Helper()

	// create a custom name for the network
	testingNetworkName := uuid.Generate()

	d, err := NewDockerDriver(hclog.NewNullLogger(), testingNetworkName)
	require.NoError(t, err)

	// destroy the network afterwards
	t.Cleanup(func() {
		if err := d.client.NetworkRemove(context.Background(), testingNetworkName); err != nil {
			fmt.Printf("[ERROR]: failed to remove network: %s", testingNetworkName)
		}
	})

	return d
}

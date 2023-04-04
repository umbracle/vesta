package docker

import (
	"fmt"
	"sync"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
)

var netCreateOnce sync.Once

func NewTestDockerDriver(t *testing.T) *Docker {
	t.Helper()

	d, err := NewDockerDriver(hclog.NewNullLogger())
	require.NoError(t, err)

	netCreateOnce.Do(func() {
		fmt.Println("_ DO ONCE ? _")
	})

	return d
}

package docker

import (
	"sync"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
)

var lock sync.Mutex

func NewTestDockerDriver(t *testing.T) *Docker {
	t.Helper()

	lock.Lock()
	defer lock.Unlock()

	d, err := NewDockerDriver(hclog.NewNullLogger())
	require.NoError(t, err)

	return d
}

package docker

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
)

var lock sync.Mutex
var xxx uint64

func NewTestDockerDriver(t *testing.T) *Docker {
	t.Helper()

	fmt.Println("- is locked ? -", time.Now())
	defer func() {
		fmt.Println("after", time.Now())
	}()

	lock.Lock()
	fmt.Println("- in -")
	defer lock.Unlock()
	fmt.Println("- out -")
	d, err := NewDockerDriver(hclog.NewNullLogger())
	require.NoError(t, err)

	return d
}

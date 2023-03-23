package allocrunner

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/umbracle/vesta/internal/server/proto"
)

func TestRunner(t *testing.T) {
	r, err := NewRunner(&RConfig{})
	require.NoError(t, err)

	dep := &proto.Deployment1{
		Tasks: []*proto.Task1{
			{
				Image: "busybox",
				Tag:   "1.29.3",
				Args:  []string{"sleep", "30"},
			},
		},
	}
	r.UpsertDeployment(dep)

	time.Sleep(1 * time.Minute)
}

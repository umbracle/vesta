package b

import (
	"testing"

	"github.com/umbracle/vesta/internal/client/runner/docker"
)

func TestB(t *testing.T) {
	docker.NewTestDockerDriver(t)
}

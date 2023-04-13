package catalog

import (
	"testing"
)

func TestLighthouse(t *testing.T) {
	tr := newTestingFramework("lighthouse")
	tr.ImageExists(t)
}

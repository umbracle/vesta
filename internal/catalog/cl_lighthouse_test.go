package catalog

import (
	"testing"
)

func TestLighthouse(t *testing.T) {
	tr := newTestingFramework("lighthouse")
	tr.ImageExists(t, map[string]interface{}{
		"execution_node": "",
	})
}

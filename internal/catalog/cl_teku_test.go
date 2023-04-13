package catalog

import "testing"

func TestTeku(t *testing.T) {
	tr := newTestingFramework("teku")
	tr.ImageExists(t, map[string]interface{}{
		"execution_node": "",
	})
}

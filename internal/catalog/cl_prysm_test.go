package catalog

import "testing"

func TestPrysm(t *testing.T) {
	tr := newTestingFramework("prysm")
	tr.ImageExists(t, map[string]interface{}{
		"execution_node": "",
	})
}

package catalog

import "testing"

func TestNethermind(t *testing.T) {
	tr := newTestingFramework("nethermind")
	tr.ImageExists(t, map[string]interface{}{})
}

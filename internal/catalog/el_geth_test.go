package catalog

import "testing"

func TestGeth(t *testing.T) {
	tr := newTestingFramework("geth")
	tr.ImageExists(t)
}

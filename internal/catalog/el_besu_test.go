package catalog

import "testing"

func TestBesu(t *testing.T) {
	tr := newTestingFramework("besu")
	tr.ImageExists(t)
}

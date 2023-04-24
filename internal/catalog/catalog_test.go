package catalog

import "testing"

func TestImages(t *testing.T) {
	// test that all the images in the catalog exist
	for name := range Catalog {
		t.Run(name, func(t *testing.T) {
			tr := newTestingFramework(name)
			tr.ImageExists(t)
		})
	}
}

func TestStartup(t *testing.T) {
	//for name := range Catalog {
	//	t.Run(name, func(t *testing.T) {
	tr := newTestingFramework("prysm")
	tr.OnStartup(t)
	//	})
	//}
}

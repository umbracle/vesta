package allocrunner

import (
	"github.com/hashicorp/go-hclog"
	"github.com/umbracle/vesta/internal/client/runner/allocrunner/allocdir"
	"github.com/umbracle/vesta/internal/client/runner/driver"
	"github.com/umbracle/vesta/internal/client/runner/structs"
)

type volumeHook struct {
	logger   hclog.Logger
	driver   driver.Driver
	alloc    *structs.Allocation
	allocDir *allocdir.AllocDir
}

func newVolumeHook(logger hclog.Logger,
	driver driver.Driver,
	allocDir *allocdir.AllocDir,
	alloc *structs.Allocation,
) *volumeHook {
	n := &volumeHook{
		driver:   driver,
		alloc:    alloc,
		allocDir: allocDir,
	}
	n.logger = logger.Named(n.Name())
	return n
}

func (v *volumeHook) Name() string {
	return "volume-hook"
}

func (v *volumeHook) Prerun() error {
	return v.allocDir.Build()
}

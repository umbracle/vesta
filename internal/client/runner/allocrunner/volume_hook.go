package allocrunner

import (
	"fmt"

	"github.com/hashicorp/go-hclog"
	"github.com/umbracle/vesta/internal/client/runner/driver"
	"github.com/umbracle/vesta/internal/client/runner/structs"
)

type volumeHook struct {
	logger hclog.Logger
	driver driver.Driver
	alloc  *structs.Allocation
}

func newVolumeHook(logger hclog.Logger,
	driver driver.Driver,
	alloc *structs.Allocation,
) *volumeHook {
	n := &volumeHook{
		driver: driver,
		alloc:  alloc,
	}
	n.logger = logger.Named(n.Name())
	return n
}

func (v *volumeHook) Name() string {
	return "volume-hook"
}

func (v *volumeHook) Prerun() error {
	// gather all the volumes and create them on the driver
	for _, task := range v.alloc.Deployment.Tasks {
		for name := range task.Volumes {
			volumeName := fmt.Sprintf("%s-%s-%s", v.alloc.Deployment.Name, task.Name, name)

			if _, err := v.driver.CreateVolume(volumeName); err != nil {
				return err
			}
		}
	}
	return nil
}

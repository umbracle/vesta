package allocrunner

import (
	"github.com/hashicorp/go-hclog"
	"github.com/umbracle/vesta/internal/client/runner/driver"
	"github.com/umbracle/vesta/internal/client/runner/structs"
)

type networkStatusSetter interface {
	SetNetworkSpec(*structs.NetworkSpec)
}

type networkHook struct {
	logger              hclog.Logger
	driver              driver.Driver
	alloc               *structs.Allocation
	networkStatusSetter networkStatusSetter
	spec                *structs.NetworkSpec
}

func newNetworkHook(logger hclog.Logger,
	driver driver.Driver,
	alloc *structs.Allocation,
	networkStatusSetter networkStatusSetter,
) *networkHook {
	n := &networkHook{
		driver:              driver,
		alloc:               alloc,
		networkStatusSetter: networkStatusSetter,
	}
	n.logger = logger.Named(n.Name())
	return n
}

func (n *networkHook) Name() string {
	return "network-hook"
}

func (n *networkHook) Prerun() error {
	spec, _, err := n.driver.CreateNetwork(n.alloc.Deployment.Name, []string{n.alloc.Deployment.Alias}, n.alloc.Deployment.Name)
	if err != nil {
		return err
	}

	if spec != nil {
		n.spec = spec
		n.networkStatusSetter.SetNetworkSpec(spec)
	}
	return nil
}

func (n *networkHook) Postrun() error {
	if n.spec == nil {
		return nil
	}

	return n.driver.DestroyNetwork(n.spec)
}

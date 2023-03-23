package allocrunner

import (
	"fmt"

	"github.com/hashicorp/go-hclog"
	dto "github.com/prometheus/client_model/go"
	"github.com/umbracle/vesta/internal/client/allocrunner/docker"
	"github.com/umbracle/vesta/internal/client/allocrunner/state"
	"github.com/umbracle/vesta/internal/server/proto"
)

type RConfig struct {
	Volume *HostVolume
}

type HostVolume struct {
	Path string
}

type Runner struct {
	config *RConfig
	logger hclog.Logger
	state  state.State
	driver *docker.Docker
	allocs map[string]*AllocRunner
}

func NewRunner(config *RConfig) (*Runner, error) {
	driver, err := docker.NewDockerDriver(hclog.NewNullLogger())
	if err != nil {
		return nil, err
	}

	r := &Runner{
		driver: driver,
	}

	if err := r.initState(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Runner) initState() error {
	state, err := state.NewBoltdbStore("client.db")
	if err != nil {
		return err
	}
	r.state = state

	allocs, err := state.GetAllocations()
	if err != nil {
		return err
	}
	for _, alloc := range allocs {
		id := alloc.Id

		config := &Config{
			Alloc:         alloc,
			Logger:        r.logger,
			State:         r.state,
			StateUpdater:  r,
			Driver:        r.driver,
			UpdateMetrics: r,
		}
		if r.config.Volume != nil {
			config.Volume = r.config.Volume.Path
		}

		handle, err := NewAllocRunner(config)
		if err != nil {
			panic(err)
		}
		r.allocs[id] = handle

		if err := handle.Restore(); err != nil {
			return err
		}
	}

	for _, a := range r.allocs {
		go a.Run()
	}

	return nil
}

func (r *Runner) UpdateMetrics(string, map[string]*dto.MetricFamily) {
	// TODO: Move hooks out
}

func (r *Runner) AllocStateUpdated(a *proto.Allocation) {
	fmt.Println(a)
}

func (r *Runner) UpsertDeployment(alloc *proto.Allocation) {
	handle, ok := r.allocs[alloc.Id]
	if ok {
		if alloc.Sequence > handle.alloc.Sequence {
			// update
			handle.Update(alloc)
		}
	} else {
		// create
		config := &Config{
			Alloc:         alloc,
			Logger:        r.logger,
			State:         r.state,
			StateUpdater:  r,
			Driver:        r.driver,
			UpdateMetrics: r,
		}
		if r.config.Volume != nil {
			config.Volume = r.config.Volume.Path
		}
		var err error
		if handle, err = NewAllocRunner(config); err != nil {
			panic(err)
		}

		r.allocs[alloc.Id] = handle
		go handle.Run()
	}

	// update allocation
	if err := r.state.PutAllocation(alloc); err != nil {
		panic(err)
	}
}

package allocrunner

import (
	"fmt"

	"github.com/hashicorp/go-hclog"
	dto "github.com/prometheus/client_model/go"
	"github.com/umbracle/vesta/internal/client/allocrunner/docker"
	"github.com/umbracle/vesta/internal/client/allocrunner/state"
	"github.com/umbracle/vesta/internal/server/proto"
	"github.com/umbracle/vesta/internal/uuid"
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

	logger := hclog.New(&hclog.LoggerOptions{Level: hclog.Info})

	r := &Runner{
		logger: logger,
		config: config,
		driver: driver,
		allocs: map[string]*AllocRunner{},
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
		r.allocs[alloc.Deployment.Name] = handle

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

func (r *Runner) AllocStateUpdated(a *proto.Allocation1) {
	fmt.Println(a)
}

func (r *Runner) UpsertDeployment(deployment *proto.Deployment1) {
	handle, ok := r.allocs[deployment.Name]
	if ok {
		if deployment.Sequence > handle.alloc.Deployment.Sequence {
			// update
			handle.Update(deployment)
		}

		// TODO: Save the allocation again

	} else {
		// create
		alloc := &proto.Allocation1{
			Id:            uuid.Generate(),
			Deployment:    deployment,
			DesiredStatus: proto.Allocation1_Run,
		}

		if err := r.state.PutAllocation(alloc); err != nil {
			panic(err)
		}

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
}

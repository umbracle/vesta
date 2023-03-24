package runner

import (
	"github.com/hashicorp/go-hclog"
	dto "github.com/prometheus/client_model/go"
	"github.com/umbracle/vesta/internal/client/runner/allocrunner"
	"github.com/umbracle/vesta/internal/client/runner/docker"
	"github.com/umbracle/vesta/internal/client/runner/hooks"
	"github.com/umbracle/vesta/internal/client/runner/state"
	"github.com/umbracle/vesta/internal/server/proto"
)

type Config struct {
	Logger hclog.Logger
	Volume *HostVolume
}

type HostVolume struct {
	Path string
}

type Runner struct {
	config *Config
	logger hclog.Logger
	state  state.State
	driver *docker.Docker
	allocs map[string]*allocrunner.AllocRunner
	hooks  []hooks.TaskHookFactory
}

func NewRunner(config *Config) (*Runner, error) {
	driver, err := docker.NewDockerDriver(config.Logger)
	if err != nil {
		return nil, err
	}

	r := &Runner{
		logger: config.Logger,
		config: config,
		driver: driver,
		allocs: map[string]*allocrunner.AllocRunner{},
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
		config := &allocrunner.Config{
			Alloc:        alloc,
			Logger:       r.logger,
			State:        r.state,
			StateUpdater: r,
			Driver:       r.driver,
		}
		if r.config.Volume != nil {
			config.Volume = r.config.Volume.Path
		}

		handle, err := allocrunner.NewAllocRunner(config)
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
	// TODO
}

func (r *Runner) UpsertDeployment(deployment *proto.Deployment1) {
	handle, ok := r.allocs[deployment.Name]
	if ok {
		if deployment.Sequence > handle.Alloc().Deployment.Sequence {
			// update
			handle.Update(deployment)
		}

		// TODO: Save the allocation again (handle race)
		if err := r.state.PutAllocation(handle.Alloc()); err != nil {
			panic(err)
		}

	} else {
		// create
		alloc := &proto.Allocation1{
			Deployment:    deployment,
			DesiredStatus: proto.Allocation1_Run,
		}

		if err := r.state.PutAllocation(alloc); err != nil {
			panic(err)
		}

		config := &allocrunner.Config{
			Alloc:        alloc,
			Logger:       r.logger,
			State:        r.state,
			StateUpdater: r,
			Driver:       r.driver,
		}
		if r.config.Volume != nil {
			config.Volume = r.config.Volume.Path
		}
		var err error
		if handle, err = allocrunner.NewAllocRunner(config); err != nil {
			panic(err)
		}

		r.allocs[alloc.Deployment.Name] = handle
		go handle.Run()
	}
}

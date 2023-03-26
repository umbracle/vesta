package runner

import (
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/umbracle/vesta/internal/client/runner/allocrunner"
	"github.com/umbracle/vesta/internal/client/runner/docker"
	"github.com/umbracle/vesta/internal/client/runner/hooks"
	"github.com/umbracle/vesta/internal/client/runner/state"
	proto "github.com/umbracle/vesta/internal/client/runner/structs"
)

type Config struct {
	Logger            hclog.Logger
	Volume            *HostVolume
	AllocStateUpdated allocrunner.StateUpdater
	State             state.State
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
	logger := config.Logger
	if logger == nil {
		logger = hclog.NewNullLogger()
	}

	driver, err := docker.NewDockerDriver(logger)
	if err != nil {
		return nil, err
	}

	r := &Runner{
		logger: logger,
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
	if r.config.State != nil {
		r.state = r.config.State
	} else {
		state, err := state.NewBoltdbStore("client.db")
		if err != nil {
			return err
		}
		r.state = state
	}

	allocs, err := r.state.GetAllocations()
	if err != nil {
		return err
	}
	for _, alloc := range allocs {
		config := &allocrunner.Config{
			Alloc:        alloc,
			Logger:       r.logger,
			State:        r.state,
			StateUpdater: r.config.AllocStateUpdated,
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

func (r *Runner) UpsertDeployment(deployment *proto.Deployment) {
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
		alloc := &proto.Allocation{
			Deployment: deployment,
		}

		if err := r.state.PutAllocation(alloc); err != nil {
			panic(err)
		}

		config := &allocrunner.Config{
			Alloc:        alloc,
			Logger:       r.logger,
			State:        r.state,
			StateUpdater: r.config.AllocStateUpdated,
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

func (r *Runner) Shutdown() error {
	wg := sync.WaitGroup{}
	for _, tr := range r.allocs {
		wg.Add(1)
		go func(tr *allocrunner.AllocRunner) {
			tr.Shutdown()
			<-tr.ShutdownCh()
			wg.Done()
		}(tr)
	}
	wg.Wait()

	if err := r.state.Close(); err != nil {
		return err
	}
	return nil
}

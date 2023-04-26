package runner

import (
	"io/ioutil"
	"reflect"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/umbracle/vesta/internal/client/runner/allocrunner"
	"github.com/umbracle/vesta/internal/client/runner/docker"
	"github.com/umbracle/vesta/internal/client/runner/hooks"
	"github.com/umbracle/vesta/internal/client/runner/state"
	"github.com/umbracle/vesta/internal/client/runner/structs"
	proto "github.com/umbracle/vesta/internal/client/runner/structs"
)

type Config struct {
	Logger            hclog.Logger
	Volume            *HostVolume
	AllocStateUpdated allocrunner.StateUpdater
	State             state.State
	Hooks             []hooks.TaskHookFactory
}

type HostVolume struct {
	Path string
}

type Runner struct {
	config      *Config
	logger      hclog.Logger
	state       state.State
	driver      *docker.Docker
	allocs      map[string]*allocrunner.AllocRunner
	deployments map[string]*structs.Deployment
	hooks       []hooks.TaskHookFactory
	closeCh     chan struct{}
	reconcileCh chan struct{}
}

func NewRunner(config *Config) (*Runner, error) {
	logger := config.Logger
	if logger == nil {
		logger = hclog.NewNullLogger()
	}

	if config.Volume == nil {
		tmpDir, err := ioutil.TempDir("/tmp", "vesta-")
		if err != nil {
			return nil, err
		}
		config.Volume = &HostVolume{
			Path: tmpDir,
		}
		logger.Info("volume not set, using temporal location", "path", tmpDir)
	} else {
		logger.Info("volume path set", "path", config.Volume.Path)
	}

	driver, err := docker.NewDockerDriver(logger, "vesta")
	if err != nil {
		return nil, err
	}

	r := &Runner{
		logger:      logger,
		config:      config,
		driver:      driver,
		allocs:      map[string]*allocrunner.AllocRunner{},
		hooks:       config.Hooks,
		closeCh:     make(chan struct{}),
		deployments: map[string]*proto.Deployment{},
		reconcileCh: make(chan struct{}),
	}

	if err := r.initState(); err != nil {
		return nil, err
	}

	go r.reconcile()

	return r, nil
}

func (r *Runner) AllocStateUpdated(alloc *proto.Allocation) {
	select {
	case r.reconcileCh <- struct{}{}:
	default:
	}

	r.config.AllocStateUpdated.AllocStateUpdated(alloc)
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
			Alloc:           alloc,
			Logger:          r.logger,
			State:           r.state,
			StateUpdater:    r,
			Driver:          r.driver,
			Hooks:           r.hooks,
			ClientVolumeDir: r.config.Volume.Path,
		}

		handle := allocrunner.NewAllocRunner(config)
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

func (r *Runner) reconcile() {
	for {
		select {
		case <-r.reconcileCh:
		case <-r.closeCh:
			return
		}

		r.handleReconcile()
	}
}

func (r *Runner) handleReconcile() {
	allocsByDep := map[string]*allocrunner.AllocRunner{}
	for _, alloc := range r.allocs {
		state := alloc.Status()

		if state.Status != proto.Allocation_Complete {
			// TODO: we should fail if there are two for the same deployment
			allocsByDep[alloc.Deployment().Name] = alloc
		}
	}

	create := []*proto.Deployment{}
	remove := []*allocrunner.AllocRunner{}

	// reconcile state
	for _, dep := range r.deployments {
		alloc, ok := allocsByDep[dep.Name]
		if !ok {
			// allocation not found, create it
			if dep.DesiredStatus == proto.Deployment_Run {
				create = append(create, dep)
			}
		} else {
			allocDep := alloc.Deployment()

			if dep.DesiredStatus == proto.Deployment_Stop {
				if allocDep.DesiredStatus == proto.Deployment_Run {
					remove = append(remove, alloc)
				}
			} else {
				// alloc found, figure out if we need to update
				if deploymentUpdated(dep, allocDep) {
					// it requires an update, schedule for removal if its
					// desired status is still running. Otherwise, it has been
					// already scheduled for removal in a previous iteration.
					if allocDep.DesiredStatus != proto.Deployment_Stop {
						remove = append(remove, alloc)
					}
				}
			}
		}
	}

	for _, dep := range create {
		alloc := &proto.Allocation{
			Deployment: dep,
		}

		if err := r.state.PutAllocation(alloc); err != nil {
			panic(err)
		}

		config := &allocrunner.Config{
			Alloc:           alloc,
			Logger:          r.logger,
			State:           r.state,
			StateUpdater:    r,
			Driver:          r.driver,
			Hooks:           r.hooks,
			ClientVolumeDir: r.config.Volume.Path,
		}

		handle := allocrunner.NewAllocRunner(config)

		r.allocs[alloc.Deployment.Name] = handle
		go handle.Run()
	}

	for _, del := range remove {
		deployment := del.Alloc().Copy().Deployment
		deployment.DesiredStatus = proto.Deployment_Stop

		del.Update(deployment)
	}
}

func (r *Runner) UpsertDeployment(newDeployment *proto.Deployment) {
	dep, ok := r.deployments[newDeployment.Name]
	if ok {
		if newDeployment.Sequence <= dep.Sequence {
			return
		}
	}

	r.deployments[newDeployment.Name] = newDeployment

	select {
	case r.reconcileCh <- struct{}{}:
	default:
	}
}

func (r *Runner) Shutdown() error {
	close(r.closeCh)

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

func deploymentUpdated(newDep, oldDep *proto.Deployment) bool {
	if len(newDep.Tasks) != len(oldDep.Tasks) {
		return true
	}

	for indx, task0 := range newDep.Tasks {
		task1 := oldDep.Tasks[indx]
		if tasksUpdated(task0, task1) {
			return true
		}
	}

	return false
}

func tasksUpdated(a, b *proto.Task) bool {
	if !reflect.DeepEqual(a.Image, b.Image) {
		return true
	}
	if !reflect.DeepEqual(a.Tag, b.Tag) {
		return true
	}
	if !reflect.DeepEqual(a.Args, b.Args) {
		return true
	}
	return false
}

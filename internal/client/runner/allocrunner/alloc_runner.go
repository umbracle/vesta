package allocrunner

import (
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/umbracle/vesta/internal/client/runner/allocrunner/allocdir"
	"github.com/umbracle/vesta/internal/client/runner/allocrunner/taskrunner"
	"github.com/umbracle/vesta/internal/client/runner/driver"
	"github.com/umbracle/vesta/internal/client/runner/hooks"
	"github.com/umbracle/vesta/internal/client/runner/state"
	proto "github.com/umbracle/vesta/internal/client/runner/structs"
)

type StateUpdater interface {
	AllocStateUpdated(alloc *proto.Allocation)
}

type Config struct {
	Logger          hclog.Logger
	Alloc           *proto.Allocation
	State           state.State
	StateUpdater    StateUpdater
	Driver          driver.Driver
	ClientVolumeDir string
	Hooks           []hooks.TaskHookFactory
}

type AllocRunner struct {
	config          *Config
	logger          hclog.Logger
	tasks           map[string]*taskrunner.TaskRunner
	waitCh          chan struct{}
	alloc           *proto.Allocation
	driver          driver.Driver
	taskUpdated     chan struct{}
	stateUpdater    StateUpdater
	state           state.State
	allocUpdatedCh  chan *proto.Allocation
	volume          string
	shutdownStarted chan struct{}
	shutdownCh      chan struct{}
	destroyCh       chan struct{}
	wg              sync.WaitGroup
	allocDir        *allocdir.AllocDir
	runnerHooks     []hooks.RunnerHook
}

func NewAllocRunner(c *Config) (*AllocRunner, error) {
	logger := c.Logger.Named("alloc_runner").With("alloc", c.Alloc.Deployment.Name)

	runner := &AllocRunner{
		config:          c,
		logger:          logger,
		tasks:           map[string]*taskrunner.TaskRunner{},
		waitCh:          make(chan struct{}),
		alloc:           c.Alloc,
		driver:          c.Driver,
		taskUpdated:     make(chan struct{}),
		stateUpdater:    c.StateUpdater,
		state:           c.State,
		volume:          c.ClientVolumeDir,
		shutdownStarted: make(chan struct{}),
		shutdownCh:      make(chan struct{}),
		destroyCh:       make(chan struct{}),
		allocUpdatedCh:  make(chan *proto.Allocation, 1),
		runnerHooks:     []hooks.RunnerHook{},
		allocDir:        allocdir.NewAllocDir(c.ClientVolumeDir, c.Alloc.Deployment.Name),
	}

	if err := runner.initHooks(); err != nil {
		return nil, err
	}

	return runner, nil
}

func (a *AllocRunner) Deployment() *proto.Deployment {
	return a.alloc.Deployment.Copy()
}

func (a *AllocRunner) SetNetworkSpec(spec *proto.NetworkSpec) {
	a.alloc.NetworkSpec = spec
}

func (a *AllocRunner) ShutdownCh() chan struct{} {
	return a.shutdownCh
}

func (a *AllocRunner) Alloc() *proto.Allocation {
	return a.alloc
}

func (a *AllocRunner) handleTaskStateUpdates() {

	// start any tasks that were started during Restore
	for _, task := range a.tasks {
		a.wg.Add(1)
		go func(runner *taskrunner.TaskRunner) {
			runner.Run()
			a.wg.Done()
		}(task)
	}

	// start the reconcile loop
	for {
		tasks := map[string]*proto.Task{}
		tasksState := map[string]*proto.TaskState{}
		for name, t := range a.tasks {
			tasks[name] = t.Task()
			tasksState[name] = t.TaskState()
		}

		res := &allocResults{}
		if !a.isShuttingDown() {
			// if the alloc runner is shutting down, the tasks have been removed
			// and the reconciler would try to allocate them again.
			r := newAllocReconciler(a.alloc, tasks, tasksState)
			res = r.Compute()
		}

		if !res.Empty() {
			a.logger.Info(res.GoString())

			// remove tasks
			for _, name := range res.removeTasks {
				a.tasks[name].KillNoWait(proto.NewTaskEvent(""))
			}

			// create a tasks
			for name, task := range res.newTasks {
				// write the task on the state
				runner := a.newTaskRunner(task)

				a.wg.Add(1)
				go func(runner *taskrunner.TaskRunner) {
					runner.Run()
					a.wg.Done()
				}(runner)

				a.tasks[name] = runner
			}
		}

		states := map[string]*proto.TaskState{}
		for taskName, task := range a.tasks {
			state := task.TaskState()
			if state.State == proto.TaskState_Dead && !a.isShuttingDown() {
				// garbage collect the task if it has finished and not shutting down
				delete(a.tasks, taskName)
			} else {
				states[taskName] = task.TaskState()
			}
		}

		// Notify about the update on the allocation
		calloc := a.clientAlloc(states)

		// Update the server
		a.stateUpdater.AllocStateUpdated(calloc)

		// wait for more updates
		select {
		case <-a.taskUpdated:
		}
	}
}

func (a *AllocRunner) Run() {
	if err := a.prerun(); err != nil {
		a.logger.Error("prerun failed", "error", err)

		goto POST
	}

	go a.handleAllocUpdates()

	go a.handleTaskStateUpdates()

	// wait for the shutdown to start and wait for the tasks to finish
	<-a.shutdownStarted

	a.wg.Wait()

POST:
	// Run the postrun hooks
	if err := a.postrun(); err != nil {
		a.logger.Error("postrun failed", "error", err)
	}

	close(a.waitCh)
}

func (a *AllocRunner) handleAllocUpdates() {
	for {
		select {
		case update := <-a.allocUpdatedCh:
			a.handleAllocUpdate(update)

		case <-a.waitCh:
			return
		}
	}
}

func (a *AllocRunner) handleAllocUpdate(alloc *proto.Allocation) {
	a.alloc = alloc

	// update the tasks
	a.TaskStateUpdated()

	if alloc.Deployment.DesiredStatus == proto.Deployment_Stop {
		close(a.shutdownStarted)

		// destroy all tasks
		for _, task := range a.tasks {
			task.KillNoWait(proto.NewTaskEvent(""))
		}

		// wait for all the tasks to finish
		<-a.waitCh
	}
}

func (a *AllocRunner) AllocStatus() proto.Allocation_Status {
	states := map[string]*proto.TaskState{}
	for name, task := range a.tasks {
		states[name] = task.TaskState()
	}
	return getClientStatus(states)
}

func (a *AllocRunner) clientAlloc(states map[string]*proto.TaskState) *proto.Allocation {
	// Notify about the update on the allocation
	calloc := a.alloc.Copy()
	calloc.TaskStates = states

	// TODO: Measure also pending tasks to be created
	calloc.Status = getClientStatus(states)

	return calloc
}

func (a *AllocRunner) newTaskRunner(task *proto.Task) *taskrunner.TaskRunner {
	config := &taskrunner.Config{
		Logger:           a.logger,
		Task:             task,
		Allocation:       a.config.Alloc,
		Driver:           a.config.Driver,
		State:            a.config.State,
		TaskStateUpdated: a.TaskStateUpdated,
		Hooks:            a.config.Hooks,
		TaskDir:          a.allocDir.NewTaskDir(task.Name),
	}

	return taskrunner.NewTaskRunner(config)
}

func (a *AllocRunner) TaskStateUpdated() {
	select {
	case a.taskUpdated <- struct{}{}:
	default:
	}
}

func (a *AllocRunner) Restore() error {
	// read from db the tasks?
	for _, task := range a.alloc.Deployment.Tasks {
		runner := a.newTaskRunner(task)
		a.tasks[task.Name] = runner

		if err := runner.Restore(); err != nil {
			return err
		}
	}
	return nil
}

func (a *AllocRunner) isShuttingDown() bool {
	select {
	case <-a.shutdownStarted:
		return true
	default:
		return false
	}
}

func (a *AllocRunner) DestroyCh() chan struct{} {
	return a.destroyCh
}

func (a *AllocRunner) Destroy() {
	a.logger.Info("alloc destroyed")
	close(a.shutdownStarted)

	go func() {
		// Kill the tasks and update the allocation status
		for _, task := range a.tasks {
			task.KillNoWait(proto.NewTaskEvent(""))
		}

		// wait for all the tasks to finish
		<-a.waitCh

		// delete the allocation folder
		if err := a.state.DeleteAllocationBucket(a.alloc.Deployment.Name); err != nil {
			a.logger.Error("failed to delete allocation", "err", err)
		}

		close(a.destroyCh)
	}()
}

func (a *AllocRunner) Update(deployment *proto.Deployment) {
	a.logger.Info("alloc updated")

	alloc := a.alloc.Copy()
	alloc.Deployment = deployment

	select {
	// Drain queued update from the channel if possible, and check the modify
	// index
	case oldUpdate := <-a.allocUpdatedCh:
		// If the old update is newer than the replacement, then skip the new one
		// and return
		if oldUpdate.Deployment.Sequence > alloc.Deployment.Sequence {
			a.allocUpdatedCh <- oldUpdate
			return
		}

	case <-a.waitCh:
		return
	default:
	}

	// Queue the new update
	a.allocUpdatedCh <- alloc
}

func (a *AllocRunner) WaitCh() <-chan struct{} {
	return a.waitCh
}

func (a *AllocRunner) Shutdown() {
	close(a.shutdownStarted)

	go func() {
		a.logger.Trace("shutting down")

		// Shutdown tasks gracefully if they were run
		wg := sync.WaitGroup{}
		for _, tr := range a.tasks {
			wg.Add(1)
			go func(tr *taskrunner.TaskRunner) {
				tr.Shutdown()
				wg.Done()
			}(tr)
		}
		wg.Wait()

		// Wait for Run to exit
		<-a.waitCh
		close(a.shutdownCh)
	}()
}

// getClientStatus takes in the task states for a given allocation and computes
// the client status and description
func getClientStatus(taskStates map[string]*proto.TaskState) proto.Allocation_Status {
	var pending, running, dead, failed bool
	for _, state := range taskStates {
		switch state.State {
		case proto.TaskState_Running:
			running = true
		case proto.TaskState_Pending:
			pending = true
		case proto.TaskState_Dead:
			if state.Failed {
				failed = true
			} else {
				dead = true
			}
		}
	}

	// Determine the alloc status
	if failed {
		return proto.Allocation_Failed
	} else if pending {
		return proto.Allocation_Pending
	} else if running {
		return proto.Allocation_Running
	} else if dead {
		return proto.Allocation_Complete
	}

	return proto.Allocation_Complete
}

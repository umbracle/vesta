package allocrunner

import (
	"context"
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
	config                   *Config
	logger                   hclog.Logger
	tasks                    map[string]*taskrunner.TaskRunner
	waitCh                   chan struct{}
	alloc                    *proto.Allocation
	driver                   driver.Driver
	taskUpdated              chan struct{}
	stateUpdater             StateUpdater
	state                    state.State
	allocUpdatedCh           chan *proto.Allocation
	volume                   string
	shutdownStarted          chan struct{}
	shutdownCh               chan struct{}
	destroyCh                chan struct{}
	taskStateUpdateHandlerCh chan struct{}
	wg                       sync.WaitGroup
	allocDir                 *allocdir.AllocDir
	runnerHooks              []hooks.RunnerHook
}

func NewAllocRunner(c *Config) *AllocRunner {
	logger := c.Logger.Named("alloc_runner").With("alloc", c.Alloc.Deployment.Name)

	runner := &AllocRunner{
		config:                   c,
		logger:                   logger,
		tasks:                    map[string]*taskrunner.TaskRunner{},
		waitCh:                   make(chan struct{}),
		alloc:                    c.Alloc,
		driver:                   c.Driver,
		taskUpdated:              make(chan struct{}),
		stateUpdater:             c.StateUpdater,
		state:                    c.State,
		volume:                   c.ClientVolumeDir,
		shutdownStarted:          make(chan struct{}),
		shutdownCh:               make(chan struct{}),
		destroyCh:                make(chan struct{}),
		taskStateUpdateHandlerCh: make(chan struct{}),
		allocUpdatedCh:           make(chan *proto.Allocation, 1),
		runnerHooks:              []hooks.RunnerHook{},
		allocDir:                 allocdir.NewAllocDir(c.ClientVolumeDir, c.Alloc.Deployment.Name),
	}

	runner.initTaskRunners(c.Alloc.Deployment.Tasks)
	runner.initHooks()

	return runner
}

func (a *AllocRunner) initTaskRunners(tasks []*proto.Task) error {
	for _, task := range tasks {
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
		runner := taskrunner.NewTaskRunner(config)
		a.tasks[task.Name] = runner
	}
	return nil
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
	defer close(a.taskStateUpdateHandlerCh)

	for done := false; !done; {
		// wait for more updates
		select {
		case <-a.taskUpdated:
		case <-a.waitCh:
			// sync once more to collect the final states
			done = true
		}

		liveRunners := []*taskrunner.TaskRunner{}

		var killEvent *proto.TaskState_Event
		var killTask string

		states := map[string]*proto.TaskState{}
		for name, t := range a.tasks {
			taskState := t.TaskState()
			states[name] = taskState

			if taskState.State != proto.TaskState_Dead {
				liveRunners = append(liveRunners, t)
				continue
			}

			if taskState.Failed {
				if killEvent == nil {
					killTask = name
					killEvent = proto.NewTaskEvent(proto.TaskSiblingFailed).
						SetTaskFailed(killTask)
				}
			}
		}

		if len(liveRunners) > 0 {
			if killEvent != nil {
				// kill the live tasks
				for _, tr := range liveRunners {
					tr.EmitEvent(killEvent)
				}

				states = a.killTasks()

				// wait for the liverunners to stop
				for _, tr := range liveRunners {
					a.logger.Info("waiting for task to exit", "task", tr.Task().Name)
					select {
					case <-tr.WaitCh():
					case <-a.waitCh:
					}
				}
			}
		}

		calloc := a.clientAlloc(states)

		a.stateUpdater.AllocStateUpdated(calloc)
	}
}

func (a *AllocRunner) runTasks() {
	// Start and wait for all tasks.
	for _, task := range a.tasks {
		go task.Run()
	}
	for _, task := range a.tasks {
		<-task.WaitCh()
	}
}

func (ar *AllocRunner) killTasks() map[string]*proto.TaskState {
	var mu sync.Mutex
	states := make(map[string]*proto.TaskState, len(ar.tasks))

	wg := sync.WaitGroup{}
	for name, tr := range ar.tasks {
		wg.Add(1)
		go func(name string, tr *taskrunner.TaskRunner) {
			defer wg.Done()

			taskEvent := proto.NewTaskEvent(proto.TaskKilling)
			err := tr.Kill(context.TODO(), taskEvent)
			if err != nil { // TODO (what if the task is not running anymore)
				ar.logger.Warn("error stopping task", "error", err, "task_name", name)
			}

			taskState := tr.TaskState()
			mu.Lock()
			states[name] = taskState
			mu.Unlock()
		}(name, tr)
	}
	wg.Wait()

	return states
}

func (a *AllocRunner) Run() {
	defer close(a.waitCh)

	go a.handleAllocUpdates()

	go a.handleTaskStateUpdates()

	if err := a.prerun(); err != nil {
		a.logger.Error("prerun failed", "error", err)

		goto POST
	}

	a.runTasks()

POST:
	if a.isShuttingDown() {
		return
	}

	// Run the postrun hooks
	if err := a.postrun(); err != nil {
		a.logger.Error("postrun failed", "error", err)
	}
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
		a.killTasks()
	}
}

type Status struct {
	Status proto.Allocation_Status
	States map[string]*proto.TaskState
}

func (a *AllocRunner) Status() *Status {
	states := map[string]*proto.TaskState{}
	for name, task := range a.tasks {
		states[name] = task.TaskState()
	}

	res := &Status{Status: getClientStatus(states), States: states}

	select {
	case <-a.waitCh:
	default:
		// wait is not over yet
		if res.Status == proto.Allocation_Complete {
			// post run task have not finished yet adn the task are still running
			// wait until everything stops to assert it is completed
			res.Status = proto.Allocation_Running
		}
	}

	return res
}

func (a *AllocRunner) clientAlloc(states map[string]*proto.TaskState) *proto.Allocation {
	// Notify about the update on the allocation
	calloc := a.alloc.Copy()
	calloc.TaskStates = states

	calloc.Status = getClientStatus(states)

	select {
	case <-a.waitCh:
	default:
		// wait is not over yet
		if calloc.Status == proto.Allocation_Complete {
			// post run task have not finished yet adn the task are still running
			// wait until everything stops to assert it is completed
			calloc.Status = proto.Allocation_Running
		}
	}

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
	for _, runner := range a.tasks {
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

	go func() {
		states := a.killTasks()
		calloc := a.clientAlloc(states)
		a.stateUpdater.AllocStateUpdated(calloc)

		<-a.waitCh

		<-a.taskStateUpdateHandlerCh

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

	return proto.Allocation_Unknown
}

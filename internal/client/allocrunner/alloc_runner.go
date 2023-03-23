package allocrunner

import (
	"os"
	"path/filepath"

	dto "github.com/prometheus/client_model/go"

	"github.com/hashicorp/go-hclog"
	"github.com/umbracle/vesta/internal/client/allocrunner/driver"
	"github.com/umbracle/vesta/internal/client/allocrunner/state"
	"github.com/umbracle/vesta/internal/client/allocrunner/taskrunner"
	"github.com/umbracle/vesta/internal/server/proto"
)

type MetricsUpdater interface {
	UpdateMetrics(string, map[string]*dto.MetricFamily)
}

type StateUpdater interface {
	AllocStateUpdated(alloc *proto.Allocation1)
}

type Config struct {
	Logger       hclog.Logger
	Alloc        *proto.Allocation1
	State        state.State
	StateUpdater StateUpdater
	Driver       driver.Driver
	Volume       string

	UpdateMetrics MetricsUpdater
}

type AllocRunner struct {
	config       *Config
	logger       hclog.Logger
	tasks        map[string]*taskrunner.TaskRunner
	waitCh       chan struct{}
	alloc        *proto.Allocation1
	driver       driver.Driver
	taskUpdated  chan struct{}
	stateUpdater StateUpdater
	volume       string

	updateMetrics MetricsUpdater
}

func NewAllocRunner(c *Config) (*AllocRunner, error) {
	logger := c.Logger.Named("alloc_runner").With("alloc", c.Alloc.Id)

	runner := &AllocRunner{
		config:        c,
		logger:        logger,
		tasks:         map[string]*taskrunner.TaskRunner{},
		waitCh:        make(chan struct{}),
		alloc:         c.Alloc,
		driver:        c.Driver,
		taskUpdated:   make(chan struct{}),
		stateUpdater:  c.StateUpdater,
		volume:        c.Volume,
		updateMetrics: c.UpdateMetrics,
	}
	return runner, nil
}

func (a *AllocRunner) Run() {

	// start any task that was restored
	for _, task := range a.tasks {
		go task.Run()
	}

	// start the reconcile loop
	for {

		tasks := map[string]*proto.Task1{}
		tasksState := map[string]*proto.TaskState{}
		taskPending := map[string]struct{}{}
		for name, t := range a.tasks {
			tasks[name] = t.Task()
			tasksState[name] = t.TaskState()
			if t.IsShuttingDown() {
				taskPending[name] = struct{}{}
			}
		}

		r := newAllocReconciler(a.alloc, tasks, tasksState, taskPending)
		res := r.Compute()

		a.logger.Info(res.GoString())

		// remove tasks
		for _, name := range res.removeTasks {
			a.tasks[name].KillNoWait()
		}

		/*
			// create a tasks
			for name, task := range res.newTasks {
				// write the task on the state
				if err := a.config.State.PutTaskSpec(a.alloc.Id, task); err != nil {
					panic(err)
				}

				fmt.Println(name)

				//runner := a.newTaskRunner(task)
				//go runner.Run()

				//a.tasks[name] = runner
			}
		*/

		states := map[string]*proto.TaskState{}
		for taskName, task := range a.tasks {
			states[taskName] = task.TaskState()
		}

		// Notify about the update on the allocation
		calloc := a.alloc.Copy()
		calloc.TaskStates = states

		// TODO: Measure also pending tasks to be created
		calloc.Status = getClientStatus(states)

		// Update the server
		a.stateUpdater.AllocStateUpdated(calloc)

		// wait for more updates
		select {
		case <-a.taskUpdated:
		}
	}
}

func (a *AllocRunner) newTaskRunner(task *proto.Task1) *taskrunner.TaskRunner {
	config := &taskrunner.Config{
		Logger:           a.logger,
		Task:             task,
		Allocation:       a.config.Alloc,
		Driver:           a.config.Driver,
		State:            a.config.State,
		TaskStateUpdated: a.TaskStateUpdated,
		MetricsUpdater:   a.updateMetrics,
	}

	if a.volume != "" {
		// create an alloc dir
		taskAllocDir := filepath.Join(a.volume, a.alloc.Id, task.Name)
		if err := os.MkdirAll(taskAllocDir, 0755); err != nil {
			// TODO
			panic(err)
		}
		config.AllocDir = taskAllocDir
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
	/*
		tasks, err := a.config.State.GetAllocationTasks(a.alloc.Id)
		if err != nil {
			return err
		}
	*/
	for _, task := range a.alloc.Tasks {
		runner := a.newTaskRunner(task)
		a.tasks[task.Name] = runner

		if err := runner.Restore(); err != nil {
			return err
		}
		if runner.TaskState().State == proto.TaskState_Dead {
			// do not load dead tasks
			delete(a.tasks, task.Name)
		}
	}
	return nil
}

func (a *AllocRunner) Update(deployment *proto.Deployment1) {
	a.logger.Info("alloc updated")
	a.alloc.Deployment = deployment
	a.TaskStateUpdated()
}

func (a *AllocRunner) WaitCh() <-chan struct{} {
	return a.waitCh
}

// getClientStatus takes in the task states for a given allocation and computes
// the client status and description
func getClientStatus(taskStates map[string]*proto.TaskState) proto.Allocation1_Status {
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
		return proto.Allocation1_Failed
	} else if running {
		return proto.Allocation1_Running
	} else if pending {
		return proto.Allocation1_Pending
	} else if dead {
		return proto.Allocation1_Complete
	}

	panic("X")
}

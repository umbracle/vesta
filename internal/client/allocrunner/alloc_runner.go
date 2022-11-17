package allocrunner

import (
	"fmt"
	"reflect"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/umbracle/vesta/internal/client/allocrunner/driver"
	"github.com/umbracle/vesta/internal/client/allocrunner/taskrunner"
	"github.com/umbracle/vesta/internal/client/state"
	"github.com/umbracle/vesta/internal/server/proto"
)

type StateUpdater interface {
	AllocStateUpdated(alloc *proto.Allocation)
}

type Config struct {
	Logger       hclog.Logger
	Alloc        *proto.Allocation
	State        state.State
	StateUpdater StateUpdater
	Driver       driver.Driver
}

type AllocRunner struct {
	config       *Config
	logger       hclog.Logger
	tasks        map[string]*taskrunner.TaskRunner
	waitCh       chan struct{}
	alloc        *proto.Allocation
	driver       driver.Driver
	taskUpdated  chan struct{}
	stateUpdater StateUpdater
}

func NewAllocRunner(c *Config) (*AllocRunner, error) {
	logger := c.Logger.Named("alloc_runner").With("alloc", c.Alloc.Id)

	runner := &AllocRunner{
		config:       c,
		logger:       logger,
		tasks:        map[string]*taskrunner.TaskRunner{},
		waitCh:       make(chan struct{}),
		alloc:        c.Alloc,
		driver:       c.Driver,
		taskUpdated:  make(chan struct{}),
		stateUpdater: c.StateUpdater,
	}

	/*
		for _, task := range c.Alloc.Deployment.Tasks {
			config := &taskrunner.Config{
				Logger:           logger,
				Task:             task,
				Allocation:       c.Alloc,
				Driver:           c.Driver,
				State:            c.State,
				TaskStateUpdated: runner.TaskStateUpdated,
			}
			taskRunner, err := taskrunner.NewTaskRunner(config)
			if err != nil {
				return nil, err
			}
			runner.tasks[task.Id] = taskRunner
		}
	*/

	go runner.handleTaskStateUpdates()

	time.Sleep(500 * time.Millisecond)
	runner.TaskStateUpdated()

	return runner, nil
}

func (a *AllocRunner) handleTaskStateUpdates() {
	fmt.Println("_ IN _")
	defer fmt.Println("_ OUT !_ ")
	for {
		select {
		case <-a.taskUpdated:
		}

		fmt.Println("===>>==>==>=>=>")

		// check what we have to do
		local := a.tasks

		real := a.alloc.Deployment.Tasks

		visited := map[string]struct{}{}

		removeTasks := []*taskrunner.TaskRunner{}

		for name, one := range local {
			fmt.Println("Local task", name, one.TaskState().State)

			if one.TaskState().State == proto.TaskState_Dead {
				fmt.Println("- task is dead? -")
				// lets not consider this anymore
				// gc collect it
				continue
			}

			isShuttingDown := one.IsShuttingDown()

			visited[name] = struct{}{}

			two, ok := real[name]

			fmt.Println(two, ok)
			fmt.Println("IsShuttingDown", isShuttingDown)

			if !ok {
				fmt.Println("A1")
				// remove the task
				removeTasks = append(removeTasks, one)
			} else {
				fmt.Println("A2")
				if isUpdateTask(one.Task(), two) {
					if !isShuttingDown {
						fmt.Println("A3")
						// handle an update
						removeTasks = append(removeTasks, one)
					} else {
						fmt.Println("A4")
					}
				}
			}
		}

		newTasks := map[string]*proto.Task{}
		for name, two := range real {
			_, ok := visited[name]
			if ok {
				continue
			}
			// new task
			newTasks[name] = two
		}

		fmt.Println("- work to do -")
		fmt.Println(removeTasks)
		fmt.Println(newTasks)

		// remove tasks
		for name, removeTask := range removeTasks {
			a.logger.Info("remove task", "name", name)
			removeTask.KillNoWait()
		}

		// create a tasks
		for name, newTask := range newTasks {
			a.logger.Info("create task", "name", name)
			config := &taskrunner.Config{
				Logger:           a.logger,
				Task:             newTask,
				Allocation:       a.config.Alloc,
				Driver:           a.config.Driver,
				State:            a.config.State,
				TaskStateUpdated: a.TaskStateUpdated,
			}
			taskRunner, err := taskrunner.NewTaskRunner(config)
			if err != nil {
				panic(err)
			}
			go taskRunner.Run()
			a.tasks[name] = taskRunner

			fmt.Println("=> NEW TASK RUNNER CREATED")
		}

		states := map[string]*proto.TaskState{}
		for taskName, task := range a.tasks {
			states[taskName] = task.TaskState()
		}

		// Get the client allocation
		calloc := a.clientAlloc(states)

		fmt.Println("_-- calloc --")
		fmt.Println(calloc.Status)

		// Update the server
		a.stateUpdater.AllocStateUpdated(calloc)
	}
}

func isUpdateTask(a, b *proto.Task) bool {
	fmt.Println("-- cmp --")
	fmt.Println(a.Args, b.Args)
	fmt.Println(a.Env, b.Env)

	if !reflect.DeepEqual(a.Args, b.Args) {
		return true
	}
	if !reflect.DeepEqual(a.Env, b.Env) {
		return true
	}
	return false
}

func (a *AllocRunner) clientAlloc(states map[string]*proto.TaskState) *proto.Allocation {
	alloc := &proto.Allocation{
		Id:         a.alloc.Id,
		TaskStates: states,
	}

	alloc.Status = getClientStatus(states)
	return alloc
}

func (a *AllocRunner) TaskStateUpdated() {
	fmt.Println("_ DO IT ! _")
	select {
	case a.taskUpdated <- struct{}{}:
	default:
	}
}

func (a *AllocRunner) Restore() error {
	// read from db the tasks?
	for _, task := range a.alloc.Deployment.Tasks {
		runner := a.tasks[task.Id]

		if err := runner.Restore(); err != nil {
			return err
		}
	}
	return nil
}

func (a *AllocRunner) Update(alloc *proto.Allocation) {
	fmt.Println("#################### update allocation", alloc.Id, alloc.Deployment.Id)
	a.alloc = alloc
	a.TaskStateUpdated()
}

func (a *AllocRunner) WaitCh() <-chan struct{} {
	return a.waitCh
}

func (a *AllocRunner) Run() {
	/*
		defer close(a.waitCh)

		for _, task := range a.tasks {
			go task.Run()
		}
	*/
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
	} else if running {
		return proto.Allocation_Running
	} else if pending {
		return proto.Allocation_Pending
	} else if dead {
		return proto.Allocation_Complete
	}

	panic("X")
}

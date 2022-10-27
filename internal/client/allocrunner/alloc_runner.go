package allocrunner

import (
	"github.com/hashicorp/go-hclog"
	"github.com/umbracle/vesta/internal/client/allocrunner/driver"
	"github.com/umbracle/vesta/internal/client/allocrunner/taskrunner"
	"github.com/umbracle/vesta/internal/client/state"
	"github.com/umbracle/vesta/internal/server/proto"
)

type Config struct {
	Logger            hclog.Logger
	Alloc             *proto.Allocation
	State             state.State
	AllocStateUpdated func(alloc *proto.Allocation)
	Driver            driver.Driver
}

type AllocRunner struct {
	config      *Config
	logger      hclog.Logger
	tasks       map[string]*taskrunner.TaskRunner
	waitCh      chan struct{}
	alloc       *proto.Allocation
	driver      driver.Driver
	taskUpdated chan struct{}
}

func NewAllocRunner(c *Config) (*AllocRunner, error) {
	logger := c.Logger.Named("alloc_runner").With("alloc", c.Alloc.Id)

	runner := &AllocRunner{
		config:      c,
		logger:      logger,
		tasks:       map[string]*taskrunner.TaskRunner{},
		waitCh:      make(chan struct{}),
		alloc:       c.Alloc,
		driver:      c.Driver,
		taskUpdated: make(chan struct{}),
	}
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

	go runner.handleTaskStateUpdates()
	return runner, nil
}

func (a *AllocRunner) handleTaskStateUpdates() {
	for {
		<-a.taskUpdated

		states := map[string]*proto.TaskState{}
		for taskName, task := range a.tasks {
			states[taskName] = task.TaskState()
		}

		a.config.AllocStateUpdated(&proto.Allocation{
			Id:         a.alloc.Id,
			TaskStates: states,
		})
	}
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
		runner := a.tasks[task.Id]

		if err := runner.Restore(); err != nil {
			return err
		}
	}
	return nil
}

func (a *AllocRunner) Update(alloc *proto.Allocation) {
}

func (a *AllocRunner) WaitCh() <-chan struct{} {
	return a.waitCh
}

func (a *AllocRunner) Run() {
	defer close(a.waitCh)

	for _, task := range a.tasks {
		go task.Run()
	}
}

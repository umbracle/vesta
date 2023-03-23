package taskrunner

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/umbracle/vesta/internal/client/allocrunner/driver"
	"github.com/umbracle/vesta/internal/client/allocrunner/state"
	"github.com/umbracle/vesta/internal/server/proto"
	"github.com/umbracle/vesta/internal/uuid"
)

var defaultMaxEvents = 10

type TaskRunner struct {
	logger           hclog.Logger
	driver           driver.Driver
	waitCh           chan struct{}
	alloc            *proto.Allocation1
	task             *proto.Task1
	allocDir         string
	shutdownCh       chan struct{}
	killCh           chan struct{}
	state            state.State
	taskStateUpdated func()
	killErr          error
	killed           bool
	statusLock       sync.Mutex
	status           *proto.TaskState
	handle           *proto.TaskHandle
	restartCount     uint64
	metricsHook      *metricsHook
}

type Config struct {
	Logger           hclog.Logger
	Driver           driver.Driver
	Allocation       *proto.Allocation1
	AllocDir         string
	Task             *proto.Task1
	State            state.State
	TaskStateUpdated func()
	MetricsUpdater   MetricsUpdater
}

func NewTaskRunner(config *Config) *TaskRunner {
	logger := config.Logger.Named("task_runner").With("task-name", config.Task.Name)

	tr := &TaskRunner{
		logger:           logger,
		driver:           config.Driver,
		alloc:            config.Allocation,
		task:             config.Task,
		allocDir:         config.AllocDir,
		waitCh:           make(chan struct{}),
		shutdownCh:       make(chan struct{}),
		killCh:           make(chan struct{}),
		state:            config.State,
		status:           proto.NewTaskState(),
		taskStateUpdated: config.TaskStateUpdated,
	}

	/*
		if config.Task.Telemetry != nil {
			tr.metricsHook = newMetricsHook(logger, config.Task, config.MetricsUpdater)
		}
	*/

	return tr
}

func (t *TaskRunner) IsShuttingDown() bool {
	select {
	case <-t.killCh:
		return true
	default:
		return false
	}
}

func (t *TaskRunner) Task() *proto.Task1 {
	return t.task
}

func (t *TaskRunner) Run() {
	defer close(t.waitCh)
	var result *proto.ExitResult

MAIN:
	for {
		select {
		case <-t.killCh:
			break MAIN
		case <-t.shutdownCh:
			return
		default:
		}

		if err := t.runDriver(); err != nil {
			goto RESTART
		}

		// Run the prestart metrics action
		if t.metricsHook != nil {
			t.metricsHook.PostStart(t.handle)
		}

		{
			result = nil

			resultCh, err := t.driver.WaitTask(context.Background(), t.handle.Id)
			if err != nil {
				t.logger.Error("failed to wait for task", "err", err)
			} else {
				select {
				case <-t.killCh:
					result = t.handleKill(resultCh)
				case <-t.shutdownCh:
					return
				case result = <-resultCh:
				}

				t.emitExitResultEvent(result)
			}
		}

		t.clearDriverHandle()

	RESTART:
		restart, delay := t.shouldRestart()
		if !restart {
			break MAIN
		}

		select {
		case <-t.shutdownCh:
			return
		case <-time.After(delay):
		}
	}

	// task is dead
	t.UpdateStatus(proto.TaskState_Dead, nil)

	// Run the poststart metrics action
	if t.metricsHook != nil {
		t.metricsHook.Stop()
	}
}

func (t *TaskRunner) handleKill(resultCh <-chan *proto.ExitResult) *proto.ExitResult {
	t.killed = true

	// Check if it is still running
	select {
	case result := <-resultCh:
		return result
	default:
	}

	if err := t.driver.StopTask(t.handle.Id, 0); err != nil {
		t.killErr = err
	}

	select {
	case result := <-resultCh:
		return result
	case <-t.shutdownCh:
		return nil
	}
}

func (t *TaskRunner) emitExitResultEvent(result *proto.ExitResult) {
	if result == nil {
		return
	}
	event := proto.NewTaskEvent(proto.TaskTerminated).
		SetExitCode(result.ExitCode).
		SetSignal(result.Signal)

	t.EmitEvent(event)
}

func (t *TaskRunner) runDriver() error {
	if t.handle != nil {
		t.UpdateStatus(proto.TaskState_Running, nil)
		return nil
	}

	invocationid := uuid.Generate()[:8]

	tt := &driver.Task{
		Id:    fmt.Sprintf("%s/%s/%s", t.alloc.Deployment.Name, t.task.Name, invocationid),
		Task1: t.task,
	}

	handle, err := t.driver.StartTask(tt, t.allocDir)
	if err != nil {
		return err
	}
	t.handle = handle
	if err := t.state.PutTaskLocalState(t.alloc.Deployment.Name, t.task.Name, handle); err != nil {
		panic(err)
	}
	t.UpdateStatus(proto.TaskState_Running, proto.NewTaskEvent(proto.TaskStarted))
	return nil
}

func (t *TaskRunner) clearDriverHandle() {
	if t.handle != nil {
		t.driver.DestroyTask(t.handle.Id, true)
	}
	t.handle = nil
}

func (t *TaskRunner) TaskState() *proto.TaskState {
	t.statusLock.Lock()
	defer t.statusLock.Unlock()
	return t.status
}

func (t *TaskRunner) shouldRestart() (bool, time.Duration) {
	if t.killed {
		return false, 0
	}

	if t.task.Batch {
		// batch tasks are not restarted
		return false, 0
	}

	t.restartCount++
	if t.restartCount > 5 {
		// too many restarts, consider this task dead and do not realocate
		t.UpdateStatus(proto.TaskState_Dead, proto.NewTaskEvent(proto.TaskNotRestarting).SetFailsTask())
		return false, 0
	}

	t.UpdateStatus(proto.TaskState_Pending, proto.NewTaskEvent(proto.TaskRestarting))
	return true, time.Duration(2 * time.Second)
}

func (t *TaskRunner) Restore() error {
	state, handle, err := t.state.GetTaskState(t.alloc.Deployment.Name, t.task.Name)
	if err != nil {
		return err
	}
	t.status = state

	if err := t.driver.RecoverTask(handle.Id, handle); err != nil {
		t.UpdateStatus(proto.TaskState_Pending, nil)
		return nil
	}

	// the handle was restored
	t.handle = handle
	return nil
}

func (t *TaskRunner) UpdateStatus(status proto.TaskState_State, ev *proto.TaskState_Event) {
	t.statusLock.Lock()
	defer t.statusLock.Unlock()

	t.logger.Info("Update status", "status", status.String())
	t.status.State = status

	if ev != nil {
		if ev.FailsTask() {
			t.status.Failed = true
		}
		t.appendEventLocked(ev)
	}

	if err := t.state.PutTaskState(t.alloc.Deployment.Name, t.task.Name, t.status); err != nil {
		t.logger.Warn("failed to persist task state during update status", "err", err)
	}
	t.taskStateUpdated()
}

func (t *TaskRunner) EmitEvent(ev *proto.TaskState_Event) {
	t.statusLock.Lock()
	defer t.statusLock.Unlock()

	t.appendEventLocked(ev)

	if err := t.state.PutTaskState(t.alloc.Deployment.Name, t.task.Name, t.status); err != nil {
		t.logger.Warn("failed to persist task state during emit event", "err", err)
	}

	t.taskStateUpdated()
}

func (t *TaskRunner) appendEventLocked(ev *proto.TaskState_Event) {
	if t.status.Events == nil {
		t.status.Events = []*proto.TaskState_Event{}
	}
	t.status.Events = append(t.status.Events, ev)
}

func (t *TaskRunner) KillNoWait() {
	close(t.killCh)
}

func (t *TaskRunner) Kill(ctx context.Context, ev *proto.TaskState_Event) error {
	close(t.killCh)

	select {
	case <-t.WaitCh():
	case <-ctx.Done():
		return ctx.Err()
	}

	return t.killErr
}

func (t *TaskRunner) WaitCh() <-chan struct{} {
	return t.waitCh
}

func (t *TaskRunner) Close() {
	close(t.shutdownCh)
	<-t.WaitCh()
}

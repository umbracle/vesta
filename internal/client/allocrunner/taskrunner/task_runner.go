package taskrunner

import (
	"context"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/umbracle/vesta/internal/client/allocrunner/driver"
	"github.com/umbracle/vesta/internal/client/state"
	"github.com/umbracle/vesta/internal/server/proto"
)

var defaultMaxEvents = 10

type TaskRunner struct {
	logger           hclog.Logger
	driver           driver.Driver
	waitCh           chan struct{}
	alloc            *proto.Allocation
	task             *proto.Task
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
}

type Config struct {
	Logger           hclog.Logger
	Driver           driver.Driver
	Allocation       *proto.Allocation
	AllocDir         string
	Task             *proto.Task
	State            state.State
	TaskStateUpdated func()
}

func NewTaskRunner(config *Config) *TaskRunner {
	logger := config.Logger.Named("task_runner").With("task-id", config.Task.Id).With("task-name", config.Task.Name)

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

func (t *TaskRunner) Task() *proto.Task {
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

		{
			result = nil

			resultCh, err := t.driver.WaitTask(context.Background(), t.task.Id)
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
}

func (tr *TaskRunner) handleKill(resultCh <-chan *proto.ExitResult) *proto.ExitResult {
	tr.killed = true

	// Check if it is still running
	select {
	case result := <-resultCh:
		return result
	default:
	}

	if err := tr.driver.StopTask(tr.task.Id, 0); err != nil {
		tr.killErr = err
	}

	select {
	case result := <-resultCh:
		return result
	case <-tr.shutdownCh:
		return nil
	}
}

func (tr *TaskRunner) emitExitResultEvent(result *proto.ExitResult) {
	if result == nil {
		return
	}
	event := proto.NewTaskEvent(proto.TaskTerminated).
		SetExitCode(result.ExitCode).
		SetSignal(result.Signal)

	tr.EmitEvent(event)
}

func (t *TaskRunner) runDriver() error {
	if t.handle != nil {
		t.UpdateStatus(proto.TaskState_Running, nil)
		return nil
	}

	handle, err := t.driver.StartTask(t.task, t.allocDir)
	if err != nil {
		return err
	}
	t.handle = handle
	if err := t.state.PutTaskLocalState(t.alloc.Id, t.task.Id, handle); err != nil {
		panic(err)
	}
	t.UpdateStatus(proto.TaskState_Running, proto.NewTaskEvent(proto.TaskStarted))
	return nil
}

func (tr *TaskRunner) clearDriverHandle() {
	if tr.handle != nil {
		tr.driver.DestroyTask(tr.task.Id, true)
	}
	tr.handle = nil
}

func (t *TaskRunner) TaskState() *proto.TaskState {
	t.statusLock.Lock()
	defer t.statusLock.Unlock()
	return t.status
}

var defDelay = time.Duration(2 * time.Second)

func (t *TaskRunner) shouldRestart() (bool, time.Duration) {
	if t.killed {
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
	state, handle, err := t.state.GetTaskState(t.alloc.Id, t.task.Id)
	if err != nil {
		return err
	}
	t.status = state

	if err := t.driver.RecoverTask(t.task.Id, handle); err != nil {
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

	if err := t.state.PutTaskState(t.alloc.Id, t.task.Id, t.status); err != nil {
		t.logger.Warn("failed to persist task state during update status", "err", err)
	}
	t.taskStateUpdated()
}

func (t *TaskRunner) EmitEvent(ev *proto.TaskState_Event) {
	t.statusLock.Lock()
	defer t.statusLock.Unlock()

	t.appendEventLocked(ev)

	if err := t.state.PutTaskState(t.alloc.Id, t.task.Id, t.status); err != nil {
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

func (tr *TaskRunner) WaitCh() <-chan struct{} {
	return tr.waitCh
}

func (t *TaskRunner) Close() {
	close(t.shutdownCh)
	<-t.WaitCh()
}

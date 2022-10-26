package runner

import (
	"context"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/umbracle/vesta/internal/client/state"
	"github.com/umbracle/vesta/internal/docker"
	"github.com/umbracle/vesta/internal/server/proto"
)

type TaskRunner struct {
	logger     hclog.Logger
	driver     *docker.Docker
	waitCh     chan struct{}
	alloc      *proto.Allocation
	task       *proto.Task
	shutdownCh chan struct{}
	killCh     chan struct{}
	state      *state.State

	TaskStateUpdated func()

	status *proto.TaskState
	handle *proto.TaskHandle
}

func NewTaskRunner(logger hclog.Logger, task *proto.Task, alloc *proto.Allocation, driver *docker.Docker, state *state.State) (*TaskRunner, error) {
	tr := &TaskRunner{
		logger:     logger.Named("task_runner").With("task", task.Id),
		driver:     driver,
		alloc:      alloc,
		task:       task,
		waitCh:     make(chan struct{}),
		shutdownCh: make(chan struct{}),
		killCh:     make(chan struct{}),
		state:      state,
		status:     proto.NewTaskState(),
	}
	return tr, nil
}

func (t *TaskRunner) Run() {
	defer close(t.waitCh)
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
			resultCh, err := t.driver.WaitTask(context.Background(), t.task.Id)
			if err != nil {
				t.logger.Error("failed to wait for task", "err", err)
			} else {
				select {
				case <-t.killCh:
					// TODO
				case <-t.shutdownCh:
					return
				case <-resultCh:
					// TODO: Use result for backoff restart timeout
				}
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

func (t *TaskRunner) runDriver() error {
	if t.handle != nil {
		t.UpdateStatus(proto.TaskState_Running, nil)
		return nil
	}

	handle, err := t.driver.StartTask(t.task)
	if err != nil {
		return err
	}
	t.handle = handle
	if err := t.state.PutTaskLocalState(t.alloc.Id, t.task.Id, handle); err != nil {
		panic(err)
	}
	t.UpdateStatus(proto.TaskState_Running, nil)
	return nil
}

func (tr *TaskRunner) clearDriverHandle() {
	if tr.handle != nil {
		tr.driver.DestroyTask(tr.task.Id, true)
	}
	tr.handle = nil
}

func (t *TaskRunner) TaskState() *proto.TaskState {
	return t.status
}

func (t *TaskRunner) shouldRestart() (bool, time.Duration) {
	return true, time.Duration(2 * time.Second)
}

func (t *TaskRunner) Restore() error {
	state, handle, err := t.state.GetTaskState(t.alloc.Id, t.task.Id)
	if err != nil {
		return err
	}
	t.status = state

	if err := t.driver.RecoverTask(t.task.Id, handle); err != nil {
		// TODO: If the task should not be running, it is okay
		// The task was not found, we have to move to pending
		t.UpdateStatus(proto.TaskState_Pending, nil)
		return nil
	}

	// the handle was restored
	t.handle = handle
	return nil
}

func (t *TaskRunner) UpdateStatus(status proto.TaskState_State, ev *proto.TaskState_Event) {
	t.logger.Info("Update status", "status", status.String())
	t.status.State = status

	if err := t.state.PutTaskState(t.alloc.Id, t.task.Id, t.status); err != nil {
		panic(err)
	}
	t.TaskStateUpdated()
}

func (t *TaskRunner) Kill() {
	close(t.killCh)
}

func (tr *TaskRunner) WaitCh() <-chan struct{} {
	return tr.waitCh
}

func (t *TaskRunner) Close() {
	close(t.shutdownCh)
	<-t.WaitCh()
}

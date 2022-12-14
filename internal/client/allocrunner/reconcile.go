package allocrunner

import (
	"fmt"
	"reflect"

	"github.com/umbracle/vesta/internal/server/proto"
)

type allocResults struct {
	// removeTasks is the list of tasks to remove
	removeTasks []string

	// newTasks is the list of tasks to create
	newTasks map[string]*proto.Task
}

func (a *allocResults) GoString() string {
	return fmt.Sprintf("alloc: remove (%d), create (%d)", len(a.removeTasks), len(a.newTasks))
}

type allocReconciler struct {
	// alloc is the allocation being processed
	alloc *proto.Allocation

	// tasks is the list of running tasks
	tasks map[string]*proto.Task

	// pendingDelete signals whether the task is being deleted
	pendingDelete map[string]struct{}

	// state is the state of the running tasks
	tasksState map[string]*proto.TaskState
}

func newAllocReconciler(alloc *proto.Allocation, tasks map[string]*proto.Task,
	tasksState map[string]*proto.TaskState, pendingDelete map[string]struct{}) *allocReconciler {
	return &allocReconciler{
		alloc:         alloc,
		tasks:         tasks,
		tasksState:    tasksState,
		pendingDelete: pendingDelete,
	}
}

func (a *allocReconciler) Compute() *allocResults {
	result := &allocResults{
		removeTasks: []string{},
		newTasks:    map[string]*proto.Task{},
	}

	// check if the whole deployment has to be destroyed
	if a.alloc.DesiredStatus == proto.Allocation_Stop {
		for name, task := range a.tasks {
			state := a.tasksState[name]

			_, isPendingDelete := a.pendingDelete[name]
			if state.State != proto.TaskState_Dead && !isPendingDelete {
				result.removeTasks = append(result.removeTasks, task.Name)
			}
		}
		return result
	}

	depTasks := map[string]*proto.Task{}
	for name, task := range a.alloc.Deployment.Tasks {
		depTasks[name] = task
	}

	for name, task := range a.tasks {
		state := a.tasksState[name]

		if state.State == proto.TaskState_Dead {
			// TODO: Garbage collect
			continue
		}

		depTask, ok := depTasks[name]
		if !ok {
			// task not expected, remove it
			result.removeTasks = append(result.removeTasks, name)
		} else {
			if tasksUpdated(task, depTask) {
				if _, ok := a.pendingDelete[name]; !ok {
					// task is not up to date, remove it. It will be
					// allocated on the next iteration once this one
					// is dead.
					result.removeTasks = append(result.removeTasks, name)
				}
			}
		}
	}

	// add tasks
	for name, task := range depTasks {
		_, ok := a.tasks[name]
		if ok {
			// if the task already exists, we only re-create it if
			// the task is fully dead and it did not fail
			taskState := a.tasksState[name]
			if taskState.State != proto.TaskState_Dead {
				continue
			}
			if taskState.Failed {
				continue
			}
		}

		result.newTasks[name] = task
	}

	return result
}

func tasksUpdated(a, b *proto.Task) bool {
	if !reflect.DeepEqual(a.Args, b.Args) {
		return true
	}
	if !reflect.DeepEqual(a.Env, b.Env) {
		return true
	}
	return false
}

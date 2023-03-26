package allocrunner

import (
	"fmt"
	"reflect"

	proto "github.com/umbracle/vesta/internal/client/runner/structs"
)

type allocResults struct {
	// removeTasks is the list of tasks to remove
	removeTasks []string

	// newTasks is the list of tasks to create
	newTasks map[string]*proto.Task
}

func (a *allocResults) Empty() bool {
	return len(a.removeTasks) == 0 && len(a.newTasks) == 0
}

func (a *allocResults) GoString() string {
	return fmt.Sprintf("alloc: remove (%d), create (%d)", len(a.removeTasks), len(a.newTasks))
}

type allocReconciler struct {
	// alloc is the allocation being processed
	alloc *proto.Allocation

	// tasks is the list of running tasks
	tasks map[string]*proto.Task

	// state is the state of the running tasks
	tasksState map[string]*proto.TaskState
}

func newAllocReconciler(alloc *proto.Allocation, tasks map[string]*proto.Task,
	tasksState map[string]*proto.TaskState) *allocReconciler {
	return &allocReconciler{
		alloc:      alloc,
		tasks:      tasks,
		tasksState: tasksState,
	}
}

func (a *allocReconciler) Compute() *allocResults {
	result := &allocResults{
		removeTasks: []string{},
		newTasks:    map[string]*proto.Task{},
	}

	depTasks := map[string]*proto.Task{}
	for _, task := range a.alloc.Deployment.Tasks {
		depTasks[task.Name] = task
	}

	// remove tasks that:
	// 1. are not part of the deployment anymore.
	// 2. have been updated.
	for name, task := range a.tasks {
		state := a.tasksState[name]

		if state.State == proto.TaskState_Dead {
			// dead tasks cannot be removed anymore. The might get re-allocated
			// if it is an update on the next step of the reconciler.
			// The 'Run' lifecycle will garbage collect these tasks later.
			continue
		}

		if state.Killing {
			// If the task is being deleted right now, this task was part of a
			// previous remove operation so it does not need to be processed again.
			continue
		}

		depTask, ok := depTasks[name]
		if !ok {
			fmt.Println("A")
			// task is not found on the deployment
			result.removeTasks = append(result.removeTasks, name)
		} else {
			if tasksUpdated(task, depTask) {
				fmt.Println("B")
				// task is not up to date, remove it. It will be
				// allocated on the next iteration once this one
				// is dead.
				result.removeTasks = append(result.removeTasks, name)
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
	if !reflect.DeepEqual(a.Image, b.Image) {
		fmt.Println("1", a.Image, b.Image)
		return true
	}
	if !reflect.DeepEqual(a.Tag, b.Tag) {
		fmt.Println("2", a.Tag, b.Tag)
		return true
	}
	if !reflect.DeepEqual(a.Args, b.Args) {
		fmt.Println("3", a.Args, b.Args)
		return true
	}
	return false
}

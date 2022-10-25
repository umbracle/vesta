package proto

func NewTaskState() *TaskState {
	return &TaskState{
		State: TaskState_Pending,
	}
}

package proto

func NewTaskState() *TaskState {
	return &TaskState{
		State: TaskState_Pending,
	}
}

func (r *ExitResult) Successful() bool {
	return r.ExitCode == 0 && r.Signal == 0 && r.Err == ""
}

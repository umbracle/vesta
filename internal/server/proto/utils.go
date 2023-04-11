package proto

import (
	"google.golang.org/protobuf/proto"
)

const (
	TaskStarted       = "Started"
	TaskTerminated    = "Terminated"
	TaskRestarting    = "Restarting"
	TaskNotRestarting = "Not-restarting"
)

func (a *Allocation) Copy() *Allocation {
	return proto.Clone(a).(*Allocation)
}

func NewTask() *Task {
	return &Task{}
}

func (t *Task) WithImage(image string) *Task {
	t.Image = image
	return t
}

func (t *Task) WithTag(tag string) *Task {
	t.Tag = tag
	return t
}

func (t *Task) WithCmd(cmd ...string) *Task {
	if len(t.Args) == 0 {
		t.Args = []string{}
	}
	t.Args = append(t.Args, cmd...)
	return t
}

func (t *Task) WithFile(path string, obj string) *Task {
	if len(t.Data) == 0 {
		t.Data = map[string]string{}
	}
	t.Data[path] = obj
	return t
}

func (t *Task) WithVolume(name, mountPoint string) *Task {
	if len(t.Volumes) == 0 {
		t.Volumes = map[string]*Task_Volume{}
	}
	t.Volumes[name] = &Task_Volume{
		Path: mountPoint,
	}
	return t
}

func (t *Task) WithTelemetry(port uint64, path string) *Task {
	t.Telemetry = &Task_Telemetry{
		Port: port,
		Path: path,
	}
	return t
}

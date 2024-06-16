package proto

import "github.com/umbracle/vesta/internal/schema"

type Item struct {
	Name   string
	Fields map[string]*schema.Field
	Chains []string
}

type ApplyRequest struct {
	// id name of the action
	Action string

	// input for the action
	Input map[string]interface{}

	// name of the allocation to modify
	AllocationId string

	Metrics  bool
	Chain    string
	Alias    string
	LogLevel string
}

type Item_Field struct {
	Name        string
	Type        string
	Description string
	Default     string
	Required    bool // remove
	ForceNew    bool
}

type VolumeStub struct {
}

type Service struct {
	Name           string
	Task           *Task
	Init           []*InitAction
	Artifacts      []*Artifact
	InitContainers []*Task
	PrevState      []byte
	Volumes        []*ServiceVolume
}

type InitAction struct {
	Cmd []string
}

type Artifact struct {
	Src string
	Dst string
}

// ServiceVolume is the reference to a volume assigned to a service
type ServiceVolume struct {
	Name string
	ID   string
}

type Volume struct {
	Id     string
	Name   string
	Labels map[string]string
}

func (v *Volume) Copy() *Volume {
	vv := new(Volume)
	*vv = *v

	vv.Labels = make(map[string]string)
	for k, v := range v.Labels {
		vv.Labels[k] = v
	}

	return vv
}

// Task represents an single container process
type Task struct {
	Image       string
	Tag         string
	Args        []string
	Env         map[string]string
	Labels      map[string]string
	SecurityOpt []string
	// list of data access for this file
	Data    map[string]string
	Volumes map[string]*Task_Volume
	Batch   bool
	Ports   []*Task_Port
}

type Event struct {
	Id        string
	Timestamp uint64
	Service   string
	Task      string
	Type      string
	Action    string
}

type Task_Volume struct {
	Name       string
	Path       string
	Id         string
	Labels     map[string]string
	Properties map[string]*Item_Field
}

type Task_Port struct {
	Name string
	Port uint64
}

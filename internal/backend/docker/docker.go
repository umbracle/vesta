package docker

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/umbracle/vesta/internal/server/proto"
	"github.com/umbracle/vesta/internal/uuid"
)

//go:embed templates/service.yml.tmpl
var templateFile string

type Docker struct {
	dir          string
	EventUpdater EventUpdater
}

type EventUpdater interface {
	UpdateEvent(event *proto.Event)
}

func NewDocker(dir string, u EventUpdater) *Docker {
	dir = filepath.Join(dir, "docker")
	if err := os.MkdirAll(dir, 0755); err != nil {
		panic(err)
	}

	d := &Docker{
		dir:          dir,
		EventUpdater: u,
	}

	go d.Start()
	return d
}

func (d *Docker) Start() {
	client, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		panic(err)
	}

	msgCh, errCh := client.Events(context.Background(), types.EventsOptions{})
	for {
		select {
		case msg := <-msgCh:
			service, ok := msg.Actor.Attributes["service"]
			if !ok {
				continue
			}

			event := &proto.Event{
				Id:        uuid.Generate(),
				Service:   service,
				Task:      msg.Actor.Attributes["task"],
				Timestamp: uint64(msg.Time),
				Action:    msg.Action,
				Type:      msg.Type,
			}
			d.EventUpdater.UpdateEvent(event)

		case <-errCh:

		}
	}
}

func (d *Docker) Destroy(name string) {
	execCmd("docker", []string{"stack", "rm", name})
}

func (d *Docker) Connect(service string, task string, port uint64) (string, error) {
	return fmt.Sprintf("http://%s.%s:%d", service, task, port), nil
}

type TaskDesc struct {
	Task    *proto.Task
	Volumes []*proto.Volume
}

func (d *Docker) RunTasks(serviceSpec *proto.Service) {
	fmt.Println("XX")
	service := &service{}
	service.Services = make([]task1, 0, 1)

	// create the volume if it does not exists
	volumesMap := map[string]string{}

	fmt.Println("-- vv", serviceSpec.Volumes)
	fmt.Println("-- task", serviceSpec.Task.Volumes)

	for _, volume := range serviceSpec.Volumes {
		path := filepath.Join(d.dir, "volumes", volume.ID)

		if err := os.MkdirAll(path, 0755); err != nil {
			panic(err)
		}
		volumesMap[volume.Name] = path
	}

	task := serviceSpec.Task

	fileCount := 0
	var files []File
	var volumes1 []volume1

	// decide what to do with the volumes
	for name, volume := range task.Volumes {
		volumes1 = append(volumes1, volume1{
			Source: volumesMap[name],
			Target: volume.Path,
		})
	}

	for target, content := range task.Data {
		// create temporal file
		f, err := os.CreateTemp("/tmp", "file-temp")
		if err != nil {
			panic(err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			panic(err)
		}
		f.Close()

		files = append(files, File{
			Name:   fmt.Sprintf("file-%d", fileCount),
			Source: f.Name(),
			Target: target,
		})
	}

	initContainers := []string{}
	for indx, task := range serviceSpec.InitContainers {
		name := fmt.Sprintf("init-%d", indx)
		service.Services = append(service.Services, task1{
			Task:         task,
			Name:         name,
			VolumesExtra: volumes1,
		})
		initContainers = append(initContainers, name)
	}

	service.Services = append(service.Services, task1{
		Task:          task,
		Name:          "node",
		Files:         files,
		VolumesExtra:  volumes1,
		InitContainer: initContainers,
	})

	fmt.Println("- service created -")
	fmt.Println(initContainers)

	content := executeTemplate(service)
	fileName := filepath.Join(d.dir, fmt.Sprintf("srv-%s.yaml", serviceSpec.Name))
	if err := os.WriteFile(fileName, []byte(content), 0644); err != nil {
		panic(err)
	}

	fmt.Println("- file written -", fileName)

	// execCmd("docker", []string{"compose", "-f", fileName, "-d", "up"})
}

func execCmd(cmdName string, args []string) error {
	cmd := exec.Command(cmdName, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(string(output))
		panic(err)
	}
	fmt.Println(string(output))
	return nil
}

type task1 struct {
	*proto.Task
	Name          string
	Files         []File
	ID            string
	VolumesExtra  []volume1
	InitContainer []string
}

type volume1 struct {
	Source string
	Target string
}

type File struct {
	Name   string
	Source string
	Target string
}

type service struct {
	Services []task1
}

func executeTemplate(input interface{}) string {
	tmpl, err := template.New("service").Parse(templateFile)
	if err != nil {
		panic(err)
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, input)
	if err != nil {
		panic(err)
	}

	return buf.String()
}

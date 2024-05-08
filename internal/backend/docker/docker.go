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

		case err := <-errCh:
			fmt.Println(err)
		}
	}
}

func (d *Docker) Destroy(name string) {
	execCmd("docker", []string{"stack", "rm", name})
}

func (d *Docker) RunTasks(serviceSpec *proto.Service) {
	service := &service{}
	service.Services = make([]task1, 0, len(serviceSpec.Tasks))

	for name, task := range serviceSpec.Tasks {
		fileCount := 0
		var files []File
		var volumes1 []volume1

		// decide what to do with the volumes
		for _, volume := range task.Volumes {
			if volume.Id == "" {
				path := filepath.Join(d.dir, "volumes", uuid.Generate())

				fmt.Println(path)

				if err := os.MkdirAll(path, 0755); err != nil {
					panic(err)
				}
				volumes1 = append(volumes1, volume1{
					Source: path,
					Target: volume.Path,
				})
			} else {
				volumes1 = append(volumes1, volume1{
					Source: volume.Id,
					Target: volume.Path,
				})
			}
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

		service.Services = append(service.Services, task1{
			Task:         task,
			Name:         name,
			Files:        files,
			VolumesExtra: volumes1,
		})
	}

	content := executeTemplate(service)
	fileName := filepath.Join(d.dir, fmt.Sprintf("srv-%s.yaml", serviceSpec.Name))
	if err := os.WriteFile(fileName, []byte(content), 0644); err != nil {
		panic(err)
	}

	execCmd("docker", []string{"stack", "deploy", "-c", fileName, serviceSpec.Name})
}

func execCmd(cmdName string, args []string) error {
	cmd := exec.Command(cmdName, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		panic(err)
	}
	fmt.Println(string(output))
	return nil
}

type task1 struct {
	*proto.Task
	Name         string
	Files        []File
	ID           string
	VolumesExtra []volume1
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

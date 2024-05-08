package docker

import (
	"fmt"
	"os"
	"testing"

	"github.com/umbracle/vesta/internal/server/proto"
)

func TestTemplate(t *testing.T) {

	tasks := map[string]*proto.Task{
		"node": {
			Image: "nginx",
			Args:  []string{"-p", "80:80"},
			Data: map[string]string{
				"/etc/nginx/nginx.conf": "servxx",
			},
		},
	}

	service := &service{}
	service.Services = make([]task1, 0, len(tasks))

	for name, task := range tasks {
		// for each file, create a map

		fileCount := 0
		var files []File
		for target, content := range task.Data {
			// create this file in tmp folder

			// create temporal file
			f, err := os.CreateTemp("/tmp", "file-temp")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := f.Write([]byte(content)); err != nil {
				t.Fatal(err)
			}
			f.Close()

			files = append(files, File{
				Name:   fmt.Sprintf("file-%d", fileCount),
				Source: f.Name(),
				Target: target,
			})
		}

		service.Services = append(service.Services, task1{
			Task:  task,
			Name:  name,
			Files: files,
		})
	}

	executeTemplate(service)
}

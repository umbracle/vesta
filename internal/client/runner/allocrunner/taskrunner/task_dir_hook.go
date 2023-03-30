package taskrunner

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/umbracle/vesta/internal/client/runner/driver"
	"github.com/umbracle/vesta/internal/client/runner/hooks"
	proto "github.com/umbracle/vesta/internal/client/runner/structs"
)

type mountSetter interface {
	setMount(*driver.MountConfig)
}

type taskDirHook struct {
	logger      hclog.Logger
	task        *proto.Task
	alloc       *proto.Allocation
	mountSetter mountSetter
	done        bool
}

func newTaskDirHook(logger hclog.Logger, alloc *proto.Allocation, task *proto.Task, mountSetter mountSetter) *taskDirHook {
	h := &taskDirHook{
		task:        task,
		mountSetter: mountSetter,
		alloc:       alloc,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (t *taskDirHook) Name() string {
	return "task-dir"
}

func (t *taskDirHook) Prestart(ctx chan struct{}, req *hooks.TaskPrestartHookRequest) error {
	if t.done {
		return nil
	}

	mountPoints := composeMountData(t.task.Data)
	for _, mount := range mountPoints {
		// create host directory for the mount
		hostPath, err := ioutil.TempDir("/tmp", "vesta-")
		if err != nil {
			return err
		}

		// create files
		for name, content := range mount.files {
			// the path of the file includes the parent directory
			// trim it to get the relative name in 'hostPath'
			localName := strings.TrimPrefix(name, mount.path)

			if err := ioutil.WriteFile(filepath.Join(hostPath, localName), []byte(content), 0644); err != nil {
				return err
			}
		}

		t.mountSetter.setMount(&driver.MountConfig{
			HostPath: hostPath,
			TaskPath: mount.path,
		})
	}

	for name, volume := range t.task.Volumes {
		volName := fmt.Sprintf("%s-%s-%s", t.alloc.Deployment.Name, t.task.Name, name)

		t.mountSetter.setMount(&driver.MountConfig{
			HostPath: volName,
			TaskPath: volume.Path,
		})
	}

	t.done = true
	return nil
}

type mountPoint struct {
	path  string
	files map[string]string
}

func composeMountData(files map[string]string) []*mountPoint {
	groups := []*mountPoint{}
	for name, content := range files {

		found := false
		for _, grp := range groups {
			prefix, ok := getPrefix(grp.path, name)
			if ok {
				found = true
				// replace the group
				grp.path = prefix
				grp.files[name] = content
				break
			}
		}
		if !found {
			// get absolute path
			groups = append(groups, &mountPoint{
				path: getAbs(name),
				files: map[string]string{
					name: content,
				},
			})
		}
	}
	return groups
}

func getAbs(path string) string {
	spl := strings.Split(path, "/")
	name := spl[:len(spl)-1]
	return strings.Join(name, "/")
}

func getPrefix(a, b string) (string, bool) {
	aSpl := strings.Split(a, "/")
	bSpl := strings.Split(b, "/")

	size := len(aSpl)
	if size > len(bSpl) {
		size = len(bSpl)
	}

	prefix := []string{}
	for i := 0; i < size; i++ {
		if aSpl[i] == bSpl[i] {
			prefix = append(prefix, aSpl[i])
		}
	}
	return strings.Join(prefix, "/"), len(prefix) != 1
}

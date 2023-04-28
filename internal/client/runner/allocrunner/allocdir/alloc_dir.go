package allocdir

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type AllocDir struct {
	AllocDir string

	TaskDirs map[string]*TaskDir
}

func NewAllocDir(clientAllocDir, allocID string) *AllocDir {
	allocDir := filepath.Join(clientAllocDir, allocID)

	return &AllocDir{
		AllocDir: allocDir,
		TaskDirs: map[string]*TaskDir{},
	}
}

func (a *AllocDir) NewTaskDir(name string) *TaskDir {
	tt := newTaskDir(a.AllocDir, name)
	a.TaskDirs[name] = tt
	return tt
}

func (a *AllocDir) Build() error {
	// build alloc directories
	if err := os.MkdirAll(a.AllocDir, 0755); err != nil {
		return fmt.Errorf("failed to make the alloc directory %v: %v", a.AllocDir, err)
	}

	return nil
}

type volumeMount struct {
	name string
	path string
}

type TaskDir struct {
	Dir        string
	VolumesDir string
	volumes    []*volumeMount
}

func newTaskDir(allocDir, taskName string) *TaskDir {
	taskDir := filepath.Join(allocDir, taskName)

	return &TaskDir{
		Dir:        taskDir,
		VolumesDir: filepath.Join(taskDir, "volumes"),
	}
}

func (a *TaskDir) Build() error {
	// build alloc directories
	if err := os.MkdirAll(a.Dir, 0755); err != nil {
		return fmt.Errorf("failed to make the task directory %v: %v", a.Dir, err)
	}

	// Make the task directory have non-root permissions.
	if err := dropDirPermissions(a.Dir, os.ModePerm); err != nil {
		return err
	}

	// Create a directory to store the volumes of the task
	if err := os.MkdirAll(a.VolumesDir, 0777); err != nil {
		return err
	}

	if err := dropDirPermissions(a.VolumesDir, os.ModePerm); err != nil {
		return err
	}

	// create a new directory for each volume
	for _, vol := range a.volumes {
		dir := filepath.Join(a.VolumesDir, vol.name)

		if err := os.MkdirAll(dir, 0777); err != nil {
			return err
		}
		if err := dropDirPermissions(dir, os.ModePerm); err != nil {
			return err
		}
	}

	return nil
}

func (a *TaskDir) GetVolume(name string) string {
	return filepath.Join(a.VolumesDir, name)
}

func (a *TaskDir) CreateVolume(name string, path string) string {
	a.volumes = append(a.volumes, &volumeMount{name: name, path: path})

	return a.GetVolume(name)
}

func (a *TaskDir) ResolvePath(path string) (string, bool) {
	for _, vol := range a.volumes {
		if strings.HasPrefix(path, vol.path) {
			relPath := strings.TrimPrefix(path, vol.path)

			dir := filepath.Join(a.VolumesDir, vol.name)
			return filepath.Join(dir, relPath), true
		}
	}
	return "", false
}

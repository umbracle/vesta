package docker

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/umbracle/vesta/internal/client/runner/driver"
	proto "github.com/umbracle/vesta/internal/client/runner/structs"
	"github.com/umbracle/vesta/internal/uuid"
)

func TestDriver_CreateContainerOptions_Labels(t *testing.T) {
	d, _ := NewDockerDriver(nil)

	tt := &driver.Task{
		Task: &proto.Task{
			Labels: map[string]string{
				"some": "label",
			},
		},
	}
	opts, err := d.createContainerOptions(tt)
	assert.NoError(t, err)

	assert.Equal(t, opts.config.Labels["some"], "label")
	assert.Equal(t, opts.config.Labels["vesta"], "true")
}

func TestDriver_CreateContainerOptions_Env(t *testing.T) {
	d, _ := NewDockerDriver(nil)

	tt := &driver.Task{
		Task: &proto.Task{
			Env: map[string]string{
				"some": "label",
			},
		},
	}
	opts, err := d.createContainerOptions(tt)
	assert.NoError(t, err)

	assert.Equal(t, opts.config.Env, []string{"some=label"})
}

func TestDriver_CreateContainerOptions_Image(t *testing.T) {
	d, _ := NewDockerDriver(nil)

	tt := &driver.Task{
		Task: &proto.Task{
			Image: "a",
			Tag:   "b",
		},
	}
	opts, err := d.createContainerOptions(tt)
	assert.NoError(t, err)

	assert.Equal(t, opts.config.Image, "a:b")
}

func TestDriver_CreateContainerOptions_DataMount(t *testing.T) {
	d, _ := NewDockerDriver(nil)

	tt := &driver.Task{
		Task: &proto.Task{
			Data: map[string]string{
				"/var/file3.txt": "c",
			},
		},
	}
	opts, err := d.createContainerOptions(tt)
	assert.NoError(t, err)

	assert.Equal(t, strings.Split(opts.host.Binds[0], ":")[1], "/var")
}

func TestDriver_Start_Wait(t *testing.T) {
	d, _ := NewDockerDriver(nil)

	tt := &driver.Task{
		Task: &proto.Task{
			Image: "busybox",
			Tag:   "1.29.3",
			Args:  []string{"nc", "-l", "-p", "3000", "127.0.0.1"},
		},
	}
	_, err := d.StartTask(tt)
	assert.NoError(t, err)

	defer d.DestroyTask(tt.Id, true)

	waitCh, _ := d.WaitTask(context.Background(), tt.Id)
	select {
	case res := <-waitCh:
		t.Fatalf("it should not finish yet: %v", res)
	case <-time.After(time.Second):
	}
}

func TestDriver_Start_WaitFinished(t *testing.T) {
	d, _ := NewDockerDriver(nil)

	tt := &driver.Task{
		Id: uuid.Generate(),
		Task: &proto.Task{
			Image: "busybox",
			Tag:   "1.29.3",
			Args:  []string{"echo", "hello"},
		},
	}
	_, err := d.StartTask(tt)
	assert.NoError(t, err)

	defer d.DestroyTask(tt.Id, true)

	waitCh, _ := d.WaitTask(context.Background(), tt.Id)
	select {
	case res := <-waitCh:
		assert.True(t, res.Successful())
	case <-time.After(time.Second):
		t.Fatalf("timeout")
	}
}

func TestDriver_Start_Kill_Wait(t *testing.T) {
	d, _ := NewDockerDriver(nil)

	tt := &driver.Task{
		Id: uuid.Generate(),
		Task: &proto.Task{
			Image: "busybox",
			Tag:   "1.29.3",
			Args:  []string{"echo", "hello"},
		},
	}
	_, err := d.StartTask(tt)
	assert.NoError(t, err)

	defer d.DestroyTask(tt.Id, true)

	waitCh, _ := d.WaitTask(context.Background(), tt.Id)

	err = d.StopTask(tt.Id, time.Second)
	assert.NoError(t, err)

	select {
	case res := <-waitCh:
		assert.True(t, res.Successful())
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func TestDriver_Start_Kill_Timeout(t *testing.T) {
	d, _ := NewDockerDriver(nil)

	tt := &driver.Task{
		Id: uuid.Generate(),
		Task: &proto.Task{
			Image: "busybox",
			Tag:   "1.29.3",
			Args:  []string{"sleep", "10"},
		},
	}
	_, err := d.StartTask(tt)
	assert.NoError(t, err)

	defer d.DestroyTask(tt.Id, true)

	waitCh, _ := d.WaitTask(context.Background(), tt.Id)

	err = d.StopTask(tt.Id, time.Second)
	assert.NoError(t, err)

	select {
	case res := <-waitCh:
		assert.Equal(t, res.ExitCode, int64(137))
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func TestDriver_Start_WithVolume(t *testing.T) {
	t.Skip("tested now that we can bind a volume")

	d, _ := NewDockerDriver(nil)

	tt := &driver.Task{
		Id: uuid.Generate(),
		Task: &proto.Task{
			Image: "busybox",
			Tag:   "1.29.3",
			Args:  []string{"touch", "/data/file"},
			Volumes: map[string]*proto.Task_Volume{
				"data": {Path: "/data"},
			},
		},
	}

	allocDir, err := os.MkdirTemp("/tmp", "driver-")
	require.NoError(t, err)

	_, err = d.StartTask(tt)
	assert.NoError(t, err)

	defer d.StopTask(tt.Id, 0)

	_, err = os.Stat(filepath.Join(allocDir, "data", "file"))
	require.NoError(t, err)
}

func TestDriver_Exec(t *testing.T) {
	d, _ := NewDockerDriver(nil)

	tt := &driver.Task{
		Id: uuid.Generate(),
		Task: &proto.Task{
			Image: "busybox",
			Tag:   "1.29.3",
			Args:  []string{"sleep", "10"},
		},
	}
	_, err := d.StartTask(tt)
	assert.NoError(t, err)

	defer d.DestroyTask(tt.Id, true)

	// send a command that returns true
	res, err := d.ExecTask(tt.Id, []string{"echo", "a"})
	require.NoError(t, err)

	require.Zero(t, res.ExitCode)
	require.Empty(t, res.Stderr)
	require.Equal(t, strings.TrimSpace(string(res.Stdout)), "a")

	// send a command that should fail (command not found)
	res, err = d.ExecTask(tt.Id, []string{"curl"})
	require.NoError(t, err)
	require.NotZero(t, res.ExitCode)
}

func TestDriver_BindMount(t *testing.T) {
	d, _ := NewDockerDriver(nil)

	bindDir, err := os.MkdirTemp("/tmp", "driver-")
	require.NoError(t, err)

	tt := &driver.Task{
		Id: uuid.Generate(),
		Task: &proto.Task{
			Image: "busybox",
			Tag:   "1.29.3",
			Args:  []string{"sleep", "10"},
		},
		Mounts: []*driver.MountConfig{
			{HostPath: bindDir, TaskPath: "/var"},
		},
	}

	_, err = d.StartTask(tt)
	assert.NoError(t, err)

	// touch the file /var/file.txt should be visible
	// on the bind folder as {bindDir}/file.txt
	res, err := d.ExecTask(tt.Id, []string{"touch", "/var/file.txt"})
	require.NoError(t, err)
	require.Zero(t, res.ExitCode)

	_, err = os.Stat(filepath.Join(bindDir, "file.txt"))
	require.NoError(t, err)
}

package docker

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/umbracle/vesta/internal/server/proto"
	"github.com/umbracle/vesta/internal/uuid"
)

func TestDriver_CreateContainerOptions_Labels(t *testing.T) {
	d, _ := NewDockerDriver(nil)

	tt := &proto.Task{
		Labels: map[string]string{
			"some": "label",
		},
	}
	opts, err := d.createContainerOptions(tt)
	assert.NoError(t, err)

	assert.Equal(t, opts.config.Labels["some"], "label")
	assert.Equal(t, opts.config.Labels["vesta"], "true")
}

func TestDriver_CreateContainerOptions_Env(t *testing.T) {
	d, _ := NewDockerDriver(nil)

	tt := &proto.Task{
		Env: map[string]string{
			"some": "label",
		},
	}
	opts, err := d.createContainerOptions(tt)
	assert.NoError(t, err)

	assert.Equal(t, opts.config.Env, []string{"some=label"})
}

func TestDriver_CreateContainerOptions_Image(t *testing.T) {
	d, _ := NewDockerDriver(nil)

	tt := &proto.Task{
		Image: "a",
		Tag:   "b",
	}
	opts, err := d.createContainerOptions(tt)
	assert.NoError(t, err)

	assert.Equal(t, opts.config.Image, "a:b")
}

func TestDriver_CreateContainerOptions_DataMount(t *testing.T) {
	d, _ := NewDockerDriver(nil)

	tt := &proto.Task{
		Data: map[string]string{
			"/var/file3.txt": "c",
		},
	}
	opts, err := d.createContainerOptions(tt)
	assert.NoError(t, err)

	assert.Equal(t, strings.Split(opts.host.Binds[0], ":")[1], "/var")
}

func TestDriver_Start_Wait(t *testing.T) {
	d, _ := NewDockerDriver(nil)

	tt := &proto.Task{
		Id:    uuid.Generate(),
		Image: "busybox",
		Tag:   "1.29.3",
		Args:  []string{"nc", "-l", "-p", "3000", "127.0.0.1"},
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

	tt := &proto.Task{
		Id:    uuid.Generate(),
		Image: "busybox",
		Tag:   "1.29.3",
		Args:  []string{"echo", "hello"},
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

	tt := &proto.Task{
		Id:    uuid.Generate(),
		Image: "busybox",
		Tag:   "1.29.3",
		Args:  []string{"echo", "hello"},
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

	tt := &proto.Task{
		Id:    uuid.Generate(),
		Image: "busybox",
		Tag:   "1.29.3",
		Args:  []string{"sleep", "10"},
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

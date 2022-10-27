package taskrunner

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/umbracle/vesta/internal/client/state"
	"github.com/umbracle/vesta/internal/docker"
	"github.com/umbracle/vesta/internal/server/proto"
	"github.com/umbracle/vesta/internal/testutil"
	"github.com/umbracle/vesta/internal/uuid"
)

func testWaitForTaskToStart(t *testing.T, tr *TaskRunner) {
	testutil.WaitForResult(func() (bool, error) {
		ts := tr.TaskState()
		return ts.State == proto.TaskState_Running, fmt.Errorf("expected task to be running, got %v", ts.State)
	}, func(err error) {
		require.NoError(t, err)
	})
}

func setupTaskRunner(t *testing.T, task *proto.Task) *TaskRunner {
	task.Id = uuid.Generate()

	driver, err := docker.NewDockerDriver(hclog.NewNullLogger())
	assert.NoError(t, err)

	alloc := &proto.Allocation{
		Id: uuid.Generate(),
		Deployment: &proto.Deployment{
			Tasks: []*proto.Task{task},
		},
	}

	tmpDir, err := ioutil.TempDir("/tmp", "task-runner-")
	assert.NoError(t, err)

	state, err := state.NewBoltdbStore(filepath.Join(tmpDir, "my.db"))
	assert.NoError(t, err)

	assert.NoError(t, state.PutAllocation(alloc))

	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	cfg := &Config{
		Logger:           hclog.NewNullLogger(),
		Allocation:       alloc,
		Task:             task,
		Driver:           driver,
		State:            state,
		TaskStateUpdated: func() {},
	}

	r, err := NewTaskRunner(cfg)
	assert.NoError(t, err)

	return r
}

func TestTaskRunner_Stop_ExitCode(t *testing.T) {
	tt := &proto.Task{
		Image: "busybox",
		Tag:   "1.29.3",
		Args:  []string{"sleep", "10"},
	}
	r := setupTaskRunner(t, tt)
	go r.Run()

	testWaitForTaskToStart(t, r)

	err := r.Kill(context.Background(), proto.NewTaskEvent("drop"))
	require.NoError(t, err)

	terminatedEvent := r.TaskState().Events[1]
	require.Equal(t, terminatedEvent.Type, proto.TaskTerminated)
	require.Equal(t, terminatedEvent.Details["exit_code"], "137")
}

package framework

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/client"
)

type TestingFramework struct {
	F Framework
}

// ImageTest tests that the images are correct and exist in the framework
func (tf *TestingFramework) ImageExists(t *testing.T) {
	client, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		t.Fatal(err)
	}

	// make sure chains does not fail either
	tf.F.Chains()

	cfg := &Config{
		// since we do not run validate, it does not need any input data
		Chain:   "mainnet",
		Metrics: true,
		Data: &FieldData{
			Schema: tf.F.Config(),
			Raw:    map[string]interface{}{},
		},
	}

	tasks := tf.F.Generate(cfg)
	for _, task := range tasks {
		if _, err := client.DistributionInspect(context.Background(), task.Image+":"+task.Tag, ""); err != nil {
			t.Fatal(err)
		}
	}
}

func (tf *TestingFramework) OnStartup(t *testing.T) {
	client, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		t.Fatal(err)
	}

	// make sure chains does not fail either
	tf.F.Chains()

	data := &FieldData{
		Schema: tf.F.Config(),
		Raw:    map[string]interface{}{},
	}

	// fill in the `execution_node` field which is required in the
	// beacon node clients. Later on, we will now that this field is a reference
	// to another node and create a dummy entry here.
	if _, ok := data.Schema["execution_node"]; ok {
		data.Raw["execution_node"] = "localhost"
	}

	cfg := &Config{
		// since we do not run validate, it does not need any input data
		Chain:   "mainnet",
		Metrics: true,
		Data:    data,
	}

	if err := cfg.Data.Validate(); err != nil {
		t.Fatal(err)
	}

	tasks := tf.F.Generate(cfg)

	// create a docker task for each node and make sure it runs.
	// since this nodes are only to validate the correctness of the flags, we do not want
	// to run them connected to the world in order not to DDos the network with transient nodes.
	for name, task := range tasks {
		if name == "babel" {
			continue
		}

		tmpDir, err := os.MkdirTemp("/tmp", "on-startup-test-")
		if err != nil {
			t.Fatal(err)
		}

		config := &container.Config{
			Image: task.Image + ":" + task.Tag,
			Cmd:   strslice.StrSlice(task.Args),
		}
		for k, v := range task.Env {
			config.Env = append(config.Env, k+"="+v)
		}

		host := &container.HostConfig{
			NetworkMode: "none",
		}

		// this is a naive approach for mounting files and it has more context for failure than
		// the one being used for the client. However, for the current uses cases of the mounting files feature
		// this is more than enough.
		for path, content := range task.Data {
			localPath := filepath.Join(tmpDir, filepath.Base(path))
			if err := ioutil.WriteFile(localPath, []byte(content), 0755); err != nil {
				t.Fatal(err)
			}

			host.Binds = append(host.Binds, localPath+":"+path)
		}

		body, err := client.ContainerCreate(context.Background(), config, host, &network.NetworkingConfig{}, nil, "")
		if err != nil {
			t.Fatal(err)
		}

		if err := client.ContainerStart(context.Background(), body.ID, types.ContainerStartOptions{}); err != nil {
			t.Fatal(err)
		}

		// wait at leas 2 seconds
		statusCh, errCh := client.ContainerWait(context.Background(), body.ID, container.WaitConditionNotRunning)

		select {
		case status := <-statusCh:
			t.Fatalf("exited with status %d", status.StatusCode)
		case err := <-errCh:
			t.Fatalf("failed: %v", err)
		case <-time.After(2 * time.Second):
		}

		// destroy (and remove) the container
		if err := client.ContainerRemove(context.Background(), body.ID, types.ContainerRemoveOptions{Force: true}); err != nil {
			t.Fatal(err)
		}
	}
}

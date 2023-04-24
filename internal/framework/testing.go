package framework

import (
	"context"
	"fmt"
	"io"
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

func (tf *TestingFramework) validateInput(input map[string]interface{}) error {
	client, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return err
	}

	fields := tf.F.Config()

	data := &FieldData{
		Schema: fields,
		Raw:    input,
	}

	// fill in the `execution_node` field which is required in the
	// beacon node clients. Later on, we will now that this field is a reference
	// to another node and create a dummy entry here.
	if _, ok := fields["execution_node"]; ok {
		data.Raw["execution_node"] = "localhost"
	}

	cfg := &Config{
		// since we do not run validate, it does not need any input data
		Chain:   input["chain"].(string),
		Metrics: input["metrics"].(bool),
		Data:    data,
	}

	delete(input, "chain")
	delete(input, "metrics")

	if err := cfg.Data.Validate(); err != nil {
		return err
	}

	tasks := tf.F.Generate(cfg)

	// create a docker task for each node and make sure it runs.
	// since this nodes are only to validate the correctness of the flags, we do not want
	// to run them connected to the world in order not to DDos the network with transient nodes.
	for name, task := range tasks {
		if name == "babel" {
			continue
		}

		imageName := task.Image + ":" + task.Tag

		// pull image if it does not exists
		_, _, err := client.ImageInspectWithRaw(context.Background(), imageName)
		if err != nil {
			reader, err := client.ImagePull(context.Background(), imageName, types.ImagePullOptions{})
			if err != nil {
				return err
			}
			_, err = io.Copy(ioutil.Discard, reader)
			if err != nil {
				return err
			}
		}

		tmpDir, err := os.MkdirTemp("/tmp", "on-startup-test-")
		if err != nil {
			return err
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
				return err
			}

			host.Binds = append(host.Binds, localPath+":"+path)
		}

		/*
			dockerCmd := "docker run "
			if len(host.Binds) != 0 {
				dockerCmd += "-v " + strings.Join(host.Binds, " -v ") + " "
			}
			dockerCmd += imageName + " " + strings.Join(task.Args, " ")
		*/

		body, err := client.ContainerCreate(context.Background(), config, host, &network.NetworkingConfig{}, nil, "")
		if err != nil {
			return err
		}

		if err := client.ContainerStart(context.Background(), body.ID, types.ContainerStartOptions{}); err != nil {
			return err
		}

		// wait at least 2 seconds
		statusCh, errCh := client.ContainerWait(context.Background(), body.ID, container.WaitConditionNotRunning)
		var execErr error

		select {
		case status := <-statusCh:
			execErr = fmt.Errorf("exited with status %d", status.StatusCode)

		case subErr := <-errCh:
			execErr = fmt.Errorf("failed: %v", subErr)

		case <-time.After(2 * time.Second):
		}

		if execErr != nil {
			return execErr
		}

		// destroy (and remove) the container
		if err := client.ContainerRemove(context.Background(), body.ID, types.ContainerRemoveOptions{Force: true}); err != nil {
			return err
		}
	}

	return nil
}

func (tf *TestingFramework) OnStartup(t *testing.T) {
	fields := tf.F.Config()

	possibleFields := map[string][]interface{}{
		"metrics": {true, false},
	}
	for name, field := range fields {
		if field.AllowedValues != nil {
			possibleFields[name] = field.AllowedValues
		}
	}
	chains := []interface{}{}
	for _, c := range tf.F.Chains() {
		chains = append(chains, c)
	}
	possibleFields["chain"] = chains

	for _, input := range generateMinimumCombinations(possibleFields) {
		if err := tf.validateInput(input); err != nil {
			t.Fatal(err)
		}
	}
}

func generateMinimumCombinations(vals map[string][]interface{}) []map[string]interface{} {
	// count up to which value for 'vals' we have use already
	// for each key
	keys := map[string]int{}
	for key := range vals {
		keys[key] = 0
	}

	combinations := []map[string]interface{}{}

	for {
		res := map[string]interface{}{}

		isEmpty := true

		// for each value in vals, figure out which entry we are going to use
		// incrementally
		for key, count := range keys {
			val := vals[key][count]
			res[key] = val

			if len(vals[key])-1 > count {
				isEmpty = false
				// increase to use the next value
				keys[key] += 1
			}
		}

		combinations = append(combinations, res)

		if isEmpty {
			break
		}
	}

	return combinations
}

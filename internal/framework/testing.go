package framework

import (
	"context"
	"testing"

	"github.com/docker/docker/client"
)

type TestingFramework struct {
	F Framework
}

// ImageTest tests that the images are correct and exist in the framework
func (tf *TestingFramework) ImageExists(t *testing.T, data map[string]interface{}) {
	client, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		t.Fatal(err)
	}

	cfg := &Config{
		Chain: "mainnet",
		Data: &FieldData{
			Schema: tf.F.Config(),
		},
	}

	tasks := tf.F.Generate(cfg)
	for _, task := range tasks {
		if _, err := client.DistributionInspect(context.Background(), task.Image+":"+task.Tag, ""); err != nil {
			t.Fatal(err)
		}
	}
}

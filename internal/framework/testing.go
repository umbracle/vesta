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

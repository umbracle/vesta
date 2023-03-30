package catalog

import (
	"github.com/umbracle/vesta/internal/framework"
	"github.com/umbracle/vesta/internal/server/proto"
)

type Teku struct {
}

type tekuConfig struct {
	ExecutionNode string `mapstructure:"execution_node"`
}

func (t *Teku) Config() interface{} {
	return &tekuConfig{}
}

func (t *Teku) Generate(config *framework.Config) map[string]*proto.Task {
	cc := config.Custom.(*tekuConfig)

	tt := &proto.Task{
		Image: "consensys/teku",
		Tag:   "22.8.0",
		Args: []string{
			"--data-base-path", "/data",
			"--network", "goerli",

			"--ee-endpoint",
			"http://" + cc.ExecutionNode + ":8551",
			"--ee-jwt-secret-file",
			"/var/lib/jwtsecret/jwt.hex",

			// metrics
			"--metrics-host-allowlist", "*",
			"--metrics-port", "8008",
			"--metrics-interface", "0.0.0.0",
		},
		Data: map[string]string{
			"/var/lib/jwtsecret/jwt.hex": jwtToken,
		},
		Volumes: map[string]*proto.Task_Volume{
			"data": {
				Path: "/data",
			},
		},
	}

	if config.Metrics {
		tt.Args = append(tt.Args, "--metrics-enabled")

		tt.Telemetry = &proto.Task_Telemetry{
			Port: 8008,
			Path: "metrics",
		}
	}

	return map[string]*proto.Task{
		"teku": tt,
	}
}

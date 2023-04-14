package catalog

import (
	"fmt"

	"github.com/umbracle/vesta/internal/framework"
	"github.com/umbracle/vesta/internal/server/proto"
)

type teku struct {
	*framework.Backend
}

func newTeku() framework.Framework {
	var b teku

	b.Backend = &framework.Backend{
		Fields:     b.ConfigFields(),
		ListChains: b.GetChains(),
		GenerateFn: b.GenerateFn2,
	}

	return &b
}

func (t *teku) ConfigFields() map[string]*framework.Field {
	return map[string]*framework.Field{
		"execution_node": {
			Required:    true,
			Type:        framework.TypeString,
			Description: "Endpoint of the execution node",
		},
	}
}

func (t *teku) GetChains() []string {
	return []string{
		"mainnet",
		"goerli",
		"sepolia",
	}
}

func (t *teku) GenerateFn2(config *framework.Config) map[string]*proto.Task {

	tt := &proto.Task{
		Image: "consensys/teku",
		Tag:   "23.3.0",
		Args: []string{
			"--data-base-path", "/data",
			"--ee-endpoint",
			"http://" + config.Data.GetString("execution_node") + ":8551",
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

	if config.Chain != mainnetChain {
		tt.Args = append(tt.Args, "--network", fmt.Sprintf("--%s", config.Chain))
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

package catalog

import (
	"github.com/umbracle/vesta/internal/framework"
	"github.com/umbracle/vesta/internal/server/proto"
)

type lighthouse struct {
	*framework.Backend
}

func newLighthouse() framework.Framework {
	var b lighthouse

	b.Backend = &framework.Backend{
		Fields:     b.ConfigFields(),
		ListChains: b.GetChains(),
		GenerateFn: b.GenerateFn2,
	}

	return &b
}

func (l *lighthouse) ConfigFields() map[string]*framework.Field {
	return map[string]*framework.Field{
		"execution_node": {
			Required:    true,
			Type:        framework.TypeString,
			Description: "Endpoint of the execution node",
		},
	}
}

func (l *lighthouse) GetChains() []string {
	return []string{
		"mainnet",
		"goerli",
		"sepolia",
	}
}

func (l *lighthouse) GenerateFn2(config *framework.Config) map[string]*proto.Task {
	t := &proto.Task{
		Image: "sigp/lighthouse",
		Tag:   "v4.0.1",
		Args: []string{
			"lighthouse",
			"bn",
			"--network", config.Chain,
			"--datadir", "/data",
			"--http",
			"--http-address", "0.0.0.0",
			"--http-port", "5052",
			"--execution-jwt", "/var/lib/jwtsecret/jwt.hex",
			"--execution-endpoint", "http://" + config.Data.GetString("execution_node") + ":8551",
			"--metrics-address", "0.0.0.0",
			"--metrics-port", "8008",
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
		t.Args = append(t.Args, "--metrics")

		t.Telemetry = &proto.Task_Telemetry{
			Port: 8008,
			Path: "metrics",
		}
	}

	return map[string]*proto.Task{
		"lighthouse": t,
	}
}

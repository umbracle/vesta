package catalog

import (
	"github.com/umbracle/vesta/internal/framework"
	"github.com/umbracle/vesta/internal/server/proto"
)

type besu struct {
	*framework.Backend
}

func newBesu() framework.Framework {
	var b besu

	b.Backend = &framework.Backend{
		Fields:     b.ConfigFields(),
		ListChains: b.GetChains(),
		GenerateFn: b.GenerateFn2,
	}

	return &b
}

func (b *besu) ConfigFields() map[string]*framework.Field {
	return map[string]*framework.Field{}
}

func (b *besu) GetChains() []string {
	return []string{
		"mainnet",
		"goerli",
		"sepolia",
	}
}

func (b *besu) GenerateFn2(config *framework.Config) map[string]*proto.Task {
	tt := &proto.Task{
		Image: "hyperledger/besu",
		Tag:   "latest",
		Args: []string{
			"--data-path",
			"/data",
			"--network", config.Chain,
			"--rpc-http-enabled",
			"--rpc-http-host", "0.0.0.0",
			"--rpc-http-port", "8545",
			"--rpc-http-cors-origins", "*",

			"--host-allowlist", "*",
			"--engine-host-allowlist", "*",
			"--engine-jwt-secret", "/var/lib/jwtsecret/jwt.hex",
			"--engine-rpc-port", "8551",

			"--metrics-host", "0.0.0.0",
			"--metrics-port", "6060",
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
			Port: 6060,
			Path: "metrics",
		}
	}

	return map[string]*proto.Task{
		"besu": tt,
	}
}

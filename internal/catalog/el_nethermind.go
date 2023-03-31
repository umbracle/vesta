package catalog

import (
	"github.com/umbracle/vesta/internal/framework"
	"github.com/umbracle/vesta/internal/server/proto"
)

type Netermind struct {
}

type nethermindConfig struct {
	A uint64
}

func (n *Netermind) Config() interface{} {
	return &nethermindConfig{}
}

func (n *Netermind) Chains() []string {
	return []string{
		"mainnet",
		"goerli",
		"sepolia",
	}
}

func (n *Netermind) Generate(config *framework.Config) map[string]*proto.Task {
	tt := &proto.Task{
		Image: "nethermind/nethermind",
		Tag:   "1.17.3",
		Args: []string{
			"--datadir",
			"/data",
			"--config", config.Chain,
			"--JsonRpc.Enabled", "true",
			"--JsonRpc.Host", "0.0.0.0",
			"--JsonRpc.Port", "8545",
			"--JsonRpc.EngineHost", "0.0.0.0",
			"--JsonRpc.EnginePort", "8551",
			"--JsonRpc.JwtSecretFile", "/var/lib/jwtsecret/jwt.hex",
			"--Metrics.ExposePort", "6060",
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
		tt.Args = append(tt.Args, "--Metrics.Enabled", "true")

		tt.Telemetry = &proto.Task_Telemetry{
			Port: 6060,
			Path: "metrics",
		}
	}

	babel := &proto.Task{
		Image: "babel",
		Tag:   "dev",
		Args: []string{
			"--plugin", "ethereum_el", "server", "url=http://0.0.0.0:8545",
		},
	}

	return map[string]*proto.Task{
		"nethermind": tt,
		"babel":      babel,
	}
}

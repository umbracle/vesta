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

func (n *Netermind) Generate(config *framework.Config) map[string]*proto.Task {
	var network string
	if config.Chain == sepoliaChain {
		network = "sepolia"
	} else if config.Chain == goerliChain {
		network = "goerli"
	} else if config.Chain == mainnetChain {
		network = "mainnet"
	}

	tt := &proto.Task{
		Image: "nethermind/nethermind",
		Tag:   "1.17.3",
		Args: []string{
			"--datadir",
			"/data",
			"--config", network,
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

	return map[string]*proto.Task{
		"nethermind": tt,
	}
}

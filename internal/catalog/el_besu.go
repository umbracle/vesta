package catalog

import (
	"github.com/umbracle/vesta/internal/framework"
	"github.com/umbracle/vesta/internal/server/proto"
)

type Besu struct {
}

type besuConfig struct {
	A uint64
}

func (b *Besu) Config() interface{} {
	return &besuConfig{}
}

func (b *Besu) Generate(config *framework.Config) map[string]*proto.Task {
	tt := &proto.Task{
		Image: "hyperledger/besu",
		Tag:   "latest",
		Args: []string{
			"--data-path",
			"/data",

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

	if config.Chain == sepoliaChain {
		tt.Args = append(tt.Args, "--network", "sepolia")
	} else if config.Chain == goerliChain {
		tt.Args = append(tt.Args, "--network", "goerli")
	} else if config.Chain != mainnetChain {
		tt.Args = append(tt.Args, "--network", "mainnet")
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

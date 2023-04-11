package catalog

import (
	"github.com/umbracle/vesta/internal/framework"
	"github.com/umbracle/vesta/internal/server/proto"
)

type Geth struct {
}

type gethConfig struct {
	A uint64
}

func (g *Geth) Config() interface{} {
	return &gethConfig{}
}

func (g *Geth) Generate(config *framework.Config) map[string]*proto.Task {
	tt := &proto.Task{
		Image: "ethereum/client-go",
		Tag:   "v1.11.5",
		Args: []string{
			"--datadir", "/data",
			// Http api
			"--http.addr", "0.0.0.0",
			"--http", "--http.port", "8545",
			"--http.vhosts", "*",
			"--http.corsdomain", "*",
			// Engine api
			"--authrpc.addr", "0.0.0.0",
			"--authrpc.port", "8551",
			"--authrpc.vhosts", "*",
			"--authrpc.jwtsecret", "/var/lib/jwtsecret/jwt.hex",
			// Metrics
			"--metrics.addr", "0.0.0.0",
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
		tt.Args = append(tt.Args, "--sepolia")
	} else if config.Chain == goerliChain {
		tt.Args = append(tt.Args, "--goerli")
	} else if config.Chain != mainnetChain {
		// mainnet is enabled by default, if the result is not
		// that, we have an incorrect network name
		panic("BAD chain")
	}

	if config.Metrics {
		tt.Args = append(tt.Args, "--metrics")

		tt.Telemetry = &proto.Task_Telemetry{
			Port: 6060,
			Path: "debug/metrics/prometheus",
		}
	}

	return map[string]*proto.Task{
		"geth": tt,
	}
}

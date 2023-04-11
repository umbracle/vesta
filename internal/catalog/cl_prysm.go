package catalog

import (
	"github.com/umbracle/vesta/internal/framework"
	"github.com/umbracle/vesta/internal/server/proto"
)

type Prysm struct {
}

type prysmConfig struct {
	ExecutionNode string `mapstructure:"execution_node"`
}

func (p *Prysm) Config() interface{} {
	return &prysmConfig{}
}

func (p *Prysm) Generate(config *framework.Config) map[string]*proto.Task {
	cc := config.Custom.(*prysmConfig)

	t := &proto.Task{
		Image: "gcr.io/prysmaticlabs/prysm/beacon-chain",
		Tag:   "v4.0.0",
		Args: []string{
			"--datadir", "/data",
			"--execution-endpoint", "http://" + cc.ExecutionNode + ":8551",
			"--jwt-secret", "/var/lib/jwtsecret/jwt.hex",
			"--grpc-gateway-host", "0.0.0.0",
			"--grpc-gateway-port", "5052",
			"--accept-terms-of-use",
			"--monitoring-host", "0.0.0.0",
			"--monitoring-port", "8008",
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
		t.Args = append(t.Args, "--sepolia")
	} else if config.Chain == goerliChain {
		t.Args = append(t.Args, "--goerli")
	} else if config.Chain != mainnetChain {
		// mainnet is enabled by default, if the result is not
		// that, we have an incorrect network name
		panic("BAD chain")
	}

	if config.Metrics {
		t.Telemetry = &proto.Task_Telemetry{
			Port: 8008,
			Path: "metrics",
		}
	}

	return map[string]*proto.Task{
		"node": t,
	}
}

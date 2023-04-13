package catalog

import (
	"fmt"

	"github.com/umbracle/vesta/internal/framework"
	"github.com/umbracle/vesta/internal/server/proto"
)

type Prysm struct {
}

func (p *Prysm) Config() map[string]*framework.Field {
	return map[string]*framework.Field{
		"execution_node": {
			Required:    true,
			Type:        framework.TypeString,
			Description: "Endpoint of the execution node",
		},
		"use_checkpoint": {
			Type:        framework.TypeBool,
			Description: "Whether to use checkpoint initial sync",
		},
	}
}

func (p *Prysm) Chains() []string {
	return []string{
		"mainnet",
		"goerli",
		"sepolia",
	}
}

func (p *Prysm) Generate(config *framework.Config) map[string]*proto.Task {
	t := &proto.Task{
		Image: "gcr.io/prysmaticlabs/prysm/beacon-chain",
		Tag:   "v4.0.0",
		Args: []string{
			"--datadir", "/data",
			"--execution-endpoint", "http://" + config.Data.GetString("execution_node") + ":8551",
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

	if config.Data.Get("use_checkpoint").(bool) {
		url := getBeaconCheckpoint(config.Chain)
		t.Args = append(t.Args, "--checkpoint-sync-url", url, "--genesis-beacon-api-url", url)
	}

	if config.Chain != mainnetChain {
		// add '--sepolia' or '--goerli'
		t.Args = append(t.Args, fmt.Sprintf("--%s", config.Chain))
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

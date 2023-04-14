package catalog

import (
	"fmt"

	"github.com/umbracle/vesta/internal/framework"
	"github.com/umbracle/vesta/internal/server/proto"
)

type geth struct {
	*framework.Backend
}

func newGeth() framework.Framework {
	var b geth

	b.Backend = &framework.Backend{
		Fields:     b.ConfigFields(),
		ListChains: b.GetChains(),
		GenerateFn: b.GenerateFn2,
	}

	return &b
}

func (g *geth) ConfigFields() map[string]*framework.Field {
	return map[string]*framework.Field{
		"dbengine": {
			Type:          framework.TypeString,
			Description:   "Database engine to use (leveldb, pebble)",
			AllowedValues: []interface{}{"leveldb", "pebble"},
			ForceNew:      true,
			Default:       "leveldb",
		},
	}
}

func (g *geth) GetChains() []string {
	return []string{
		"mainnet",
		"goerli",
		"sepolia",
	}
}

func (g *geth) GenerateFn2(config *framework.Config) map[string]*proto.Task {
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

	if engine := config.Data.GetString("dbengine"); engine == "pebble" {
		// use the pebble database
		tt.Args = append(tt.Args, "--db.engine", "pebble")
	}

	if config.Chain != mainnetChain {
		// add '--sepolia' or '--goerli'
		tt.Args = append(tt.Args, fmt.Sprintf("--%s", config.Chain))
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

package catalog

import (
	"github.com/umbracle/vesta/internal/framework"
	"github.com/umbracle/vesta/internal/server/proto"
)

type Lighthouse struct {
}

type lighthouseConfig struct {
	ExecutionNode string `mapstructure:"execution_node"`
}

func (l *Lighthouse) Config() interface{} {
	return &lighthouseConfig{}
}

func (l *Lighthouse) Chains() []string {
	return []string{
		"mainnet",
		"goerli",
		"sepolia",
	}
}

func (l *Lighthouse) Generate(config *framework.Config) map[string]*proto.Task {
	cc := config.Custom.(*lighthouseConfig)

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
			"--execution-endpoint", "http://" + cc.ExecutionNode + ":8551",
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

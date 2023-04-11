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

func (l *Lighthouse) Generate(config *framework.Config) map[string]*proto.Task {
	cc := config.Custom.(*lighthouseConfig)

	var network string
	if config.Chain == sepoliaChain {
		network = "sepolia"
	} else if config.Chain == goerliChain {
		network = "goerli"
	} else if config.Chain == mainnetChain {
		network = "mainnet"
	}

	cmd := []string{
		"lighthouse",
		"bn",
		"--network", network,
		"--datadir", "/data",
		"--http",
		"--http-address", "0.0.0.0",
		"--http-port", "5052",
		"--execution-jwt", "/var/lib/jwtsecret/jwt.hex",
		"--execution-endpoint", "http://" + cc.ExecutionNode + ":8551",
		"--metrics-address", "0.0.0.0",
		"--metrics-port", "8008",
	}

	t := proto.NewTask()
	t.WithImage("sigp/lighthouse").
		WithTag("v4.0.1").
		WithCmd(cmd...).
		WithFile("/var/lib/jwtsecret/jwt.hex", jwtToken).
		WithVolume("data", "/data")

	if config.Metrics {
		t.WithCmd("--metrics").WithTelemetry(8080, "metrics")
	}

	return map[string]*proto.Task{
		"lighthouse": t,
	}
}

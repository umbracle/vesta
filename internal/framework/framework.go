package framework

import "github.com/umbracle/vesta/internal/server/proto"

type Framework interface {
	Config() interface{}
	Generate(config *Config) map[string]*proto.Task
}

type Config struct {
	Metrics bool
	Chain   string
	Custom  interface{}
}

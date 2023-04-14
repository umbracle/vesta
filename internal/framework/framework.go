package framework

import (
	"github.com/umbracle/vesta/internal/server/proto"
)

type Framework interface {
	Config() map[string]*Field
	Chains() []string
	Generate(config *Config) map[string]*proto.Task
}

type Config struct {
	Metrics bool
	Chain   string
	Data    *FieldData
}

var commonFields = map[string]*Field{
	"metrics": {
		Type: TypeBool,
	},
	"log_level": {
		Type: TypeString,
	},
}

type Backend struct {
	Fields     map[string]*Field
	ListChains []string
	GenerateFn func(config *Config) map[string]*proto.Task
}

func (b *Backend) Config() map[string]*Field {
	return b.Fields
}

func (b *Backend) Chains() []string {
	return b.ListChains
}

func (b *Backend) Generate(config *Config) map[string]*proto.Task {
	return b.GenerateFn(config)
}

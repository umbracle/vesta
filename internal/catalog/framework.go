package catalog

import (
	"github.com/umbracle/vesta/internal/schema"
	"github.com/umbracle/vesta/internal/server/proto"
)

type Framework interface {
	Config() map[string]*schema.Field
	Chains() []string
	Volumes() map[string]proto.VolumeStub
	Generate(config *Config) *proto.Service
	Generate2(data *schema.FieldData) *proto.Service
}

type Config struct {
	Metrics bool
	Chain   string
	Data    *schema.FieldData
}

package catalog

import (
	"embed"
	"fmt"
	"reflect"

	"github.com/mitchellh/mapstructure"
	"github.com/umbracle/vesta/internal/framework"
	"github.com/umbracle/vesta/internal/server/proto"
	"go.starlark.net/starlark"
)

//go:embed templates/*
var content embed.FS

var Catalog = map[string]framework.Framework{
	"lighthouse": newBackend("lighthouse"),
	"prysm":      newBackend("prysm"),
	"teku":       newBackend("teku"),
	"besu":       newBackend("besu"),
	"geth":       newBackend("geth"),
	"nethermind": newBackend("nethermind"),
}

var jwtToken = "04592280e1778419b7aa954d43871cb2cfb2ebda754fb735e8adeb293a88f9bf"

var (
	goerliChain  = "goerli"
	sepoliaChain = "sepolia"
	mainnetChain = "mainnet"
)

func newTestingFramework(chain string) *framework.TestingFramework {
	fr := &framework.TestingFramework{
		F: Catalog[chain],
	}
	return fr
}

func getBeaconCheckpoint(chain string) string {
	if chain == mainnetChain {
		return "https://beaconstate.info"
	} else if chain == goerliChain {
		return "https://goerli.beaconstate.info"
	} else if chain == sepoliaChain {
		return "https://sepolia.beaconstate.info"
	}
	return ""
}

type backend struct {
	thread  *starlark.Thread
	globals starlark.StringDict
	fields  map[string]*framework.Field
	chains  []string
}

func newBackend(name string) framework.Framework {
	content, err := content.ReadFile("templates/" + name + ".star")
	if err != nil {
		panic(fmt.Errorf("failed to load '%s': %v", name, err))
	}

	thread := &starlark.Thread{Name: "my thread"}
	globals, err := starlark.ExecFile(thread, "", content, nil)
	if err != nil {
		panic(fmt.Errorf("failed to exec '%s': %v", name, err))
	}

	b := &backend{
		thread:  thread,
		globals: globals,
	}

	if err := b.generateStaticConfig(); err != nil {
		panic(fmt.Errorf("failed to generate static config: %v", err))
	}

	return b
}

type field struct {
	Type          string        `mapstructure:"type"`
	Required      bool          `mapstructure:"required"`
	Default       interface{}   `mapstructure:"default"`
	ForceNew      bool          `mapstructure:"force_new"`
	Description   string        `mapstructure:"description"`
	AllowedValues []interface{} `mapstructure:"allowed_values"`
}

func (f *field) ToType() *framework.Field {
	res := &framework.Field{
		Required:      f.Required,
		Default:       f.Default,
		ForceNew:      f.ForceNew,
		Description:   f.Description,
		AllowedValues: f.AllowedValues,
	}
	if f.Type == "string" {
		res.Type = framework.TypeString
	} else if f.Type == "bool" {
		res.Type = framework.TypeBool
	} else if f.Type == "int" {
		res.Type = framework.TypeInt
	} else {
		panic(fmt.Sprintf("type '%s' not found", f.Type))
	}
	return res
}

func (b *backend) generateStaticConfig() error {
	configValue := b.globals["config"]

	var configResult map[string]*field
	if err := mapstructure.Decode(toGoValue(configValue), &configResult); err != nil {
		return err
	}

	b.fields = map[string]*framework.Field{}
	for name, res := range configResult {
		b.fields[name] = res.ToType()
	}

	// append the default configuration fields
	for name, res := range defaultConfiguration {
		b.fields[name] = res
	}

	chainsValue := b.globals["chains"]
	if err := mapstructure.Decode(toGoValue(chainsValue), &b.chains); err != nil {
		return err
	}
	return nil
}

var defaultConfiguration = map[string]*framework.Field{
	"log_level": {
		Type:          framework.TypeString,
		Default:       "info",
		Description:   "Log level for the logs emitted by the client",
		AllowedValues: []interface{}{"all", "debug", "info", "warn", "error", "silent"},
	},
}

func (b *backend) Config() map[string]*framework.Field {
	return b.fields
}

func (b *backend) Chains() []string {
	return b.chains
}

func (b *backend) Generate(config *framework.Config) map[string]*proto.Task {
	input := starlark.NewDict(1)
	input.SetKey(starlark.String("chain"), starlark.String(config.Chain))
	input.SetKey(starlark.String("metrics"), starlark.Bool(config.Metrics))

	for name := range config.Data.Schema {
		val := config.Data.Get(name)
		if str, ok := val.(string); ok {
			input.SetKey(starlark.String(name), starlark.String(str))
		} else if bol, ok := val.(bool); ok {
			input.SetKey(starlark.String(name), starlark.Bool(bol))
		} else if num, ok := val.(uint64); ok {
			input.SetKey(starlark.String(name), starlark.MakeInt(int(num)))
		} else {
			panic(fmt.Errorf("unknown type %s", reflect.TypeOf(val).Kind()))
		}
	}

	v, err := starlark.Call(b.thread, b.globals["generate"], starlark.Tuple{input}, nil)
	if err != nil {
		panic(err)
	}

	var result map[string]*proto.Task
	if err := mapstructure.Decode(toGoValue(v), &result); err != nil {
		panic(err)
	}

	return result
}

func toGoValue(v starlark.Value) interface{} {
	switch obj := v.(type) {
	case *starlark.List:
		res := []interface{}{}
		for i := 0; i < obj.Len(); i++ {
			res = append(res, toGoValue(obj.Index(i)))
		}
		return res

	case *starlark.Dict:
		res := map[string]interface{}{}
		for _, k := range obj.Keys() {
			val, ok, err := obj.Get(k)
			if err != nil {
				panic(err)
			}
			if !ok {
				panic("expected to be found")
			}
			res[string(k.(starlark.String))] = toGoValue(val)
		}
		return res

	case starlark.Int:
		val, ok := obj.Uint64()
		if !ok {
			panic(fmt.Errorf("cannot convert uint64"))
		}
		return val

	case starlark.String:
		return string(obj)

	case starlark.Bool:
		return bool(obj)

	default:
		panic(fmt.Sprintf("BUG: starlark type %s not found", v.Type()))
	}
}

package catalog

import (
	"embed"
	"fmt"

	"github.com/mitchellh/mapstructure"
	"github.com/umbracle/vesta/internal/framework"
	"github.com/umbracle/vesta/internal/server/proto"
	"go.starlark.net/starlark"
)

//go:embed templates/*
var content embed.FS

var Catalog = map[string]framework.Framework{
	"lighthouse": newBackend("lighthouse"),
	"prysm":      &Prysm{},
	"teku":       &Teku{},
	"besu":       &Besu{},
	"geth":       newBackend("geth"),
	"nethermind": &Netermind{},
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
}

func newBackend(name string) framework.Framework {
	content, err := content.ReadFile("templates/" + name + ".star")
	if err != nil {
		panic(err)
	}

	thread := &starlark.Thread{Name: "my thread"}
	globals, err := starlark.ExecFile(thread, "", content, nil)
	if err != nil {
		panic(err)
	}

	b := &backend{
		thread:  thread,
		globals: globals,
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
	} else {
		panic(fmt.Sprintf("type '%s' not found", f.Type))
	}
	return res
}

func (b *backend) Config() map[string]*framework.Field {
	v, err := starlark.Call(b.thread, b.globals["config"], starlark.Tuple{}, nil)
	if err != nil {
		panic(err)
	}

	var result map[string]*field
	if err := mapstructure.Decode(toGoValue(v), &result); err != nil {
		panic(err)
	}

	types := map[string]*framework.Field{}
	for name, res := range result {
		types[name] = res.ToType()
	}
	return types
}

func (b *backend) Chains() []string {
	v, err := starlark.Call(b.thread, b.globals["chains"], starlark.Tuple{}, nil)
	if err != nil {
		panic(err)
	}

	var result []string
	if err := mapstructure.Decode(toGoValue(v), &result); err != nil {
		panic(err)
	}
	return result
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

	case starlark.String:
		return string(obj)

	case starlark.Bool:
		return bool(obj)

	default:
		panic(fmt.Sprintf("BUG: starlark type %s not found", v.Type()))
	}
}

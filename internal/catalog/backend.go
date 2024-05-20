package catalog

import (
	"fmt"
	"reflect"

	"github.com/mitchellh/mapstructure"
	"github.com/umbracle/vesta/internal/framework"
	"github.com/umbracle/vesta/internal/server/proto"
	"go.starlark.net/starlark"
)

type backend struct {
	thread  *starlark.Thread
	globals starlark.StringDict
	name    string
	fields  map[string]*framework.Field
	chains  []string
	labels  map[string]string
}

func newBackend(content []byte) framework.Framework {
	thread := &starlark.Thread{Name: "my thread"}
	globals, err := starlark.ExecFile(thread, "", content, nil)
	if err != nil {
		panic(fmt.Errorf("failed to exec: %v", err))
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
	Type          string                `mapstructure:"type"`
	Required      bool                  `mapstructure:"required"`
	Default       interface{}           `mapstructure:"default"`
	ForceNew      bool                  `mapstructure:"force_new"`
	Description   string                `mapstructure:"description"`
	AllowedValues []interface{}         `mapstructure:"allowed_values"`
	Filters       []framework.Filter    `mapstructure:"filters"`
	Params        map[string]string     `mapstructure:"params"`
	References    *framework.References `mapstructure:"references"`
}

func (f *field) ToType() *framework.Field {
	res := &framework.Field{
		Required:      f.Required,
		Default:       f.Default,
		ForceNew:      f.ForceNew,
		Description:   f.Description,
		AllowedValues: f.AllowedValues,
		Filters:       f.Filters,
		Params:        f.Params,
		References:    f.References,
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
	nameValue := b.globals["name"]
	if err := mapstructure.Decode(toGoValue(nameValue), &b.name); err != nil {
		return err
	}

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

func (b *backend) Labels() map[string]string {
	return b.labels
}

func (b *backend) Chains() []string {
	return b.chains
}

func (b *backend) validateFn(name string, config interface{}, obj interface{}) bool {
	v, err := starlark.Call(b.thread, b.globals[name], starlark.Tuple{toStarlarkValue(config), toStarlarkValue(obj)}, nil)
	if err != nil {
		panic(err)
	}

	val, ok := toGoValue(v).(bool)
	if !ok {
		panic("it should be boolean")
	}
	return val
}

func (b *backend) Generate(config *framework.Config) *proto.Service {
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
		} else if num, ok := val.(int); ok {
			input.SetKey(starlark.String(name), starlark.MakeInt(num))
		} else {
			panic(fmt.Errorf("unknown type %s", reflect.TypeOf(val).Kind()))
		}
	}

	v, err := starlark.Call(b.thread, b.globals["generate"], starlark.Tuple{input}, nil)
	if err != nil {
		panic(err)
	}

	var result *proto.Service
	if err := mapstructure.Decode(toGoValue(v), &result); err != nil {
		panic(err)
	}

	return result
}

func toStarlarkValue(obj interface{}) starlark.Value {
	return toStarlarkValue2(reflect.ValueOf(obj))
}

func toStarlarkValue2(val reflect.Value) starlark.Value {
	switch val.Kind() {
	case reflect.Map:
		dict := &starlark.Dict{}
		for _, key := range val.MapKeys() {
			dict.SetKey(toStarlarkValue2(key), toStarlarkValue2(val.MapIndex(key)))
		}
		return dict

	case reflect.Slice:
		list := &starlark.List{}
		for i := 0; i < val.Len(); i++ {
			list.Append(toStarlarkValue2(val.Index(i)))
		}
		return list

	case reflect.Struct:
		dict := &starlark.Dict{}
		for i := 0; i < val.NumField(); i++ {
			field := val.Type().Field(i)
			dict.SetKey(starlark.String(field.Name), toStarlarkValue2(val.Field(i)))
		}
		return dict

	case reflect.String:
		return starlark.String(val.String())

	case reflect.Bool:
		return starlark.Bool(val.Bool())

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return starlark.MakeInt(int(val.Int()))

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return starlark.MakeUint(uint(val.Uint()))

	case reflect.Interface, reflect.Ptr:
		return toStarlarkValue2(val.Elem())

	default:
		panic(fmt.Sprintf("BUG: type %s not found", val.Kind()))
	}
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

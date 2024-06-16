package catalog

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/mitchellh/mapstructure"
	"github.com/umbracle/vesta/internal/schema"
	"github.com/umbracle/vesta/internal/server/proto"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

type backend struct {
	thread  *starlark.Thread
	globals starlark.StringDict
	name    string
	fields  map[string]*schema.Field
	chains  []string
	volumes map[string]proto.VolumeStub
	labels  map[string]string
}

func newBackend(content []byte) Framework {
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

func newOtherBackend(content []byte) {
	thread := &starlark.Thread{Name: "my thread"}
	globals, err := starlark.ExecFile(thread, "", content, nil)
	if err != nil {
		panic(fmt.Errorf("failed to exec: %v", err))
	}

	// convert string dict to dict (of type starlark.Value)
	// to be able to make the conversion to Go types.
	dict := starlark.NewDict(len(globals))
	for k, v := range globals {
		dict.SetKey(starlark.String(k), v)
	}

	fmt.Println(dict)

	var desc ChainDescriptionModule
	if err := toGoValueAt(dict, &desc); err != nil {
		panic(fmt.Errorf("failed to convert dict to Go type: %v", err))
	}

	fmt.Println(desc)
}

type ChainDescriptionModule struct {
	Network Network `mapstructure:"network"`
}

type Network struct {
	Name   string           `mapstructure:"name"`
	Chains map[string]Chain `mapstructure:"chains"`
}

type Chain struct {
}

type field struct {
	Type          string             `mapstructure:"type"`
	Required      bool               `mapstructure:"required"`
	Default       interface{}        `mapstructure:"default"`
	ForceNew      bool               `mapstructure:"force_new"`
	Description   string             `mapstructure:"description"`
	AllowedValues []interface{}      `mapstructure:"allowed_values"`
	Filters       []schema.Filter    `mapstructure:"filters"`
	Params        map[string]string  `mapstructure:"params"`
	References    *schema.References `mapstructure:"references"`
}

func (f *field) ToType() *schema.Field {
	res := &schema.Field{
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
		res.Type = schema.TypeString
	} else if f.Type == "bool" {
		res.Type = schema.TypeBool
	} else if f.Type == "int" {
		res.Type = schema.TypeInt
	} else {
		panic(fmt.Sprintf("type '%s' not found", f.Type))
	}
	return res
}

func (b *backend) generateStaticConfig() error {
	nameValue, ok := b.globals["name"]
	if !ok {
		return fmt.Errorf("name not found")
	}
	if err := mapstructure.Decode(toGoValue(nameValue), &b.name); err != nil {
		return err
	}

	configValue, ok := b.globals["config"]
	if !ok {
		return fmt.Errorf("config not found")
	}
	var configResult map[string]*field
	if err := mapstructure.Decode(toGoValue(configValue), &configResult); err != nil {
		return err
	}

	b.fields = map[string]*schema.Field{}
	for name, res := range configResult {
		b.fields[name] = res.ToType()
	}

	// append the default configuration fields
	for name, res := range defaultConfiguration {
		b.fields[name] = res
	}

	chainsValue, ok := b.globals["chains"]
	if !ok {
		return fmt.Errorf("chains not found")
	}
	if err := mapstructure.Decode(toGoValue(chainsValue), &b.chains); err != nil {
		return err
	}

	volumesValue, ok := b.globals["volumes"]
	if ok {
		if err := mapstructure.Decode(toGoValue(volumesValue), &b.volumes); err != nil {
			return err
		}
	}

	return nil
}

var defaultConfiguration = map[string]*schema.Field{
	"log_level": {
		Type:          schema.TypeString,
		Default:       "info",
		Description:   "Log level for the logs emitted by the client",
		AllowedValues: []interface{}{"all", "debug", "info", "warn", "error", "silent"},
	},
}

func (b *backend) Volumes() map[string]proto.VolumeStub {
	return b.volumes
}

func (b *backend) Config() map[string]*schema.Field {
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

func (b *backend) Generate(config *Config) *proto.Service {
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

type starlarkFn = func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error)

func convertToFunction(i interface{}) starlarkFn {
	val := reflect.ValueOf(i)
	if val.Kind() != reflect.Func {
		panic("not a function")
	}

	typ := val.Type()
	outNum := typ.NumOut()
	if outNum > 2 {
		panic(fmt.Errorf("expected 2 output arguments but got %d", outNum))
	}
	if outNum == 2 && !isErrorType(typ.Out(1)) {
		panic(fmt.Errorf("expected the second output argument to be an error but got %s", typ.Out(1)))
	}

	fn := func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		// convert the arguments
		if len(args) != typ.NumIn() {
			return nil, fmt.Errorf("expected %d arguments, got %d", typ.NumIn(), len(args))
		}

		// convert the input starlark arguments to Go types
		inputArgs := make([]reflect.Value, typ.NumIn())
		for i := 0; i < typ.NumIn(); i++ {
			inputArg := reflect.New(typ.In(i))
			if err := toGoValueAt(args.Index(i), inputArg.Interface()); err != nil {
				return nil, fmt.Errorf("failed to convert argument %d: %v", i, err)
			}
			inputArgs[i] = inputArg.Elem()
		}

		// call the function
		output := val.Call(inputArgs)

		// decode the error type as the last argument (if there is any)
		if outNum == 2 {
			if err := getError(output[1]); err != nil {
				return nil, err
			}
		}

		// convert the output to a starlark value
		res := output[0].Interface()
		return toStarlarkValue(res), nil
	}

	return fn
}

type starlarkModule struct {
	*starlarkstruct.Module
}

func createNewModule(name string) *starlarkModule {
	return &starlarkModule{
		Module: &starlarkstruct.Module{
			Name:    name,
			Members: starlark.StringDict{},
		},
	}
}

func (m *starlarkModule) addFunction(name string, fn interface{}) *starlarkModule {
	m.Members[name] = starlark.NewBuiltin(name, convertToFunction(fn))
	return m
}

func (b *backend) Generate2(data *schema.FieldData) *proto.Service {

	module := createNewModule("ctx").
		addFunction("get", data.Get).
		addFunction("getString", data.GetString)

	var argsTuple starlark.Tuple
	argsTuple = append(argsTuple, module)

	generateFn, ok := b.globals["generate2"]
	if !ok {
		panic("generate2 not found")
	}

	v, err := starlark.Call(b.thread, generateFn, argsTuple, nil)
	if err != nil {
		panic(err)
	}

	var result *proto.Service
	if err := mapstructure.Decode(toGoValue(v), &result); err != nil {
		panic(err)
	}

	return result
}

var errt = reflect.TypeOf((*error)(nil)).Elem()

func isErrorType(t reflect.Type) bool {
	return t.Implements(errt)
}

func getError(v reflect.Value) error {
	if v.IsNil() {
		return nil
	}

	extractedErr, ok := v.Interface().(error)
	if !ok {
		return errors.New("invalid type assertion, unable to extract error")
	}

	return extractedErr
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

func toGoValueAt(v starlark.Value, obj interface{}) error {
	val := toGoValue(v)
	fmt.Println("-- val --", val)
	if err := mapstructure.Decode(val, &obj); err != nil {
		return err
	}
	return nil
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

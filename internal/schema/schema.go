package schema

import (
	"fmt"

	"github.com/mitchellh/mapstructure"
)

type Type int

func StringToType(s string) Type {
	switch s {
	case "string":
		return TypeString
	case "bool":
		return TypeBool
	case "int":
		return TypeInt
	default:
		return TypeInvalid
	}
}

const (
	TypeInvalid Type = iota
	TypeString
	TypeBool
	TypeInt
)

func (t Type) Zero() interface{} {
	switch t {
	case TypeString:
		return ""
	case TypeBool:
		return false
	case TypeInt:
		return 0
	default:
		panic("unknown type: " + t.String())
	}
}

func (t Type) String() string {
	switch t {
	case TypeString:
		return "string"
	case TypeBool:
		return "bool"
	case TypeInt:
		return "int"
	default:
		return "unknown type"
	}
}

type FieldData struct {
	Prev   map[string]interface{}
	Raw    map[string]interface{}
	Schema map[string]*Field
}

func (d *FieldData) GetString(k string) string {
	return fmt.Sprintf("%v", d.Get(k))
}

func (d *FieldData) Get(k string) interface{} {
	schema, ok := d.Schema[k]
	if !ok {
		panic(fmt.Sprintf("field %s not in the schema", k))
	}

	value, ok := d.GetOk(k)
	if !ok || value == nil {
		value = schema.DefaultOrZero()
	}

	return value
}

func (d *FieldData) GetOk(k string) (interface{}, bool) {
	schema, ok := d.Schema[k]
	if !ok {
		return nil, false
	}

	result, ok, err := d.GetOkErr(k)
	if err != nil {
		panic(fmt.Sprintf("error reading %s: %s", k, err))
	}

	if ok && result == nil {
		result = schema.DefaultOrZero()
	}

	return result, ok
}

func (d *FieldData) GetOkErr(k string) (interface{}, bool, error) {
	schema, ok := d.Schema[k]
	if !ok {
		return nil, false, fmt.Errorf("unknown field: %q", k)
	}

	switch schema.Type {
	case TypeString, TypeBool, TypeInt:
		return d.getPrimitive(k, schema)
	default:
		return nil, false,
			fmt.Errorf("unknown field type %q for field %q", schema.Type, k)
	}
}

func (d *FieldData) getPrimitive(k string, schema *Field) (interface{}, bool, error) {
	raw, ok := d.Raw[k]
	if !ok {
		return nil, false, nil
	}

	switch t := schema.Type; t {
	case TypeString:
		var result string
		if err := mapstructure.WeakDecode(raw, &result); err != nil {
			return nil, false, err
		}
		return result, true, nil

	case TypeBool:
		var result bool
		if err := mapstructure.WeakDecode(raw, &result); err != nil {
			return nil, false, err
		}
		return result, true, nil

	case TypeInt:
		var result int
		if err := mapstructure.WeakDecode(raw, &result); err != nil {
			return nil, false, err
		}
		return result, true, nil

	default:
		panic(fmt.Sprintf("Unknown type: %s", schema.Type))
	}
}

func (d *FieldData) Validate() error {
	for field, schema := range d.Schema {
		value, ok := d.Raw[field]
		if !ok {
			if schema.Required {
				return fmt.Errorf("required field '%s' not found", field)
			}
			// add the default value
			if schema.Default != nil {
				d.Raw[field] = schema.Default
			}
			continue
		}

		_, _, err := d.getPrimitive(field, schema)
		if err != nil {
			return fmt.Errorf("error converting input %v for field %q: %v", value, field, err)
		}

		if schema.AllowedValues != nil {
			var found bool
			for _, a := range schema.AllowedValues {
				if a == value {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("field '%s' value '%s' is not an allowed value", field, value)
			}
		}
	}

	// check if there is any value in Raw that is not part of the schema
	for k := range d.Raw {
		if _, ok := d.Schema[k]; !ok {
			return fmt.Errorf("field not found '%s'", k)
		}
	}

	return nil
}

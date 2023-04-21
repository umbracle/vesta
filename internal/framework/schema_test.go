package framework

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSchema_Validate_Required(t *testing.T) {
	f := &FieldData{
		Raw: map[string]interface{}{},
		Schema: map[string]*Field{
			"a": {Type: TypeString, Required: true},
		},
	}
	require.Error(t, f.Validate())
}

func TestSchema_Validate_Allowed(t *testing.T) {
	f := &FieldData{
		Raw: map[string]interface{}{
			"a": "c",
		},
		Schema: map[string]*Field{
			"a": {Type: TypeString, AllowedValues: []interface{}{"a", "b"}},
		},
	}
	require.Error(t, f.Validate())

	f.Raw["a"] = "a"
	require.NoError(t, f.Validate())
}

func TestSchema_FieldData_Get(t *testing.T) {
	f := &FieldData{
		Raw: map[string]interface{}{
			"a": "1",
			"b": false,
			"c": "false",
		},
		Schema: map[string]*Field{
			"a": {Type: TypeString},
			"b": {Type: TypeBool},
			"c": {Type: TypeBool},
		},
	}

	for k := range f.Raw {
		_, ok, err := f.GetOkErr(k)
		require.NoError(t, err)
		require.True(t, ok)
	}
}

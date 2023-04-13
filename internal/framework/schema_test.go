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
		},
		Schema: map[string]*Field{
			"a": {Type: TypeString},
		},
	}

	require.Equal(t, "1", f.Get("a"))
	require.Equal(t, "1", f.GetString("a"))

	val, ok, err := f.GetOkErr("a")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "1", val)

	_, ok = f.GetOk("b")
	require.NoError(t, err)
	require.False(t, ok)
}

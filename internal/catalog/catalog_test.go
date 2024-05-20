package catalog

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCatalog_ProcessInput(t *testing.T) {
	cases := []struct {
		state  map[string]interface{}
		input  map[string]interface{}
		fields map[string]*Field
		result map[string]interface{}
		err    bool
	}{
		{
			// no state
			nil,
			map[string]interface{}{
				"a": "x",
			},
			map[string]*Field{
				"a": {
					Type: TypeString,
				},
			},
			map[string]interface{}{
				"a": "x",
			},
			false,
		},
		{
			// input with state override
			map[string]interface{}{
				"a": "x",
			},
			map[string]interface{}{
				"a": "y",
			},
			map[string]*Field{
				"a": {
					Type: TypeString,
				},
			},
			map[string]interface{}{
				"a": "y",
			},
			false,
		},
		{
			// input overrides force new value should fail
			map[string]interface{}{
				"a": "x",
			},
			map[string]interface{}{
				"a": "y",
			},
			map[string]*Field{
				"a": {
					Type:     TypeString,
					ForceNew: true,
				},
			},
			nil,
			true,
		},
		{
			// input overrides default not set value should fail
			map[string]interface{}{},
			map[string]interface{}{
				"a": "y",
			},
			map[string]*Field{
				"a": {
					Type:     TypeString,
					ForceNew: true,
					Default:  "x",
				},
			},
			nil,
			true,
		},
	}

	for _, c := range cases {
		result, _, err := processInput(c.fields, c.state, c.input)
		if err != nil && !c.err {
			t.Fatal(err)
		}
		if err == nil && c.err {
			t.Fatal("it should fail")
		}
		if !c.err {
			require.Equal(t, result, c.result)
		}
	}
}

func TestStarlark_Conversion(t *testing.T) {
	type X struct {
		A int
	}

	x := X{A: 10}

	val := toStarlarkValue(x)
	fmt.Println(val)

	x2 := toGoValue(val)
	fmt.Println(x2)
}

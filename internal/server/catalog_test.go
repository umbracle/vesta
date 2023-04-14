package server

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/umbracle/vesta/internal/framework"
)

func TestCatalog_ProcessInput(t *testing.T) {
	cases := []struct {
		state  map[string]interface{}
		input  map[string]interface{}
		fields map[string]*framework.Field
		result map[string]interface{}
		err    bool
	}{
		{
			// no state
			nil,
			map[string]interface{}{
				"a": "x",
			},
			map[string]*framework.Field{
				"a": {
					Type: framework.TypeString,
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
			map[string]*framework.Field{
				"a": {
					Type: framework.TypeString,
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
			map[string]*framework.Field{
				"a": {
					Type:     framework.TypeString,
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
			map[string]*framework.Field{
				"a": {
					Type:     framework.TypeString,
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

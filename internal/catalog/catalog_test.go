package catalog

import (
	"fmt"
	"testing"
)

/*
	func TestCatalog_ProcessInput(t *testing.T) {
		cases := []struct {
			state  map[string]interface{}
			input  map[string]interface{}
			fields map[string]*proto.Field
			result map[string]interface{}
			err    bool
		}{
			{
				// no state
				nil,
				map[string]interface{}{
					"a": "x",
				},
				map[string]*proto.Field{
					"a": {
						Type: schema.TypeString,
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
				map[string]*proto.Field{
					"a": {
						Type: schema.TypeString,
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
				map[string]*proto.Field{
					"a": {
						Type:     schema.TypeString,
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
				map[string]*proto.Field{
					"a": {
						Type:     schema.TypeString,
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
*/

func TestFieldData(t *testing.T) {
	b := newBackend(content)
	fmt.Println(b)
	b.Generate2(nil)
}

var content = []byte(`
name = ""

config = {}

chains = []

def generate2(ctx):
	print(ctx.get("123"))

	return {}
`)

func TestAutoGenerate(t *testing.T) {
	newOtherBackend(content2)
}

var content2 = []byte(`
network = {
	"name": "abc",
	"chains": {
		"mainnet": {},
		"sepolia": {},
		"holesky": {}
	},
	"nodetypes": {
		"execution_node": {
		},
		"consensus_node": {
		}
	}
}
`)

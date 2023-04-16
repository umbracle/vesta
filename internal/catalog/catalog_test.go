package catalog

import (
	_ "embed"
	"fmt"
	"testing"

	"go.starlark.net/starlark"
)

//go:embed templates/lighthouse.star
var lighthouseStar string

func TestCatalogXXX(t *testing.T) {

	thread := &starlark.Thread{Name: "my thread"}
	globals, err := starlark.ExecFile(thread, "", lighthouseStar, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Retrieve a module global.
	fibonacci := globals["generate"]

	input := starlark.NewDict(1)
	input.SetKey(starlark.String("chain"), starlark.String("mainnet"))

	// Call Starlark function from Go.
	v, err := starlark.Call(thread, fibonacci, starlark.Tuple{input}, nil)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(v.(*starlark.Dict))
}

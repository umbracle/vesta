package jsonnet

import (
	"fmt"
	"io/fs"
	"log"
	"strings"
	"testing"

	"github.com/google/go-jsonnet"
)

func TestStuff(t *testing.T) {
	data, err := fs.ReadFile(builtin, "builtin/manifest.jsonnet")
	if err != nil {
		panic(err)
	}

	contents := map[string][]byte{}
	fs.WalkDir(builtin, "builtin", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		data, err := fs.ReadFile(builtin, path)
		if err != nil {
			return err
		}
		contents[strings.TrimPrefix(path, "builtin/")] = data
		return nil
	})

	vm := jsonnet.MakeVM()

	importer := &jsonnet.MemoryImporter{
		Data: map[string]jsonnet.Contents{},
	}
	for k, v := range contents {
		importer.Data[k] = jsonnet.MakeContents(string(v))
	}
	vm.Importer(importer)

	jsonStr, err := vm.EvaluateAnonymousSnippet("example1.jsonnet", string(data))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(jsonStr)
}

var ctxData = `
 
`

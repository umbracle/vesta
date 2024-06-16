package jsonnet

import (
	"fmt"
	"testing"
)

func TestStuff(t *testing.T) {
	catalog := Load()

	out := catalog.GetNetwork("ethereum").Apply("prysm", map[string]interface{}{
		"chain":   "sepolia",
		"archive": false,
	})
	fmt.Println(out.Args)

	/*
		return

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

		var generateInput = &jsonnet.NativeFunction{
			Name:   "input",
			Params: ast.Identifiers{},
			Func: func(x []interface{}) (interface{}, error) {
				// return map[string]interface{}{}, nil

				return map[string]interface{}{
					"name": "geth",
					"params": map[string]interface{}{
						"chain":   "sepolia",
						"archive": false,
					},
				}, nil
			},
		}

		vm := jsonnet.MakeVM()

		importer := &jsonnet.MemoryImporter{
			Data: map[string]jsonnet.Contents{},
		}
		for k, v := range contents {
			importer.Data[k] = jsonnet.MakeContents(string(v))
		}
		vm.Importer(importer)
		vm.NativeFunction(generateInput)

		jsonStr, err := vm.EvaluateAnonymousSnippet("example1.jsonnet", string(data))
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println(jsonStr)
	*/
}

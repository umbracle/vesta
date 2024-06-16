package jsonnet

import (
	"embed"
	_ "embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"strings"

	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
	"github.com/umbracle/vesta/internal/server/proto"
)

//go:embed builtin/*
var builtin embed.FS

func Load() []*Network {
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

	var network Network
	if err := json.Unmarshal(evaluate(data, contents), &network); err != nil {
		panic(err)
	}

	network.Exex = map[string]*Exex{}
	// for each of the imported nodes, try to load now the executor
	for _, node := range network.Nodes {
		data, err := fs.ReadFile(builtin, "builtin/"+node.Name+"_exec.jsonnet")
		if err != nil {
			panic(err)
		}
		network.Exex[node.Name] = &Exex{Content: data}
	}

	return []*Network{&network}
}

type Output struct {
	Artifacts []*proto.Artifact
	Args      []string
	Files     map[string]string
}

func ApplyCode(snippet []byte, input map[string]interface{}) *Output {
	var jsonToString = &jsonnet.NativeFunction{
		Name:   "ctx",
		Params: ast.Identifiers{},
		Func: func(x []interface{}) (interface{}, error) {
			return input, nil
		},
	}

	vm := jsonnet.MakeVM()
	vm.NativeFunction(jsonToString)

	jsonStr, err := vm.EvaluateAnonymousSnippet("example1.jsonnet", string(snippet))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("-- valudation of args --")
	fmt.Println(jsonStr)

	output := &Output{}
	if err := json.Unmarshal([]byte(jsonStr), &output); err != nil {
		panic(err)
	}
	return output
}

func evaluate(snippet []byte, data map[string][]byte) []byte {
	vm := jsonnet.MakeVM()

	importer := &jsonnet.MemoryImporter{
		Data: map[string]jsonnet.Contents{},
	}
	for k, v := range data {
		importer.Data[k] = jsonnet.MakeContents(string(v))
	}
	vm.Importer(importer)

	jsonStr, err := vm.EvaluateAnonymousSnippet("example1.jsonnet", string(snippet))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(jsonStr)

	return []byte(jsonStr)
}

type Network struct {
	Network string
	Chains  map[string]*NetworkChain
	Nodes   []*Node
	Exex    map[string]*Exex
}

type Exex struct {
	Content []byte
}

type NetworkChain struct {
	Type string
}

type Node struct {
	Name    string
	Image   string
	Chains  map[string]*NodeChain
	Tags    map[string]*NodeTag
	Ports   []*proto.Task_Port
	Volumes []*proto.Task_Volume
}

type NodeChain struct {
	MaxVersion string `json:"max_version"`
	MinVersion string `json:"min_version"`
}

type NodeTag struct {
}

/*
type importer struct {
}

func (i *importer) Import(importedFrom, importedPath string) (contents jsonnet.Contents, foundAt string, err error) {

}
*/

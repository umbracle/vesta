package jsonnet

import (
	"embed"
	_ "embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"path/filepath"
	"strings"

	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
	"github.com/umbracle/vesta/internal/server/proto"
)

//go:embed std/*
var stdLib embed.FS

//go:embed builtin/*
var builtin embed.FS

type Catalog struct {
	Networks []*Network
}

func (c *Catalog) GetNetwork(name string) *Network {
	for _, network := range c.Networks {
		if strings.ToLower(network.Network) == name {
			return network
		}
	}
	return nil
}

func Load() *Catalog {
	return LoadDir(builtin)
}

func loadStd() map[string][]byte {
	contents := map[string][]byte{}

	fs.WalkDir(stdLib, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		data, err := fs.ReadFile(stdLib, path)
		if err != nil {
			return err
		}
		contents[path] = data
		return nil
	})

	return contents
}

func LoadDir(dir fs.FS) *Catalog {
	contents := map[string][]byte{}
	manifestFiles := []string{}
	fs.WalkDir(dir, ".", func(path string, d fs.DirEntry, err error) error {
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
		contents[path] = data

		if strings.HasSuffix(path, "manifest.jsonnet") {
			manifestFiles = append(manifestFiles, path)
		}
		return nil
	})

	// load all the manifest files
	var networks []*Network

	for _, manifest := range manifestFiles {
		network, err := loadNetwork(manifest, contents)
		if err != nil {
			panic(err)
		}
		networks = append(networks, network)
	}

	/*
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
	*/

	return &Catalog{Networks: networks}
}

func loadNetwork(path string, data map[string][]byte) (*Network, error) {
	vm := jsonnet.MakeVM()

	var generateInput = &jsonnet.NativeFunction{
		Name:   "input",
		Params: ast.Identifiers{},
		Func: func(x []interface{}) (interface{}, error) {
			return map[string]interface{}{}, nil
		},
	}

	importer := newImporter().withPwd(path).addFiles(data).addFiles(loadStd())

	vm.Importer(importer)
	vm.NativeFunction(generateInput)

	jsonStr, err := vm.EvaluateAnonymousSnippet("example1.jsonnet", string(data[path]))
	if err != nil {
		log.Fatal(err)
	}

	// fmt.Println(jsonStr)

	var network Network
	if err := json.Unmarshal([]byte(jsonStr), &network); err != nil {
		return nil, err
	}

	network.Contents = data
	network.ManifestPath = path
	return &network, nil
}

func (n *Network) Apply(name string, params map[string]interface{}) *Output {
	vm := jsonnet.MakeVM()

	var generateInput = &jsonnet.NativeFunction{
		Name:   "input",
		Params: ast.Identifiers{},
		Func: func(x []interface{}) (interface{}, error) {
			return map[string]interface{}{
				"name":   name,
				"params": params,
			}, nil
		},
	}

	importer := newImporter().withPwd(n.ManifestPath).addFiles(n.Contents).addFiles(loadStd())

	vm.Importer(importer)
	vm.NativeFunction(generateInput)

	jsonStr, err := vm.EvaluateAnonymousSnippet("example1.jsonnet", string(n.Contents[n.ManifestPath]))
	if err != nil {
		log.Fatal(err)
	}

	var outputAll struct {
		Nodes []struct {
			Name string
			Task *Output
		}
	}

	if err := json.Unmarshal([]byte(jsonStr), &outputAll); err != nil {
		panic(err)
	}

	// search for the correct one
	var output *Output
	for _, node := range outputAll.Nodes {
		if node.Name == name {
			output = node.Task
			break
		}
	}
	if output == nil {
		panic("not found")
	}

	return output
}

type Output struct {
	Artifacts []*proto.Artifact
	Args      []string
	Files     map[string]string
}

type Network struct {
	Network string
	Chains  map[string]*NetworkChain
	Nodes   []*Node

	Contents     map[string][]byte
	ManifestPath string
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

type importer struct {
	files map[string]jsonnet.Contents
	pwd   string
}

func newImporter() *importer {
	return &importer{
		files: map[string]jsonnet.Contents{},
	}
}

func (i *importer) withPwd(pwd string) *importer {
	i.pwd = pwd
	return i
}

func (i *importer) addFiles(data map[string][]byte) *importer {
	for k, v := range data {
		i.files[k] = jsonnet.MakeContents(string(v))
	}
	return i
}

func (i *importer) Import(importedFrom, importedPath string) (contents jsonnet.Contents, foundAt string, err error) {
	var searchPath string

	if strings.HasPrefix(importedPath, "std/") {
		searchPath = importedPath
	} else {
		if importedFrom == "" {
			importedFrom = i.pwd
		}
		searchPath = filepath.Join(filepath.Dir(importedFrom), importedPath)
	}

	result, ok := i.files[searchPath]
	if !ok {
		return jsonnet.Contents{}, "", fmt.Errorf("file not found: %s", searchPath)
	}
	return result, searchPath, nil
}

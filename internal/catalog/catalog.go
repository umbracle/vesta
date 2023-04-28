package catalog

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/mapstructure"
	"github.com/umbracle/vesta/internal/framework"
	"github.com/umbracle/vesta/internal/server/proto"
)

//go:embed builtin/*
var builtinBackends embed.FS

type Catalog struct {
	logger   hclog.Logger
	backends map[string]framework.Framework
}

func NewCatalog() (*Catalog, error) {
	c := &Catalog{
		backends: map[string]framework.Framework{},
		logger:   hclog.NewNullLogger(),
	}

	if err := c.initBuiltin(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Catalog) SetLogger(logger hclog.Logger) {
	c.logger = logger.Named("catalog")
}

func (c *Catalog) Load(path string) error {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return err
	}

	var starFiles []string
	if fileInfo.IsDir() {
		// directory
		if err := filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
			if d.IsDir() {
				return nil
			}
			if strings.HasSuffix(path, ".star") {
				starFiles = append(starFiles, path)
			}
			return nil
		}); err != nil {
			return err
		}
	} else {
		// single file
		starFiles = append(starFiles, path)
	}

	for _, starFile := range starFiles {
		starContent, err := ioutil.ReadFile(starFile)
		if err != nil {
			return err
		}

		fr := newBackend(starContent).(*backend)
		c.backends[fr.name] = fr

		c.logger.Info("Loaded backend", "name", fr.name)
	}

	return nil
}

func (c *Catalog) initBuiltin() error {
	var starFiles []string
	if err := fs.WalkDir(builtinBackends, ".", func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".star") {
			starFiles = append(starFiles, path)
		}
		return nil
	}); err != nil {
		return err
	}

	for _, starFile := range starFiles {
		starContent, err := builtinBackends.ReadFile(starFile)
		if err != nil {
			return err
		}

		fr := newBackend(starContent).(*backend)
		c.backends[fr.name] = fr
	}

	return nil
}

func (c *Catalog) Build(prev []byte, req *proto.ApplyRequest) ([]byte, map[string]*proto.Task, error) {
	cc, ok := c.backends[strings.ToLower(req.Action)]
	if !ok {
		return nil, nil, fmt.Errorf("not found plugin: %s", req.Action)
	}

	// validate that the plugin can run this chain
	var found bool
	for _, c := range cc.Chains() {
		if c == req.Chain {
			found = true
			break
		}
	}
	if !found {
		return nil, nil, fmt.Errorf("cannot run chain '%s'", req.Chain)
	}

	var prevMap map[string]interface{}
	if prev != nil {
		if err := json.Unmarshal(prev, &prevMap); err != nil {
			return nil, nil, err
		}
	}

	var inputMap map[string]interface{}
	if err := json.Unmarshal(req.Input, &inputMap); err != nil {
		return nil, nil, err
	}

	// add to input the typed parameters from the request
	if req.LogLevel != "" {
		inputMap["log_level"] = req.LogLevel
	}

	// validate the input and the state
	state, data, err := processInput(cc.Config(), prevMap, inputMap)
	if err != nil {
		return nil, nil, err
	}

	config := &framework.Config{
		Metrics: req.Metrics,
		Chain:   req.Chain,
		Data:    data,
	}

	deployableTasks := cc.Generate(config)

	rawState, err := json.Marshal(state)
	if err != nil {
		return nil, nil, err
	}

	return rawState, deployableTasks, nil
}

func processInput(fields map[string]*framework.Field, state map[string]interface{}, input map[string]interface{}) (map[string]interface{}, *framework.FieldData, error) {
	// validate that the input matches the schema
	inputData := &framework.FieldData{
		Raw:    input,
		Schema: fields,
	}
	if err := inputData.Validate(); err != nil {
		return nil, nil, fmt.Errorf("failed to validate input: %v", err)
	}

	if state != nil {
		// validate that any new value is not a forceNew field
		stateData := &framework.FieldData{
			Raw:    state,
			Schema: fields,
		}
		for k := range input {
			if fields[k].ForceNew {
				oldVal := stateData.Get(k)
				newVal := inputData.Get(k)

				if newVal != oldVal {
					return nil, nil, fmt.Errorf("force new value '%s' has changed", k)
				}
			}
		}

		// merge the input values into the state
		for k, v := range input {
			state[k] = v
		}

	} else {
		// new deployment, take as state the current input
		state = input
	}

	data := &framework.FieldData{
		Raw:    state,
		Schema: fields,
	}
	if err := data.Validate(); err != nil {
		return nil, nil, fmt.Errorf("failed to validate input: %v", err)
	}

	return state, data, nil
}

func (c *Catalog) ListPlugins() []string {
	res := []string{}
	for name := range c.backends {
		res = append(res, name)
	}
	return res
}

func (c *Catalog) GetPlugin(name string) (*proto.Item, error) {
	pl, ok := c.backends[strings.ToLower(name)]
	if !ok {
		return nil, fmt.Errorf("plugin %s not found", name)
	}

	cfg := pl.Config()

	// convert to the keys to return a list of inputs. Note, this does
	// not work if the config has nested items
	var input map[string]interface{}
	if err := mapstructure.Decode(cfg, &input); err != nil {
		return nil, err
	}

	var inputNames []string
	for name := range input {
		inputNames = append(inputNames, name)
	}

	item := &proto.Item{
		Name:   name,
		Fields: []*proto.Item_Field{},
		Chains: pl.Chains(),
	}
	for name, field := range cfg {
		item.Fields = append(item.Fields, &proto.Item_Field{
			Name:        name,
			Type:        field.Type.String(),
			Description: field.Description,
			Required:    field.Required,
		})
	}

	return item, nil
}

func newTestingFramework(f framework.Framework) *framework.TestingFramework {
	fr := &framework.TestingFramework{
		F:         f,
		Artifacts: map[string]string{},
	}
	return fr
}

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
	"github.com/umbracle/vesta/internal/schema"
	"github.com/umbracle/vesta/internal/server/proto"
)

//go:embed builtin/*
var builtinBackends embed.FS

type Catalog struct {
	logger   hclog.Logger
	backends map[string]Framework
}

func NewCatalog() (*Catalog, error) {
	c := &Catalog{
		backends: map[string]Framework{},
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
	return nil

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

func (c *Catalog) GetFields(id string, input []byte) (*schema.FieldData, []proto.VolumeStub, error) {
	cc, ok := c.backends[strings.ToLower(id)]
	if !ok {
		return nil, nil, fmt.Errorf("not found plugin: %s", id)
	}

	var inputMap map[string]interface{}
	if err := json.Unmarshal(input, &inputMap); err != nil {
		return nil, nil, err
	}

	_, data, err := processInput(cc.Config(), nil, inputMap)
	if err != nil {
		return nil, nil, err
	}
	return data, nil, nil
}

func (c *Catalog) ValidateFn(plugin string, validationFn string, config, obj interface{}) bool {
	cc, ok := c.backends[strings.ToLower(plugin)]
	if !ok {
		return false
	}

	return cc.(*backend).validateFn(validationFn, config, obj)
}

func (c *Catalog) Build2(name string, data *schema.FieldData) *proto.Service {
	cc, ok := c.backends[strings.ToLower(name)]
	if !ok {
		return nil
	}

	return cc.Generate2(data)
}

func (c *Catalog) Build(prev []byte, req *proto.ApplyRequest) (*schema.FieldData, *proto.Service, error) {
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

	// add to input the typed parameters from the request
	if req.LogLevel != "" {
		req.Input["log_level"] = req.LogLevel
	}

	// validate the input and the state
	_, data, err := processInput(cc.Config(), prevMap, req.Input)
	if err != nil {
		return nil, nil, err
	}

	config := &Config{
		Metrics: req.Metrics,
		Chain:   req.Chain,
		Data:    data,
	}

	deployableTasks := cc.Generate(config)
	return data, deployableTasks, nil
}

func processInput(fields map[string]*schema.Field, state map[string]interface{}, input map[string]interface{}) (map[string]interface{}, *schema.FieldData, error) {
	// validate that the input matches the schema
	inputData := &schema.FieldData{
		Raw:    input,
		Schema: fields,
	}
	if err := inputData.Validate(); err != nil {
		return nil, nil, fmt.Errorf("failed to validate input: %v", err)
	}

	stateCopy := map[string]interface{}{}
	for k, v := range state {
		stateCopy[k] = v
	}

	if state != nil {
		// validate that any new value is not a forceNew field
		stateData := &schema.FieldData{
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

	data := &schema.FieldData{
		Prev:   stateCopy,
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

func (c *Catalog) GetPlugin(name string) (*proto.Item, map[string]proto.VolumeStub, error) {
	pl, ok := c.backends[strings.ToLower(name)]
	if !ok {
		return nil, nil, fmt.Errorf("plugin %s not found", name)
	}

	cfg := pl.Config()

	// convert to the keys to return a list of inputs. Note, this does
	// not work if the config has nested items
	var input map[string]interface{}
	if err := mapstructure.Decode(cfg, &input); err != nil {
		return nil, nil, err
	}

	item := &proto.Item{
		Name:   name,
		Fields: cfg,
		Chains: pl.Chains(),
	}

	return item, pl.Volumes(), nil
}

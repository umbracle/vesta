package server

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/umbracle/vesta/internal/catalog"
	"github.com/umbracle/vesta/internal/framework"
	"github.com/umbracle/vesta/internal/server/proto"
)

type Catalog interface {
	Build(req *proto.ApplyRequest) (map[string]*proto.Task, error)
}

type localCatalog struct {
}

func (l *localCatalog) Build(req *proto.ApplyRequest) (map[string]*proto.Task, error) {
	var rawInput map[string]interface{}
	if err := json.Unmarshal(req.Input, &rawInput); err != nil {
		return nil, err
	}

	cc, ok := catalog.Catalog[strings.ToLower(req.Action)]
	if !ok {
		return nil, fmt.Errorf("not found plugin: %s", req.Action)
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
		return nil, fmt.Errorf("cannot run chain '%s'", req.Chain)
	}

	data := &framework.FieldData{
		Raw:    rawInput,
		Schema: cc.Config(),
	}
	if err := data.Validate(); err != nil {
		return nil, fmt.Errorf("failed to validate input: %v", err)
	}

	config := &framework.Config{
		Metrics: req.Metrics,
		Chain:   req.Chain,
		Data:    data,
	}

	deployableTasks := cc.Generate(config)
	return deployableTasks, nil
}

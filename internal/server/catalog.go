package server

import (
	"fmt"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/umbracle/vesta/internal/catalog"
	"github.com/umbracle/vesta/internal/framework"
	"github.com/umbracle/vesta/internal/server/proto"
)

type Catalog interface {
	Build(req *proto.ApplyRequest, input map[string]interface{}) (map[string]*proto.Task, error)
}

type localCatalog struct {
}

func (l *localCatalog) Build(req *proto.ApplyRequest, input map[string]interface{}) (map[string]*proto.Task, error) {
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

	customConfig := cc.Config()
	if err := mapstructure.WeakDecode(input, &customConfig); err != nil {
		return nil, err
	}

	config := &framework.Config{
		Metrics: req.Metrics,
		Chain:   req.Chain,
		Custom:  customConfig,
	}

	deployableTasks := cc.Generate(config)
	return deployableTasks, nil
}

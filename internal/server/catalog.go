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
	Build(prev []byte, req *proto.ApplyRequest) ([]byte, map[string]*proto.Task, error)
}

type localCatalog struct {
}

func (l *localCatalog) Build(prev []byte, req *proto.ApplyRequest) ([]byte, map[string]*proto.Task, error) {
	cc, ok := catalog.Catalog[strings.ToLower(req.Action)]
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

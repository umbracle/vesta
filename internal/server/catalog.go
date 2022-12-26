package server

import (
	"fmt"
	"strconv"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	cueload "cuelang.org/go/cue/load"
	"github.com/umbracle/vesta/internal/server/proto"
	"github.com/zclconf/go-cty/cty"
)

type Ref string

type Catalog struct {
	v       *cue.Value
	actions map[string]*Action
}

func NewCatalog() *Catalog {
	r := &Catalog{
		actions: map[string]*Action{},
	}
	return r
}

type InputRef struct {
	Path string
	Ref  string
}

type Field struct {
	Type cty.Type
}

type Action struct {
	r      *Catalog
	name   string
	path   cue.Path
	value  cue.Value
	fields map[string]*Field
}

func (a *Action) GetFields() map[string]*proto.Field {
	res := map[string]*proto.Field{}

	for name, f := range a.fields {
		var typ string
		switch f.Type {
		case cty.String:
			typ = "string"
		case cty.Bool:
			typ = "bool"
		}

		res[name] = &proto.Field{
			Type: typ,
		}
	}

	return res
}

func (r *Catalog) getAction(name string) *Action {
	return r.actions[name]
}

func isNodeTag(v cue.Value) (string, bool) {
	attrs := v.Attributes(cue.ValueAttr)

	for _, attr := range attrs {
		name := attr.Name()
		if name == "obj" {
			// loop over args (CSV content in attribute)
			for i := 0; i < attr.NumArgs(); i++ {
				key, _ := attr.Arg(i)
				// one or several values where provided, filter
				return key, true
			}
		}
	}

	return "", false
}

func (r *Catalog) Apply(name string, input map[string]interface{}) (*proto.Deployment, error) {
	act, ok := r.actions[name]
	if !ok {
		return nil, fmt.Errorf("action not found '%s'", name)
	}

	v := r.v

	// get the reference for the selected node type
	nodeCue := v.LookupPath(act.path)

	// TODO: Typed encoding of input
	if m, ok := input["metrics"]; ok {
		str, ok := m.(string)
		if ok {
			mm, err := strconv.ParseBool(str)
			if err != nil {
				return nil, fmt.Errorf("failed to parse bool '%s': %v", str, err)
			}
			input["metrics"] = mm
		}
	}

	// apply the input
	nodeCue = nodeCue.FillPath(cue.MakePath(cue.Str("input")), input)
	if err := nodeCue.Err(); err != nil {
		return nil, fmt.Errorf("failed to apply input: %v", err)
	}

	// decode the tasks
	tasksCue := nodeCue.LookupPath(cue.MakePath(cue.Str("tasks")))
	if err := tasksCue.Err(); err != nil {
		return nil, fmt.Errorf("failed to decode tasks: %v", err)
	}
	rawTasks := map[string]*runtimeHandler{}
	if err := tasksCue.Decode(&rawTasks); err != nil {
		return nil, fmt.Errorf("failed to decode tasks2: %v", err)
	}
	deployableTasks := map[string]*proto.Task{}
	for name, x := range rawTasks {
		deployableTasks[name] = x.ToProto(name)
	}

	dep := &proto.Deployment{
		Tasks: deployableTasks,
	}
	return dep, nil
}

func (r *Catalog) fillActions() []*Action {
	r.actions = map[string]*Action{}

	r.v.Walk(func(v cue.Value) bool {
		if _, ok := isNodeTag(v); ok {
			selectors := v.Path().Selectors()
			name := selectors[len(selectors)-1]

			vv := v.LookupPath(cue.MakePath(cue.Str("input")))

			fields := map[string]*Field{}
			for iter, _ := vv.Fields(cue.Optional(true)); iter.Next(); {
				val := iter.Value()
				name := iter.Label()

				var typ cty.Type

				ik := val.IncompleteKind()
				switch ik.String() {
				case "bool":
					typ = cty.Bool
				case "string":
					typ = cty.String
				default:
					panic("Unexpected type")
				}

				fields[name] = &Field{
					Type: typ,
				}
			}

			r.actions[name.String()] = &Action{
				r:      r,
				name:   name.String(),
				path:   v.Path(),
				value:  v,
				fields: fields,
			}
			return false
		}
		return true
	}, nil)

	res := []*Action{}
	for _, a := range r.actions {
		res = append(res, a)
	}
	return res
}

var DefaultContext = cuecontext.New()

func (r *Catalog) load() []*Action {
	instances := cueload.Instances([]string{"./pkg/vesta.io/vesta/schema.cue"}, &cueload.Config{Dir: "."})
	if len(instances) != 1 {
		panic("bad")
	}
	instance := instances[0]
	if err := instance.Err; err != nil {
		panic(err)
	}

	v := DefaultContext.BuildInstance(instance)

	if err := v.Err(); err != nil {
		panic(err)
	}
	r.v = &v

	return r.fillActions()
}

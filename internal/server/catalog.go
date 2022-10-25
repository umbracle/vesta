package server

import (
	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	cueload "cuelang.org/go/cue/load"
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

type Action struct {
	r     *Catalog
	name  string
	path  cue.Path
	value cue.Value
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

func (r *Catalog) fillActions() []*Action {
	r.actions = map[string]*Action{}

	r.v.Walk(func(v cue.Value) bool {
		if _, ok := isNodeTag(v); ok {
			selectors := v.Path().Selectors()
			name := selectors[len(selectors)-1]

			r.actions[name.String()] = &Action{
				r:     r,
				name:  name.String(),
				path:  v.Path(),
				value: v,
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

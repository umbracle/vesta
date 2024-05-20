package server

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/boltdb/bolt"
	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/mapstructure"
	"github.com/umbracle/vesta/internal/backend/docker"
	"github.com/umbracle/vesta/internal/catalog"
	"github.com/umbracle/vesta/internal/server/proto"
	"github.com/umbracle/vesta/internal/server/state"
	"github.com/umbracle/vesta/internal/uuid"
)

type Config struct {
	PersistentDB *bolt.DB
	Catalog      []string
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{}
}

type Catalog interface {
	GetFields(id string, input []byte) (*catalog.FieldData, error)
	Build(prev []byte, req *proto.ApplyRequest) (*catalog.FieldData, *proto.Service, error)
	Build2(name string, data *catalog.FieldData) *proto.Service
	ListPlugins() []string
	GetPlugin(name string) (*proto.Item, error)
	ValidateFn(plugin string, validationFn string, config, obj interface{}) bool
}

type Server struct {
	logger  hclog.Logger
	state   *state.StateStore
	catalog Catalog
	backend *docker.Docker
}

func NewServer(logger hclog.Logger, config *Config) (*Server, error) {
	var statedb *state.StateStore

	if config.PersistentDB != nil {
		s, err := state.NewStateStoreWithBoltDB(config.PersistentDB)
		if err != nil {
			return nil, err
		}
		statedb = s

	} else {
		s, err := state.NewStateStore("server.db")
		if err != nil {
			return nil, err
		}
		statedb = s
	}

	catalog, err := catalog.NewCatalog()
	if err != nil {
		return nil, err
	}

	catalog.SetLogger(logger)

	// load the custom catalogs
	for _, ctg := range config.Catalog {
		if err := catalog.Load(ctg); err != nil {
			return nil, fmt.Errorf("failed to load catalog '%s': %v", ctg, err)
		}
	}

	srv := &Server{
		logger:  logger,
		state:   statedb,
		catalog: catalog,
	}

	vestaPath, err := filepath.Abs("./data-vesta")
	if err != nil {
		return nil, err
	}
	srv.backend = docker.NewDocker(vestaPath, srv)

	return srv, nil
}

func (s *Server) UpdateEvent(event *proto.Event) {
	if err := s.state.InsertEvent(event); err != nil {
		s.logger.Error("failed to insert event", "err", err)
	}
}

func validateField(raw interface{}, schema *proto.Item_Field) (interface{}, bool, error) {
	switch t := schema.Type; t {
	case "string":
		var result string
		if err := mapstructure.WeakDecode(raw, &result); err != nil {
			return nil, false, err
		}
		return result, true, nil

	case "bool":
		var result bool
		if err := mapstructure.WeakDecode(raw, &result); err != nil {
			return nil, false, err
		}
		return result, true, nil

	case "int":
		var result int
		if err := mapstructure.WeakDecode(raw, &result); err != nil {
			return nil, false, err
		}
		return result, true, nil

	default:
		panic(fmt.Sprintf("Unknown type: %s", schema.Type))
	}
}

func (s *Server) Create(req *proto.ApplyRequest) (string, error) {
	alias := req.AllocationId
	var prestate []byte

	// get the reference to the plugin and download on the background if necessary
	backend, err := s.catalog.GetPlugin(req.Action)
	if err != nil {
		return "", fmt.Errorf("failed to get plugin '%s': %v", req.Action, err)
	}

	// validate that all the inputs are expected and the required items are present
	visited := map[string]struct{}{}
	for _, field := range backend.Fields {
		visited[field.Name] = struct{}{}

		val, ok := req.Input[field.Name]
		if !ok && field.Required {
			return "", fmt.Errorf("field '%s' is required", field.Name)
		}
		if ok {
			// validate the type of the input
			if _, _, err := validateField(val, field); err != nil {
				return "", fmt.Errorf("failed to validate field '%s': %v", field.Name, err)
			}
		}
	}

	// check if there are extra fields
	for k := range req.Input {
		if _, ok := visited[k]; !ok {
			return "", fmt.Errorf("field '%s' is not part of the schema", k)
		}
	}

	// validate any reference input with the state.
	// TODO

	// create the field data
	fieldData := catalog.FieldData{
		Raw:    req.Input,
		Schema: map[string]*catalog.Field{},
	}
	for _, field := range backend.Fields {
		fieldData.Schema[field.Name] = &catalog.Field{
			Type: catalog.StringToType(field.Type),
		}
	}

	s.catalog.Build2(req.Action, &fieldData)
	return "", nil

	// try to resolve the alias to check if we have some previous allocation
	if alias != "" {
		if prevAlloc, err := s.state.GetDeployment(alias); err == nil {
			prestate = prevAlloc.PrevState
		}
	}

	stateDiff, service, err := s.catalog.Build(prestate, req)
	if err != nil {
		return "", fmt.Errorf("failed to run plugin '%s': %v", req.Action, err)
	}

	if alias == "" {
		// generate the alias from the chain name and the node type
		alias = fmt.Sprintf("%s-%s", strings.ToLower(req.Chain), strings.ToLower(req.Action))
	}

	// Validate if the external reference is valid or not
	for fieldName, field := range stateDiff.Schema {
		if field.References != nil {
			val, ok := stateDiff.Raw[fieldName]
			if !ok {
				continue
			}
			refName := val.(string)

			// validate the reference
			var obj interface{}
			if field.References.Type == "node" {
				node, err := s.state.GetDeployment(refName)
				if err != nil {
					return "", fmt.Errorf("failed to get node deployment '%s': %v", refName, err)
				}
				obj = node.Tasks["node"]
			} else if field.References.Type == "volume" {
				if obj, err = s.state.GetVolume(refName); err != nil {
					return "", fmt.Errorf("failed to get volume '%s': %v", refName, err)
				}
			}

			if !s.catalog.ValidateFn(req.Action, field.References.FilterCriteriaFunc, stateDiff.Raw, obj) {
				return "", fmt.Errorf("failed to validate reference '%s'", refName)
			}
		}
	}

	// 2. If there are new volumes, create them. This means creating a new id for the volume
	// The volumes are going to be in the service BUT they will have no id created.
	for _, volume := range service.Volumes {
		if volume.Id == "" {
			// this will force to create the new volume, now the volume is assigned to the service
			volume.Id = uuid.Generate()
		}
	}

	// Add basic label to each task
	for name, task := range service.Tasks {
		if task.Labels == nil {
			task.Labels = map[string]string{}
		}
		task.Labels["task"] = name
		task.Labels["service"] = alias
		task.Labels["chain"] = req.Chain
	}

	newState, err := json.Marshal(stateDiff.Raw)
	if err != nil {
		return "", err
	}

	newService := &proto.Service{
		Name:      alias,
		Tasks:     service.Tasks,
		PrevState: newState,
		Volumes:   service.Volumes,
	}

	fmt.Println("-- write deployment --")
	if err := s.state.PutDeployment(newService); err != nil {
		return "", err
	}

	s.backend.RunTasks(newService)

	return alias, nil
}

func (s *Server) Apply(ctx context.Context, req *proto.ApplyRequest) (string, error) {
	id, err := s.Create(req)
	if err != nil {
		return "", err
	}

	return id, nil
}

func (s *Server) VolumeList(ctx context.Context) ([]*proto.Volume, error) {
	return s.state.GetVolumes()
}

func (s *Server) DeploymentList(ctx context.Context) ([]*proto.Service, error) {
	return s.state.GetDeployments()
}

func (s *Server) Destroy(ctx context.Context, id string) error {
	s.backend.Destroy(id)
	return nil
}

func (s *Server) CatalogList(ctx context.Context) ([]string, error) {
	return s.catalog.ListPlugins(), nil
}

func (s *Server) CatalogInspect(ctx context.Context, name string) (*proto.Item, error) {
	return s.catalog.GetPlugin(name)
}

/*
func (s *service) SubscribeEvents(req *proto.SubscribeEventsRequest, stream proto.VestaService_SubscribeEventsServer) error {
	for {
		ws := memdb.NewWatchSet()
		it := s.srv.state.SubscribeEvents(req.Service, ws)

		for obj := it.Next(); obj != nil; obj = it.Next() {
			event := obj.(*proto.Event)

			if err := stream.Send(event); err != nil {
				panic(err)
			}

			select {
			case <-stream.Context().Done():
				return nil
			default:
			}
		}

		// wait for the duties to change
		select {
		case <-ws.WatchCh(context.Background()):
		case <-stream.Context().Done():
			return nil
		}
	}
}
*/

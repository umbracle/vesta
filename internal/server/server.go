package server

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/hashicorp/go-hclog"
	"github.com/umbracle/vesta/internal/backend/docker"
	"github.com/umbracle/vesta/internal/catalog"
	"github.com/umbracle/vesta/internal/framework"
	"github.com/umbracle/vesta/internal/server/proto"
	"github.com/umbracle/vesta/internal/server/state"
	"github.com/umbracle/vesta/internal/uuid"
	"google.golang.org/grpc"
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
	GetFields(id string, input []byte) (*framework.FieldData, error)
	Build(prev []byte, req *proto.ApplyRequest) (*framework.FieldData, *proto.Service, error)
	ListPlugins() []string
	GetPlugin(name string) (*proto.Item, error)
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

func (s *Server) withLoggingUnaryInterceptor() grpc.ServerOption {
	return grpc.UnaryInterceptor(s.loggingServerInterceptor)
}

func (s *Server) loggingServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	start := time.Now()
	h, err := handler(ctx, req)
	s.logger.Trace("Request", "method", info.FullMethod, "duration", time.Since(start), "error", err)
	return h, err
}

func (s *Server) Create(req *proto.ApplyRequest) (string, error) {
	alias := req.AllocationId
	var prestate []byte

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

	// 1. If the endpoints have changed, check that they are valid.
	// Force new has already being checked in Build.
	for fieldName, field := range stateDiff.Schema {
		if len(field.Filters) == 0 {
			continue
		}

		linkName := stateDiff.Raw[fieldName].(string)
		linkDeployment, err := s.state.GetDeployment(linkName)
		if err != nil {
			return "", fmt.Errorf("failed to get link deployment '%s': %v", linkName, err)
		}

		// get both labels and ports that this deployment links
		labels := map[string]string{}
		for _, task := range linkDeployment.Tasks {
			for k, v := range task.Labels {
				labels[k] = v
			}
		}

		type portWrapper struct {
			task string
			port *proto.Task_Port
		}

		ports := map[string]*portWrapper{}
		for taskName, task := range linkDeployment.Tasks {
			for _, port := range task.Ports {
				ports[port.Name] = &portWrapper{
					task: taskName,
					port: port,
				}
			}
		}

		field.Filters = append(field.Filters, framework.Filter{
			Criteria: "chain",
			Value:    req.Chain,
		})

		// check if the filters apply
		for _, filter := range field.Filters {
			if filter.Value == "" {
				// assume this is for bind ports, make sure that ports exists
				port, ok := ports[filter.Criteria]
				if !ok {
					return "", fmt.Errorf("filter criteria '%s' not found in ports", filter.Criteria)
				}
				// now try to resolve this port for this task and service
				url, err := s.backend.Connect(linkName, port.task, port.port.Port)
				if err != nil {
					return "", err
				}
				// WE HAVE TO REPLACE NOW THIS VALUE IN THE INPUT
				stateDiff.Raw[fieldName] = url
			} else {
				v, ok := labels[filter.Criteria]
				if !ok {
					return "", fmt.Errorf("filter criteria '%s' not found in labels", filter.Criteria)
				} else if v != filter.Value {
					return "", fmt.Errorf("filter criteria '%s' does not match with value '%s'", v, filter.Value)
				}
			}
		}
	}

	forceNewFields := map[string]string{}
	for name, field := range stateDiff.Schema {
		if field.ForceNew {
			forceNewFields[name] = fmt.Sprintf("%v", stateDiff.Raw[name])
		}
	}

	// If there is a param that matches the volume with a ref, check that you can use that volume
	// if the user supplies the field.
	// This is a bit hacky but it works for now.
	for fieldName, field := range stateDiff.Schema {
		fmt.Println("-- params", field.Params)

		if _, ok := field.Params["ref"]; ok {
			// check if it was supplied
			if val, ok := stateDiff.Raw[fieldName]; ok && val != nil {
				fmt.Println("-- val --", val)

				// get the volume
				volume, err := s.state.GetVolume(fmt.Sprintf("%v", val))
				if err != nil {
					return "", fmt.Errorf("failed to get volume '%s': %v", val, err)
				}

				if !reflect.DeepEqual(volume.Labels, forceNewFields) {
					return "", fmt.Errorf("force new value has changed, you cannot use this volume")
				}

				// TODO: check if volume is not being used already.
			}
		}
	}

	// 2. If there are new volumes, create them. This means creating a new id for the volume
	// The volumes are going to be in the service BUT they will have no id created.
	for _, volume := range service.Volumes {
		if volume.Id == "" {
			// this will force to create the new volume, now the volume is assigned to the service
			volume.Id = uuid.Generate()

			// to the volume also add the labels forceNew in the fields
			volume.Labels = forceNewFields
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

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	babel "github.com/umbracle/babel/sdk"
	"github.com/umbracle/vesta/internal/backend/docker"
	"github.com/umbracle/vesta/internal/catalog"
	"github.com/umbracle/vesta/internal/framework"
	"github.com/umbracle/vesta/internal/server/proto"
	"github.com/umbracle/vesta/internal/server/state"
	"github.com/umbracle/vesta/internal/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

type Config struct {
	GrpcAddr     string
	PersistentDB *bolt.DB
	Catalog      []string
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		GrpcAddr: "localhost:4003",
	}
}

type Catalog interface {
	GetFields(id string, input []byte) (*framework.FieldData, error)
	Build(prev []byte, req *proto.ApplyRequest) ([]byte, map[string]*proto.Task, error)
	ListPlugins() []string
	GetPlugin(name string) (*proto.Item, error)
}

type Server struct {
	logger     hclog.Logger
	grpcServer *grpc.Server
	state      *state.StateStore
	catalog    Catalog
	backend    *docker.Docker
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

	if err := srv.setupGRPCServer(config.GrpcAddr); err != nil {
		return nil, err
	}
	return srv, nil
}

func (s *Server) InMemoryConn() proto.VestaServiceClient {
	buffer := 1024 * 1024
	listener := bufconn.Listen(buffer)

	grpcServer := grpc.NewServer(s.withLoggingUnaryInterceptor())
	proto.RegisterVestaServiceServer(grpcServer, &service{srv: s})

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			panic(err)
		}
	}()

	conn, _ := grpc.DialContext(context.TODO(), "", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithInsecure(), grpc.WithBlock())

	client := proto.NewVestaServiceClient(conn)
	return client
}

func (s *Server) UpdateEvent(event *proto.Event) {
	if err := s.state.InsertEvent(event); err != nil {
		s.logger.Error("failed to insert event", "err", err)
	}
}

func (s *Server) setupGRPCServer(addr string) error {
	if addr == "" {
		return nil
	}
	s.grpcServer = grpc.NewServer(s.withLoggingUnaryInterceptor())
	proto.RegisterVestaServiceServer(s.grpcServer, &service{srv: s})

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	go func() {
		if err := s.grpcServer.Serve(lis); err != nil {
			s.logger.Error("failed to serve grpc server", "err", err)
		}
	}()

	s.logger.Info("GRPC Server started", "addr", addr)
	return nil
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

func (s *Server) Stop() {
	if s.grpcServer != nil {
		s.grpcServer.Stop()
	}
}

func (s *Server) Create(req *proto.ApplyRequest) (string, error) {
	allocId := req.AllocationId

	var alloc *proto.Service
	var prestate []byte

	{
		fields, err := s.catalog.GetFields(req.Action, req.Input)
		if err != nil {
			return "", err
		}

		for name, field := range fields.Schema {
			if len(field.Filters) == 0 {
				continue
			}

			// get the name of the deployment
			linkName := fields.Raw[name].(string)
			linkDeployment, err := s.state.GetDeployment(linkName)
			if err != nil {
				return "", fmt.Errorf("failed to get link deployment '%s': %v", linkName, err)
			}

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

			// also check the chain
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
					fields.Raw[name] = url
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

		// If there was some binding the input has changed so we need to replace it
		newInput, err := json.Marshal(fields.Raw)
		if err != nil {
			return "", err
		}

		fmt.Println("-- replaced input --")
		fmt.Println(string(newInput))

		req.Input = newInput
	}

	// try to resolve the references to other nodes
	if allocId != "" {
		// load allocation from the state
		var err error

		if alloc, err = s.state.GetDeployment(allocId); err != nil {
			return "", err
		}
		prestate = alloc.PrevState
	}

	newState, deployableTasks, err := s.catalog.Build(prestate, req)
	if err != nil {
		return "", fmt.Errorf("failed to run plugin '%s': %v", req.Action, err)
	}

	alias := req.Alias
	if alias == "" {
		// generate the alias from the chain name and the node type
		alias = fmt.Sprintf("%s-%s", strings.ToLower(req.Chain), strings.ToLower(req.Action))
	}

	// add a label with the alias for each task, I am assuming this alias is the service name
	for name, task := range deployableTasks {
		if task.Labels == nil {
			task.Labels = map[string]string{}
		}
		task.Labels["task"] = name
		task.Labels["service"] = alias
		task.Labels["chain"] = req.Chain

		// figure out the volumes in the task
		definedVolumes := map[string]*proto.Task_Volume{}
		if alloc != nil {
			if prevTask, ok := alloc.Tasks[name]; ok {
				for name, vol := range prevTask.Volumes {
					definedVolumes[name] = vol
				}
			}
		}

		for name, vol := range task.Volumes {
			if exists, ok := definedVolumes[name]; ok {
				vol.Id = exists.Id
			} else {
				vol.Id = uuid.Generate()
			}
		}
	}

	newService := &proto.Service{
		Name:      alias,
		Tasks:     deployableTasks,
		PrevState: newState,
	}

	if err := s.state.PutDeployment(newService); err != nil {
		return "", err
	}

	s.backend.RunTasks(newService)

	return alias, nil
}

func (s *Server) Pull(nodeId string, ws memdb.WatchSet) ([]*proto.Allocation, error) {
	tasks, err := s.state.AllocationListByNodeId(nodeId, ws)
	if err != nil {
		return nil, err
	}
	return tasks, nil
}

func (s *Server) UpdateSyncStatus(alloc, task string, status *babel.SyncStatus) error {
	realAlloc, err := s.state.GetAllocation(alloc)
	if err != nil {
		return err
	}
	if realAlloc == nil {
		return fmt.Errorf("alloc not found: %s", alloc)
	}

	realAlloc = realAlloc.Copy()
	if realAlloc.SyncStatus == nil {
		realAlloc.SyncStatus = map[string]*proto.Allocation_SyncStatus{}
	}

	realAlloc.SyncStatus[task] = &proto.Allocation_SyncStatus{
		IsSynced:     status.IsSynced,
		HighestBlock: status.HighestBlock,
		CurrentBlock: status.CurrentBlock,
		NumPeers:     status.NumPeers,
	}

	if err := s.state.UpsertAllocation(realAlloc); err != nil {
		return err
	}
	return nil
}

func (s *Server) UpdateAlloc(alloc *proto.Allocation) error {
	// merge alloc types
	realAlloc, err := s.state.GetAllocation(alloc.Id)
	if err != nil {
		return err
	}
	if realAlloc == nil {
		return fmt.Errorf("alloc not found: %s", alloc)
	}

	realAlloc = realAlloc.Copy()
	realAlloc.Status = alloc.Status
	realAlloc.TaskStates = alloc.TaskStates

	if err := s.state.UpsertAllocation(realAlloc); err != nil {
		return err
	}
	return nil
}

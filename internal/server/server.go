package server

import (
	"context"
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
	"github.com/umbracle/vesta/internal/server/proto"
	"github.com/umbracle/vesta/internal/server/state"
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

	var alloc *proto.Allocation
	var prevState []byte

	if allocId != "" {
		// load allocation from the state
		var err error

		if alloc, err = s.state.GetAllocation(allocId); err != nil {
			return "", err
		}
		prevState = alloc.InputState
	}

	_, deployableTasks, err := s.catalog.Build(prevState, req)
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
	}

	service := &proto.Service{
		Name:  alias,
		Tasks: deployableTasks,
	}
	s.backend.RunTasks(service)

	return alias, nil

	/*
		if alloc != nil {
			// update the deployment
			alloc = alloc.Copy()
			alloc.Sequence++
			alloc.Tasks = deployableTasks
			alloc.InputState = state

			if err := s.state.UpsertAllocation(alloc); err != nil {
				return "", err
			}
		} else {
			allocId = uuid.Generate()

			alloc := &proto.Allocation{
				Id:         allocId,
				NodeId:     "local",
				Tasks:      deployableTasks,
				InputState: state,
				Alias:      req.Alias,
			}
			if err := s.state.UpsertAllocation(alloc); err != nil {
				return "", err
			}
		}

		return allocId, nil
	*/
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

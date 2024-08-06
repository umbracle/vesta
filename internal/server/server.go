package server

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/boltdb/bolt"
	"github.com/hashicorp/go-hclog"
	"github.com/umbracle/vesta/internal/backend"
	"github.com/umbracle/vesta/internal/catalog"
	"github.com/umbracle/vesta/internal/server/proto"
	"github.com/umbracle/vesta/internal/server/state2"
	"github.com/umbracle/vesta/internal/uuid"
	"google.golang.org/grpc"
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
	state2     *state2.State
	// state      *state.StateStore
	catalog Catalog
	swarm   *backend.Swarm
}

func NewServer(logger hclog.Logger, config *Config) (*Server, error) {
	//var statedb *state.StateStore

	/*
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
	*/

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

	state, err := state2.NewState("example.db")
	if err != nil {
		return nil, err
	}

	srv := &Server{
		logger: logger,
		// state:   statedb,
		state2:  state,
		catalog: catalog,
	}

	srv.swarm = backend.NewSwarm(srv)

	if err := srv.setupGRPCServer(config.GrpcAddr); err != nil {
		return nil, err
	}
	return srv, nil
}

func (s *Server) UpdateEvent(event *proto.Event2) {
	s.logger.Info("creating event", "deployment", event.Deployment, "task", event.Task, "type", event.Type)

	if err := s.state2.CreateEvent(event); err != nil {
		s.logger.Error("failed to create event", "err", err)
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
	s.grpcServer.Stop()
}

func (s *Server) Create(req *proto.ApplyRequest) (string, error) {
	allocId := req.AllocationId

	var alloc *proto.Deployment2
	var prevState []byte

	if allocId != "" {
		// load allocation from the state
		var err error

		if alloc, err = s.state2.GetDeploymentById(allocId); err != nil {
			return "", err
		}
		prevState = alloc.Spec
	}

	_, deployableTasks, err := s.catalog.Build(prevState, req)
	if err != nil {
		return "", fmt.Errorf("failed to run plugin '%s': %v", req.Action, err)
	}

	if alloc != nil {
		s.logger.Info("updating deployment")

		// update the deployment
		if err := s.state2.UpdateDeployment(alloc); err != nil {
			return "", err
		}
	} else {
		// create a new deployment
		allocId = uuid.Generate()

		s.logger.Info("creating deployment", "id", allocId)

		alloc := &proto.Deployment2{
			Id:   allocId,
			Spec: req.Input,
		}
		if err := s.state2.CreateDeployment(alloc); err != nil {
			return "", err
		}
	}

	// do it here because if we create the deployment the alloc id is generated now
	for _, task := range deployableTasks {
		if task.Labels == nil {
			task.Labels = map[string]string{}
		}
		task.Labels["deployment"] = allocId
	}

	if err := s.swarm.Deploy("test", deployableTasks); err != nil {
		panic(err)
	}

	return allocId, nil
}

/*
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
*/

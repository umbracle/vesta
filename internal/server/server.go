package server

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/mitchellh/mapstructure"
	"github.com/umbracle/vesta/internal/catalog"
	"github.com/umbracle/vesta/internal/framework"
	"github.com/umbracle/vesta/internal/server/proto"
	"github.com/umbracle/vesta/internal/server/state"
	"github.com/umbracle/vesta/internal/uuid"
	"google.golang.org/grpc"
)

type Config struct {
	GrpcAddr string
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		GrpcAddr: "localhost:4003",
	}
}

type Server struct {
	logger     hclog.Logger
	grpcServer *grpc.Server
	state      *state.StateStore
}

func NewServer(logger hclog.Logger, config *Config) (*Server, error) {
	srv := &Server{
		logger: logger,
		state:  state.NewStateStore(),
	}

	if err := srv.setupGRPCServer(config.GrpcAddr); err != nil {
		return nil, err
	}
	return srv, nil
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

func (s *Server) Create(req *proto.ApplyRequest, input map[string]interface{}) (string, error) {
	cc, ok := catalog.Catalog[strings.ToLower(req.Action)]
	if !ok {
		return "", fmt.Errorf("not found plugin: %s", req.Action)
	}

	customConfig := cc.Config()
	if err := mapstructure.WeakDecode(input, &customConfig); err != nil {
		panic(err)
	}

	config := &framework.Config{
		Metrics: req.Metrics,
		Chain:   req.Chain,
		Custom:  customConfig,
	}

	deployableTasks := cc.Generate(config)

	allocId := req.AllocationId

	if allocId != "" {
		// update the deployment
		alloc, err := s.state.GetAllocation(allocId)
		if err != nil {
			return "", err
		}

		alloc = alloc.Copy()
		alloc.Sequence++
		alloc.Tasks = deployableTasks

		if err := s.state.UpsertAllocation(alloc); err != nil {
			return "", err
		}
	} else {
		allocId = uuid.Generate()

		alloc := &proto.Allocation{
			Id:     allocId,
			NodeId: "local",
			Tasks:  deployableTasks,
		}
		if err := s.state.UpsertAllocation(alloc); err != nil {
			return "", err
		}
	}

	return allocId, nil
}

func (s *Server) Pull(nodeId string, ws memdb.WatchSet) ([]*proto.Allocation, error) {
	tasks, err := s.state.AllocationListByNodeId(nodeId, ws)
	if err != nil {
		return nil, err
	}
	return tasks, nil
}

func (s *Server) UpdateAlloc(alloc *proto.Allocation) error {
	// TODO: Persistence
	return nil

	// merge alloc types
	realAlloc, err := s.state.GetAllocation(alloc.Id)
	if err != nil {
		return err
	}

	realAlloc.Status = alloc.Status
	realAlloc.TaskStates = alloc.TaskStates

	if err := s.state.UpsertAllocation(realAlloc); err != nil {
		return err
	}
	return nil
}

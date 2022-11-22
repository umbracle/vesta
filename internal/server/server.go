package server

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"cuelang.org/go/cue"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
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
	runner     *Catalog
	state      *state.StateStore
}

func NewServer(logger hclog.Logger, config *Config) (*Server, error) {
	srv := &Server{
		logger: logger,
		runner: NewCatalog(),
		state:  state.NewStateStore(),
	}

	srv.runner.load()
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

func (s *Server) Create(allocId string, act *Action, input map[string]interface{}) (string, error) {
	v := *s.runner.v

	// get the reference for the selected node type
	nodeCue := v.LookupPath(act.path)

	// TODO: Typed encoding of input
	if m, ok := input["metrics"]; ok {
		str, ok := m.(string)
		if ok {
			mm, err := strconv.ParseBool(str)
			if err != nil {
				return "", fmt.Errorf("failed to parse bool '%s': %v", str, err)
			}
			input["metrics"] = mm
		}
	}

	// apply the input
	nodeCue = nodeCue.FillPath(cue.MakePath(cue.Str("input")), input)
	if err := nodeCue.Err(); err != nil {
		return "", fmt.Errorf("failed to apply input: %v", err)
	}

	// decode the tasks
	tasksCue := nodeCue.LookupPath(cue.MakePath(cue.Str("tasks")))
	if err := tasksCue.Err(); err != nil {
		return "", fmt.Errorf("failed to decode tasks: %v", err)
	}
	rawTasks := map[string]*runtimeHandler{}
	if err := tasksCue.Decode(&rawTasks); err != nil {
		return "", fmt.Errorf("failed to decode tasks2: %v", err)
	}
	deployableTasks := map[string]*proto.Task{}
	for name, x := range rawTasks {
		deployableTasks[name] = x.ToProto(name)
	}

	dep := &proto.Deployment{
		Tasks: deployableTasks,
	}

	if allocId != "" {
		// update the deployment
		if err := s.state.UpdateAllocationDeployment(allocId, dep); err != nil {
			return "", err
		}
	} else {
		allocId = uuid.Generate()

		alloc := &proto.Allocation{
			Id:         allocId,
			NodeId:     "local",
			Deployment: dep,
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
	if err := s.state.UpsertAllocation(alloc); err != nil {
		return err
	}
	return nil
}

type Mount struct {
	Dest     string
	Contents string
}

type runtimeHandler struct {
	Image string
	Tag   string
	Args  []string
	Ports map[string]struct {
		Port uint64
		Type string
	}
	Env     map[string]string
	Mounts  map[string]*Mount
	Volumes map[string]struct {
		Path string
	}
}

func (r *runtimeHandler) ToProto(name string) *proto.Task {
	dataFile := map[string]string{}
	for _, m := range r.Mounts {
		dataFile[m.Dest] = m.Contents
	}

	c := &proto.Task{
		Id:      uuid.Generate(),
		Image:   r.Image,
		Name:    name,
		Tag:     r.Tag,
		Args:    r.Args,
		Env:     r.Env,
		Data:    dataFile,
		Volumes: map[string]*proto.Task_Volume{},
	}

	for name, vol := range r.Volumes {
		c.Volumes[name] = &proto.Task_Volume{
			Path: vol.Path,
		}
	}
	return c
}

package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/go-memdb"
	"github.com/umbracle/vesta/internal/server/proto"
)

type service struct {
	proto.UnimplementedVestaServiceServer

	srv *Server
}

func (s *service) Apply(ctx context.Context, req *proto.ApplyRequest) (*proto.ApplyResponse, error) {
	act := s.srv.runner.getAction(req.Action)
	if act == nil {
		return nil, fmt.Errorf("action '%s' not found", req.Action)
	}
	var input map[string]interface{}
	if err := json.Unmarshal(req.Input, &input); err != nil {
		return nil, err
	}

	// create
	id, err := s.srv.Create(req.AllocationId, act, input)
	if err != nil {
		return nil, err
	}

	return &proto.ApplyResponse{Id: id}, nil
}

func (s *service) DeploymentList(ctx context.Context, req *proto.ListDeploymentRequest) (*proto.ListDeploymentResponse, error) {
	ws := memdb.NewWatchSet()
	allocs, err := s.srv.state.AllocationList(ws)
	if err != nil {
		return nil, err
	}

	resp := &proto.ListDeploymentResponse{
		Allocations: allocs,
	}
	return resp, nil
}

func (s *service) DeploymentStatus(ctx context.Context, req *proto.DeploymentStatusRequest) (*proto.DeploymentStatusResponse, error) {
	allocation, err := s.srv.state.GetAllocation(req.Id)
	if err != nil {
		return nil, err
	}
	if allocation == nil {
		return nil, fmt.Errorf("not found")
	}
	resp := &proto.DeploymentStatusResponse{
		Allocation: allocation,
	}
	return resp, nil
}

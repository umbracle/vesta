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
	var input map[string]interface{}
	if err := json.Unmarshal(req.Input, &input); err != nil {
		return nil, err
	}

	dep, err := s.srv.catalog.Apply(req.Action, input)
	if err != nil {
		return nil, fmt.Errorf("failed to plan dep: %v", err)
	}

	// create
	id, err := s.srv.Create(req.AllocationId, dep)
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

func (s *service) Destroy(ctx context.Context, req *proto.DestroyRequest) (*proto.DestroyResponse, error) {
	if err := s.srv.state.DestroyAllocation(req.Id); err != nil {
		return nil, err
	}
	return &proto.DestroyResponse{}, nil
}

func (s *service) CatalogList(ctx context.Context, req *proto.CatalogListRequest) (*proto.CatalogListResponse, error) {
	resp := &proto.CatalogListResponse{
		Dep: []string{},
	}
	for name := range s.srv.catalog.actions {
		resp.Dep = append(resp.Dep, name)
	}
	return resp, nil
}

func (s *service) CatalogEntry(ctx context.Context, req *proto.CatalogEntryRequest) (*proto.CatalogEntryResponse, error) {
	act := s.srv.catalog.getAction(req.Action)
	if act == nil {
		return nil, fmt.Errorf("action %s does not exists", req.Action)
	}
	fields := act.GetFields()

	resp := &proto.CatalogEntryResponse{
		Fields: fields,
	}
	return resp, nil
}

package server

import (
	"context"

	"github.com/umbracle/vesta/internal/server/proto"
)

type service struct {
	proto.UnimplementedVestaServiceServer

	srv *Server
}

func (s *service) Apply(ctx context.Context, req *proto.ApplyRequest) (*proto.ApplyResponse, error) {
	// create
	id, err := s.srv.Create(req)
	if err != nil {
		return nil, err
	}

	return &proto.ApplyResponse{Id: id}, nil
}

func (s *service) DeploymentList(ctx context.Context, req *proto.ListDeploymentRequest) (*proto.ListDeploymentResponse, error) {
	allocs, err := s.srv.state2.ListDeployments()
	if err != nil {
		return nil, err
	}

	resp := &proto.ListDeploymentResponse{
		Allocations: allocs,
	}
	return resp, nil
}

func (s *service) DeploymentStatus(ctx context.Context, req *proto.DeploymentStatusRequest) (*proto.DeploymentStatusResponse, error) {
	deployment, err := s.srv.state2.GetDeploymentById(req.Id)
	if err != nil {
		return nil, err
	}

	events, err := s.srv.state2.GetEventsByDeployment(req.Id)
	if err != nil {
		return nil, err
	}

	resp := &proto.DeploymentStatusResponse{
		Allocation: deployment,
		Events:     events,
	}
	return resp, nil
}

func (s *service) Destroy(ctx context.Context, req *proto.DestroyRequest) (*proto.DestroyResponse, error) {
	/*
		alloc, err := s.srv.state.AllocationByAliasOrIDOrPrefix(req.Id)
		if err != nil {
			return nil, err
		}

		if err := s.srv.state.DestroyAllocation(alloc.Id); err != nil {
			return nil, err
		}
		return &proto.DestroyResponse{}, nil
	*/
	return nil, nil
}

func (s *service) CatalogList(ctx context.Context, req *proto.CatalogListRequest) (*proto.CatalogListResponse, error) {
	resp := &proto.CatalogListResponse{
		Plugins: s.srv.catalog.ListPlugins(),
	}
	return resp, nil
}

func (s *service) CatalogInspect(ctx context.Context, req *proto.CatalogInspectRequest) (*proto.CatalogInspectResponse, error) {
	item, err := s.srv.catalog.GetPlugin(req.Name)
	if err != nil {
		return nil, err
	}

	resp := &proto.CatalogInspectResponse{
		Item: item,
	}
	return resp, nil
}

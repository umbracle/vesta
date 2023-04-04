package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/go-memdb"
	"github.com/mitchellh/mapstructure"
	"github.com/umbracle/vesta/internal/catalog"
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

	// create
	id, err := s.srv.Create(req, input)
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
		Plugins: []string{},
	}
	for name := range catalog.Catalog {
		resp.Plugins = append(resp.Plugins, name)
	}
	return resp, nil
}

func (s *service) CatalogInspect(ctx context.Context, req *proto.CatalogInspectRequest) (*proto.CatalogInspectResponse, error) {
	pl, ok := catalog.Catalog[strings.ToLower(req.Name)]
	if !ok {
		return nil, fmt.Errorf("plugin %s not found", req.Name)
	}

	cfg := pl.Config()

	// convert to the keys to return a list of inputs. Note, this does
	// not work if the config has nested items
	var input map[string]interface{}
	if err := mapstructure.Decode(cfg, &input); err != nil {
		return nil, err
	}

	var inputNames []string
	for name := range input {
		inputNames = append(inputNames, name)
	}

	resp := &proto.CatalogInspectResponse{
		Item: &proto.Item{
			Name:  req.Name,
			Input: inputNames,
		},
	}

	return resp, nil
}

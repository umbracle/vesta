package client

import (
	"context"
	"net"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/umbracle/vesta/internal/client/runner"
	cproto "github.com/umbracle/vesta/internal/client/runner/proto"
	"github.com/umbracle/vesta/internal/server/proto"
)

type Config struct {
	NodeID       string
	ControlPlane ControlPlane
	Volume       *HostVolume
}

type HostVolume struct {
	Path string
}

type Client struct {
	logger    hclog.Logger
	config    *Config
	closeCh   chan struct{}
	collector *collector
	runner    *runner.Runner
}

func NewClient(logger hclog.Logger, config *Config) (*Client, error) {
	c := &Client{
		logger:    logger.Named("agent"),
		config:    config,
		closeCh:   make(chan struct{}),
		collector: newCollector(),
	}

	go c.startCollectorPrometheusServer(&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 5555})

	go c.handle()
	c.logger.Info("agent started")

	rConfig := &runner.Config{
		Logger:            logger,
		AllocStateUpdated: c,
	}
	r, err := runner.NewRunner(rConfig)
	if err != nil {
		return nil, err
	}
	c.runner = r

	return c, nil
}

func (c *Client) handle() {
	for {
		ws := memdb.NewWatchSet()
		allocations, err := c.config.ControlPlane.Pull(c.config.NodeID, ws)
		if err != nil {
			panic(err)
		}
		for _, alloc := range allocations {
			dep2 := &cproto.Deployment{
				Name:     alloc.Id,
				Tasks:    []*cproto.Task{},
				Sequence: alloc.Sequence,
			}
			for name, tt := range alloc.Tasks {
				ttt := &cproto.Task{
					Name:        name,
					Image:       tt.Image,
					Tag:         tt.Tag,
					Args:        tt.Args,
					Env:         tt.Env,
					Labels:      tt.Labels,
					SecurityOpt: tt.SecurityOpt,
					Data:        tt.Data,
					Batch:       tt.Batch,
				}
				dep2.Tasks = append(dep2.Tasks, ttt)
			}

			c.runner.UpsertDeployment(dep2)
		}

		select {
		case <-c.closeCh:
			return
		case <-ws.WatchCh(context.Background()):
		}
	}
}

func (c *Client) AllocStateUpdated(a *cproto.Allocation) {
	// update back to the client important data
	alloc := &proto.Allocation{
		Id:         a.Deployment.Name,
		Status:     proto.Allocation_Status(a.Status),
		TaskStates: map[string]*proto.TaskState{},
	}
	for name, state := range a.TaskStates {
		alloc.TaskStates[name] = &proto.TaskState{
			State:    proto.TaskState_State(state.State),
			Failed:   state.Failed,
			Restarts: state.Restarts,
			Id:       state.Id,
			Killing:  state.Killing,
		}
	}

	if err := c.config.ControlPlane.UpdateAlloc(alloc); err != nil {
		c.logger.Error("failed to update alloc", "id", alloc.Id, "err", err)
	}
}

func (c *Client) Stop() {
	close(c.closeCh)
}

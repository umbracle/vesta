package client

import (
	"context"
	"net"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/umbracle/vesta/internal/client/allocrunner"
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
	runner    *allocrunner.Runner
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

	r, err := allocrunner.NewRunner(&allocrunner.RConfig{})
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
			c.runner.UpsertDeployment(alloc)
		}

		select {
		case <-c.closeCh:
			return
		case <-ws.WatchCh(context.Background()):
		}
	}
}

func (c *Client) AllocStateUpdated(a *proto.Allocation) {
	if err := c.config.ControlPlane.UpdateAlloc(a); err != nil {
		c.logger.Error("failed to update alloc", "id", a.Id, "err", err)
	}
}

func (c *Client) Stop() {
	close(c.closeCh)
}

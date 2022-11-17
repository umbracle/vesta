package client

import (
	"context"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/umbracle/vesta/internal/client/allocrunner"
	"github.com/umbracle/vesta/internal/client/state"
	"github.com/umbracle/vesta/internal/docker"
	"github.com/umbracle/vesta/internal/server/proto"
)

type Config struct {
	NodeID       string
	ControlPlane ControlPlane
}

type Client struct {
	logger  hclog.Logger
	config  *Config
	driver  *docker.Docker
	closeCh chan struct{}
	state   state.State
	allocs  map[string]*allocrunner.AllocRunner
}

func NewClient(logger hclog.Logger, config *Config) (*Client, error) {
	driver, err := docker.NewDockerDriver(logger)
	if err != nil {
		return nil, err
	}
	c := &Client{
		logger:  logger.Named("agent"),
		config:  config,
		driver:  driver,
		closeCh: make(chan struct{}),
		allocs:  map[string]*allocrunner.AllocRunner{},
	}

	if err := c.initState(); err != nil {
		return nil, err
	}

	go c.handle()
	c.logger.Info("agent started")

	return c, nil
}

func (c *Client) initState() error {
	state, err := state.NewBoltdbStore("client.db")
	if err != nil {
		return err
	}
	c.state = state

	allocs, err := state.GetAllocations()
	if err != nil {
		return err
	}
	if len(allocs) > 1 {
		panic("bad for testing")
	}
	for _, alloc := range allocs {
		id := alloc.Id

		config := &allocrunner.Config{
			Alloc:        alloc,
			Logger:       c.logger,
			State:        c.state,
			StateUpdater: c,
			Driver:       c.driver,
		}
		handle, err := allocrunner.NewAllocRunner(config)
		if err != nil {
			panic(err)
		}
		c.allocs[id] = handle

		if err := handle.Restore(); err != nil {
			return err
		}
	}

	for _, a := range c.allocs {
		go a.Run()
	}
	return nil
}

func (c *Client) handle() {
	handleAlloc := func(a *proto.Allocation) {
		handle, ok := c.allocs[a.Id]
		if ok {
			// update
			handle.Update(a)
		} else {
			// create
			config := &allocrunner.Config{
				Alloc:        a,
				Logger:       c.logger,
				State:        c.state,
				StateUpdater: c,
				Driver:       c.driver,
			}
			var err error
			if handle, err = allocrunner.NewAllocRunner(config); err != nil {
				panic(err)
			}

			c.allocs[a.Id] = handle
			go handle.Run()
		}

		// update allocation
		if err := c.state.PutAllocation(a); err != nil {
			panic(err)
		}
	}

	for {
		ws := memdb.NewWatchSet()
		allocations, err := c.config.ControlPlane.Pull(c.config.NodeID, ws)
		if err != nil {
			panic(err)
		}
		for _, alloc := range allocations {
			handleAlloc(alloc)
		}

		select {
		case <-c.closeCh:
			return
		case <-ws.WatchCh(context.Background()):
		}
	}
}

func (c *Client) AllocStateUpdated(a *proto.Allocation) {
	c.config.ControlPlane.UpdateAlloc(a)
}

func (c *Client) Stop() {
	close(c.closeCh)
}

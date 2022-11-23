package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/umbracle/vesta/internal/client/allocrunner"
	"github.com/umbracle/vesta/internal/client/state"
	"github.com/umbracle/vesta/internal/docker"
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

	go c.metricsLoop()

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
	for _, alloc := range allocs {
		id := alloc.Id

		config := &allocrunner.Config{
			Alloc:        alloc,
			Logger:       c.logger,
			State:        c.state,
			StateUpdater: c,
			Driver:       c.driver,
		}
		if c.config.Volume != nil {
			config.Volume = c.config.Volume.Path
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
			if c.config.Volume != nil {
				config.Volume = c.config.Volume.Path
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
	if err := c.config.ControlPlane.UpdateAlloc(a); err != nil {
		c.logger.Error("failed to update alloc", "id", a.Id, "err", err)
	}
}

func (c *Client) Stop() {
	close(c.closeCh)
}

func stringPtr(s string) *string {
	return &s
}

func (c *Client) metricsLoop() {
	for {
		res, err := http.Get("http://localhost:6060/debug/metrics/prometheus")
		if err != nil {
			c.logger.Error("failed to get url")
		} else {
			/*
				data, err := io.ReadAll(res.Body)
				if err != nil {
					panic(err)
				}
			*/
			metrics, err := getMetricFamilies(res.Body)
			if err != nil {
				panic(err)
			}

			out := new(bytes.Buffer)
			encoder := expfmt.NewEncoder(out, expfmt.FmtText)
			for _, mf := range metrics {
				for _, m := range mf.Metric {
					m.Label = append(m.Label, &dto.LabelPair{Name: stringPtr("host"), Value: stringPtr("abcd")})
				}
				encoder.Encode(mf)
			}
			fmt.Println(out.String())
		}

		select {
		case <-c.closeCh:
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func getMetricFamilies(sourceData io.Reader) (map[string]*dto.MetricFamily, error) {
	parser := expfmt.TextParser{}
	metricFamiles, err := parser.TextToMetricFamilies(sourceData)
	if err != nil {
		return nil, err
	}
	return metricFamiles, nil
}

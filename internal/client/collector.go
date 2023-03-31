package client

import (
	"errors"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
	"github.com/umbracle/vesta/internal/client/runner/hooks"
	proto "github.com/umbracle/vesta/internal/client/runner/structs"
)

type collector struct {
	lock    sync.Mutex
	metrics map[string][]*dto.MetricFamily
}

func newCollector() *collector {
	return &collector{lock: sync.Mutex{}, metrics: map[string][]*dto.MetricFamily{}}
}

func (c *collector) hookFactory(logger hclog.Logger, task *proto.Task) hooks.TaskHook {
	return newMetricsHook(logger, task, c)
}

func (c *collector) push(id string, metrics map[string]*dto.MetricFamily) {
	c.lock.Lock()
	defer c.lock.Unlock()

	res := []*dto.MetricFamily{}
	for _, m := range metrics {
		res = append(res, m)
	}
	c.metrics[id] = res
}

func (c *collector) Gather() ([]*dto.MetricFamily, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	res := []*dto.MetricFamily{}
	for _, metrics := range c.metrics {
		for _, mm := range metrics {
			res = append(res, mm)
		}
	}
	return res, nil
}

func (c *collector) UpdateMetrics(id string, metrics map[string]*dto.MetricFamily) {
	c.push(id, metrics)
}

func (c *Client) startCollectorPrometheusServer(listenAddr *net.TCPAddr) *http.Server {
	srv := &http.Server{
		Addr: listenAddr.String(),
		Handler: promhttp.HandlerFor(
			c.collector,
			promhttp.HandlerOpts{},
		),
		ReadHeaderTimeout: 60 * time.Second,
	}

	go func() {
		c.logger.Info("Prometheus server started", "addr=", listenAddr.String())

		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			c.logger.Error("Prometheus HTTP server ListenAndServe", "err", err)
		}
	}()

	return srv
}

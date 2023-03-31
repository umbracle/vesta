package client

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hashicorp/go-hclog"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/umbracle/vesta/internal/client/runner/hooks"
	proto "github.com/umbracle/vesta/internal/client/runner/structs"
)

var _ hooks.TaskHook = &metricsHook{}
var _ hooks.TaskPoststartHook = &metricsHook{}

type MetricsUpdater interface {
	UpdateMetrics(string, map[string]*dto.MetricFamily)
}

type metricsHook struct {
	logger  hclog.Logger
	closeCh chan struct{}
	task    *proto.Task
	updater MetricsUpdater
	ip      string
	url     string
}

func newMetricsHook(logger hclog.Logger, task *proto.Task, updater MetricsUpdater) *metricsHook {
	h := &metricsHook{
		task:    task,
		closeCh: make(chan struct{}),
		updater: updater,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (m *metricsHook) Name() string {
	return "metrics-hook"
}

func (m *metricsHook) Poststart(ctx chan struct{}, req *hooks.TaskPoststartHookRequest) error {
	if req.Spec.Ip == "" {
		return nil
	}

	tel, ok := m.task.Metadata["telemetry"]
	if !ok {
		return nil
	}

	m.url = fmt.Sprintf("http://%s:%s", req.Spec.Ip, tel)

	go m.collectMetrics()
	return nil
}

func (m *metricsHook) collectMetrics() {
	for {
		res, err := http.Get(m.url)
		if err != nil {
			m.logger.Error("failed to query prometheus endpoint", "url", m.url, "err", err)
		} else {
			metrics, err := getMetricFamilies(res.Body)
			if err != nil {
				m.logger.Error("failed to process promtheus format", "err", err)
			} else {
				for _, mf := range metrics {
					for _, metric := range mf.Metric {
						metric.Label = append(metric.Label, &dto.LabelPair{Name: stringPtr("host"), Value: stringPtr(m.task.Name)})
					}
				}
				m.updater.UpdateMetrics(m.task.Tag, metrics)
			}
		}

		select {
		case <-m.closeCh:
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func (m *metricsHook) Stop() {
	close(m.closeCh)
}

func getMetricFamilies(sourceData io.Reader) (map[string]*dto.MetricFamily, error) {
	parser := expfmt.TextParser{}
	metricFamiles, err := parser.TextToMetricFamilies(sourceData)
	if err != nil {
		return nil, err
	}
	return metricFamiles, nil
}

func stringPtr(s string) *string {
	return &s
}

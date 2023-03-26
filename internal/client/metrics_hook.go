package client

import (
	"io"

	"github.com/hashicorp/go-hclog"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	proto "github.com/umbracle/vesta/internal/client/runner/structs"
)

type MetricsUpdater interface {
	UpdateMetrics(string, map[string]*dto.MetricFamily)
}

type metricsHook struct {
	logger  hclog.Logger
	closeCh chan struct{}
	task    *proto.Task
	updater MetricsUpdater
	ip      string
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

func (m *metricsHook) PostStart(handle *proto.TaskHandle) {
	m.ip = handle.Network.Ip
	go m.collectMetrics()
}

func (m *metricsHook) collectMetrics() {
	return

	/*
		url := fmt.Sprintf("http://%s:%d/%s", m.ip, m.task.Telemetry.Port, m.task.Telemetry.Path)

		for {
			res, err := http.Get(url)
			if err != nil {
				m.logger.Error("failed to query prometheus endpoint", "url", url, "err", err)
			} else {
				metrics, err := getMetricFamilies(res.Body)
				if err != nil {
					m.logger.Error("failed to process promtheus format", "err", err)
				} else {
					for _, mf := range metrics {
						for _, metric := range mf.Metric {
							metric.Label = append(metric.Label, &dto.LabelPair{Name: stringPtr("host"), Value: stringPtr(m.task.Id)})
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
	*/
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

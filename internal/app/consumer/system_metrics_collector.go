package consumer

import (
	"context"

	"task-processor/internal/app/monitoring"
)

type systemMetricsCollectorAdapter struct {
	collector *monitoring.MetricsCollector
}

func newSystemMetricsCollectorAdapter(collector *monitoring.MetricsCollector) *systemMetricsCollectorAdapter {
	return &systemMetricsCollectorAdapter{collector: collector}
}

func (a *systemMetricsCollectorAdapter) Start(ctx context.Context) error {
	return a.collector.Start(ctx)
}

func (a *systemMetricsCollectorAdapter) Stop(ctx context.Context) error {
	return a.collector.Stop(ctx)
}

func (a *systemMetricsCollectorAdapter) SetCounter(name string, value float64, labels map[string]string, description string) {
	a.collector.SetCounter(name, value, labels, description)
}

func (a *systemMetricsCollectorAdapter) SetGauge(name string, value float64, labels map[string]string, description string) {
	a.collector.SetGauge(name, value, labels, description)
}

func (a *systemMetricsCollectorAdapter) Snapshot() map[string]float64 {
	metrics := a.collector.GetMetrics()
	result := make(map[string]float64, len(metrics))
	for name, metric := range metrics {
		if metric != nil {
			result[name] = metric.Value
		}
	}
	return result
}

package rabbitmq

import "context"

// SystemMetricsCollector is the queue runtime's narrow metrics dependency.
// Application composition supplies the concrete collector implementation.
type SystemMetricsCollector interface {
	Start(context.Context) error
	Stop(context.Context) error
	SetCounter(name string, value float64, labels map[string]string, description string)
	SetGauge(name string, value float64, labels map[string]string, description string)
	Snapshot() map[string]float64
}

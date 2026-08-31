package consumer

import (
	"context"
	"testing"
	"time"

	"task-processor/internal/app/monitoring"
	"task-processor/internal/platform/queue/rabbitmq"

	"github.com/sirupsen/logrus"
)

func TestSystemMetricsCollectorAdapterPreservesLifecycleAndValues(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	collector := monitoring.NewMetricsCollector(logger, time.Hour)
	adapter := newSystemMetricsCollectorAdapter(collector)
	var _ rabbitmq.SystemMetricsCollector = adapter

	if err := adapter.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if !collector.IsRunning() {
		t.Fatal("underlying collector is not running after adapter Start")
	}
	adapter.SetGauge("system_cpu_cores", 8, nil, "CPU cores")
	adapter.SetCounter("rabbitmq_tasks_processed_total", 12, nil, "processed")
	got := adapter.Snapshot()
	if got["system_cpu_cores"] != 8 || got["rabbitmq_tasks_processed_total"] != 12 {
		t.Fatalf("snapshot = %#v, want converted metric values", got)
	}
	if err := adapter.Stop(context.Background()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if collector.IsRunning() {
		t.Fatal("underlying collector is still running after adapter Stop")
	}
}

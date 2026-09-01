package rabbitmq

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

type fakeSystemMetricsCollector struct {
	mu      sync.Mutex
	started bool
	stopped bool
	values  map[string]float64
}

func newFakeSystemMetricsCollector() *fakeSystemMetricsCollector {
	return &fakeSystemMetricsCollector{values: make(map[string]float64)}
}

func (c *fakeSystemMetricsCollector) Start(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.started = true
	return nil
}

func (c *fakeSystemMetricsCollector) Stop(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stopped = true
	return nil
}

func (c *fakeSystemMetricsCollector) SetCounter(name string, value float64, _ map[string]string, _ string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values[name] = value
}

func (c *fakeSystemMetricsCollector) SetGauge(name string, value float64, _ map[string]string, _ string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values[name] = value
}

func (c *fakeSystemMetricsCollector) Snapshot() map[string]float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make(map[string]float64, len(c.values))
	for name, value := range c.values {
		result[name] = value
	}
	return result
}

func TestLoadMonitorDelegatesCollectorLifecycleAndSnapshot(t *testing.T) {
	collector := newFakeSystemMetricsCollector()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	lm := NewLoadMonitor(LoadMonitorConfig{UpdateInterval: time.Hour}, collector, logger)

	if err := lm.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	lm.RecordTaskProcessed("orders", true, 25*time.Millisecond)
	lm.updateStats()
	if err := lm.Stop(context.Background()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	collector.mu.Lock()
	started, stopped := collector.started, collector.stopped
	collector.mu.Unlock()
	if !started || !stopped {
		t.Fatalf("collector lifecycle = started:%t stopped:%t, want both true", started, stopped)
	}
	got := lm.SystemMetricsSnapshot()
	if got["rabbitmq_tasks_processed_total"] != 1 {
		t.Fatalf("rabbitmq_tasks_processed_total = %v, want 1; snapshot=%#v", got["rabbitmq_tasks_processed_total"], got)
	}
}

func TestLoadMonitorNilCollectorDisablesOnlySystemMetrics(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	lm := NewLoadMonitor(LoadMonitorConfig{UpdateInterval: time.Hour}, nil, logger)

	if err := lm.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	lm.RecordTaskProcessed("orders", false, 25*time.Millisecond)
	if err := lm.Stop(context.Background()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	stats := lm.GetStats()
	if stats.TasksProcessed != 1 || stats.TasksFailed != 1 {
		t.Fatalf("load stats = %#v, want one failed task", stats)
	}
	if got := lm.SystemMetricsSnapshot(); len(got) != 0 {
		t.Fatalf("system metrics = %#v, want empty", got)
	}
}

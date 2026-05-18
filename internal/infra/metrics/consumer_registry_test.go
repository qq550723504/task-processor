package metrics

import (
	"strings"
	"testing"
	"time"

	coremetrics "task-processor/internal/core/metrics"
	"task-processor/internal/infra/rabbitmq"

	promtest "github.com/prometheus/client_golang/prometheus/testutil"
)

func TestConsumerRegistryExportsSnapshotMetrics(t *testing.T) {
	registry := NewConsumerRegistry()
	registry.UpdateConsumerSnapshot(ConsumerSnapshot{
		Load: rabbitmq.LoadStats{
			TasksProcessed: 12,
			TasksSucceeded: 9,
			TasksFailed:    3,
		},
		Task: coremetrics.TaskMetricsSnapshot{
			ProcessingCount:     4,
			CompletedCount:      9,
			FailedCount:         3,
			RequeuedCount:       2,
			HighPriorityCount:   5,
			MediumPriorityCount: 4,
			LowPriorityCount:    3,
			TotalWaitTime:       8 * time.Second,
			TotalProcessTime:    18 * time.Second,
			TaskCount:           4,
		},
		Shein: coremetrics.SheinMetricsSnapshot{
			PublishedCount: 7,
			TopSuccessStores: []coremetrics.SheinStoreStatsSnapshot{
				{TenantID: 11, StoreID: 22, PublishedCount: 5},
			},
		},
		System: map[string]float64{
			"system_goroutines_count":              21,
			"rabbitmq_avg_processing_time_seconds": 2.5,
		},
	})

	expected := `
# HELP tasks_processed_total Total number of tasks processed
# TYPE tasks_processed_total counter
tasks_processed_total 12
# HELP listing_tasks_wait_seconds_avg Average wait time for listing tasks in seconds
# TYPE listing_tasks_wait_seconds_avg gauge
listing_tasks_wait_seconds_avg 2
# HELP shein_tasks_published_total Total number of SHEIN tasks published successfully
# TYPE shein_tasks_published_total counter
shein_tasks_published_total 7
# HELP shein_top_success_store_published_total Top SHEIN success stores by published tasks
# TYPE shein_top_success_store_published_total gauge
shein_top_success_store_published_total{rank="1",store_id="22",tenant_id="11"} 5
# HELP goroutine_count Current number of goroutines
# TYPE goroutine_count gauge
goroutine_count 21
`

	if err := promtest.GatherAndCompare(registry.Registry(), strings.NewReader(expected),
		"tasks_processed_total",
		"listing_tasks_wait_seconds_avg",
		"shein_tasks_published_total",
		"shein_top_success_store_published_total",
		"goroutine_count",
	); err != nil {
		t.Fatalf("GatherAndCompare() error = %v", err)
	}
}

func TestConsumerRegistryIncludesPrometheusBuiltInCollectors(t *testing.T) {
	registry := NewConsumerRegistry()

	families, err := registry.Registry().Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}

	names := make(map[string]int, len(families))
	for _, family := range families {
		names[family.GetName()] = len(family.GetMetric())
	}

	if count := names["go_goroutines"]; count == 0 {
		t.Fatalf("go_goroutines collector missing or empty: %+v", names)
	}
	if count := names["process_start_time_seconds"]; count == 0 {
		t.Fatalf("process_start_time_seconds collector missing or empty: %+v", names)
	}
}

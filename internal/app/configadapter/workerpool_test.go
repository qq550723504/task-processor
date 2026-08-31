package configadapter

import (
	"testing"
	"time"

	coreconfig "task-processor/internal/core/config"
)

func TestWorkerPoolConfigUsesLegacyConcurrencyAndBuffer(t *testing.T) {
	t.Parallel()

	got := WorkerPool(coreconfig.WorkerConfig{Concurrency: 9, BufferSize: 40})
	if got.Concurrency != 9 || got.BufferSize != 40 || got.TaskTimeout != 15*time.Minute || !got.EnableMetrics || got.ShutdownTimeout != 30*time.Second {
		t.Fatalf("pool config = %#v", got)
	}
}

package pipeline

import (
	"testing"
	"time"

	coreconfig "task-processor/internal/core/config"
)

func TestPlatformWorkerPoolConfigPreservesLegacyDefaultsAndOverrides(t *testing.T) {
	t.Parallel()

	got := platformWorkerPoolConfig(coreconfig.WorkerConfig{Concurrency: 11, BufferSize: 47})
	if got.Concurrency != 11 || got.BufferSize != 47 || got.TaskTimeout != 15*time.Minute || !got.EnableMetrics || got.ShutdownTimeout != 30*time.Second {
		t.Fatalf("pool config = %#v", got)
	}
}

package temu

import (
	"testing"
	"time"

	coreconfig "task-processor/internal/core/config"
)

func TestPlatformWorkerPoolConfigPreservesLegacyDefaultsAndOverrides(t *testing.T) {
	t.Parallel()

	got := platformWorkerPoolConfig(coreconfig.WorkerConfig{Concurrency: 7, BufferSize: 31})
	if got.Concurrency != 7 || got.BufferSize != 31 || got.TaskTimeout != 15*time.Minute || !got.EnableMetrics || got.ShutdownTimeout != 30*time.Second {
		t.Fatalf("pool config = %#v", got)
	}
}

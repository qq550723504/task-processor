package configadapter

import (
	"reflect"
	"testing"
	"time"

	coreconfig "task-processor/internal/core/config"
	"task-processor/internal/platform/queue/rabbitmq"
)

func TestLoadMonitorConfigPreservesAllFields(t *testing.T) {
	t.Parallel()

	in := coreconfig.LoadMonitorConfig{
		UpdateInterval: 7 * time.Second,
		EnableCPU:      true,
		EnableMemory:   true,
		EnableTasks:    false,
	}
	want := rabbitmq.LoadMonitorConfig{
		UpdateInterval: 7 * time.Second,
		EnableCPU:      true,
		EnableMemory:   true,
		EnableTasks:    false,
	}
	if got := LoadMonitor(in); !reflect.DeepEqual(got, want) {
		t.Fatalf("config = %#v, want %#v", got, want)
	}
}

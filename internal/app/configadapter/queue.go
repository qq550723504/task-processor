package configadapter

import (
	coreconfig "task-processor/internal/core/config"
	"task-processor/internal/platform/queue/rabbitmq"
)

// LoadMonitor translates the application schema into queue runtime configuration.
func LoadMonitor(cfg coreconfig.LoadMonitorConfig) rabbitmq.LoadMonitorConfig {
	return rabbitmq.LoadMonitorConfig{
		UpdateInterval: cfg.UpdateInterval,
		EnableCPU:      cfg.EnableCPU,
		EnableMemory:   cfg.EnableMemory,
		EnableTasks:    cfg.EnableTasks,
	}
}

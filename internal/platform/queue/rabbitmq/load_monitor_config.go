package rabbitmq

import "time"

// LoadMonitorConfig configures RabbitMQ load and system-metric sampling.
type LoadMonitorConfig struct {
	UpdateInterval time.Duration
	EnableCPU      bool
	EnableMemory   bool
	EnableTasks    bool
}

// MonitorConfig preserves the existing internal alias without depending on application config.
type MonitorConfig = LoadMonitorConfig

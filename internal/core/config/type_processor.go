package config

// ProcessorConfig 处理器配置
type ProcessorConfig struct {
	MaxRetries       int  `yaml:"maxRetries"`       // 最大重试次数
	Timeout          int  `yaml:"timeout"`          // 超时时间（秒）
	SchedulerEnabled bool `yaml:"schedulerEnabled"` // 是否在 worker 进程内启动调度服务
}

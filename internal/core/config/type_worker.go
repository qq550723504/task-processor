package config

// WorkerConfig 工作池配置
type WorkerConfig struct {
	Concurrency        int `yaml:"concurrency"`        // 并发工作协程数
	BufferSize         int `yaml:"bufferSize"`         // 队列缓冲区大小
	TaskInterval       int `yaml:"taskInterval"`       // 任务获取间隔（秒）
	MaxFetchPerCycle   int `yaml:"maxFetchPerCycle"`   // 单次最多获取任务数
	QueueThreshold     int `yaml:"queueThreshold"`     // 队列使用率阈值（%）
	CleanupInterval    int `yaml:"cleanupInterval"`    // 清理间隔（秒）
	TaskTimeout        int `yaml:"taskTimeout"`        // 任务超时时间（秒）
	StuckTaskThreshold int `yaml:"stuckTaskThreshold"` // 任务卡住阈值（秒）
	ForceCleanupAfter  int `yaml:"forceCleanupAfter"`  // 30分钟强制清理阈值（秒）
}

package config

// ProcessorConfig 处理器配置
type ProcessorConfig struct {
	MaxRetries int `yaml:"maxRetries"` // 最大重试次数
	Timeout    int `yaml:"timeout"`    // 超时时间（秒）
}

package updater

import "time"

// Config 更新器配置
type Config struct {
	Enabled        bool          `yaml:"enabled"`        // 是否启用自动更新
	UpdateURL      string        `yaml:"update_url"`     // 版本检查地址
	CheckInterval  time.Duration `yaml:"check_interval"` // 检查间隔（秒）
	CurrentVersion string        `yaml:"-"`              // 当前版本（从编译时注入）
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Enabled:       true,
		UpdateURL:     "https://auto-update-1303159911.cos.ap-shanghai.myqcloud.com/task-processor/version.json",
		CheckInterval: 5 * time.Minute, // 每5分钟检查一次
	}
}

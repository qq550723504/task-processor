// Package types 提供配置类型定义
package types

// UpdaterConfig 自动更新配置
type UpdaterConfig struct {
	Enabled            bool   `yaml:"enabled"`              // 是否启用自动更新
	UpdateURL          string `yaml:"update_url"`           // 版本检查地址
	CheckInterval      int    `yaml:"check_interval"`       // 检查间隔（秒）
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"` // 跳过TLS证书验证
	CurrentVersion     string `yaml:"-"`                    // 当前版本（从编译时注入）
}

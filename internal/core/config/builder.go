// Package config 提供配置管理功能
package config

import (
	"task-processor/internal/core/config/loaders"
)

// buildConfig 构建配置对象
func buildConfig() *Config {
	typesConfig := loaders.BuildConfig()
	return &Config{Config: typesConfig}
}

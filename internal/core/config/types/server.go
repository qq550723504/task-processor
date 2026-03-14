// Package types 提供配置类型定义
package types

import "time"

// ServerConfig HTTP 服务器配置
type ServerConfig struct {
	Port         int           `mapstructure:"port"`
	Timeout      time.Duration `mapstructure:"timeout"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

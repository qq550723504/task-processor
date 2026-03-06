// Package config 提供通用配置类型
package config

import "time"

// RetryConfig 通用重试配置
// 用于替代分散在多个包中的重复定义
type RetryConfig struct {
	MaxRetries    int           `json:"max_retries" yaml:"max_retries"`       // 最大重试次数
	InitialDelay  time.Duration `json:"initial_delay" yaml:"initial_delay"`   // 初始延迟
	MaxDelay      time.Duration `json:"max_delay" yaml:"max_delay"`           // 最大延迟
	BackoffFactor float64       `json:"backoff_factor" yaml:"backoff_factor"` // 退避因子
}

// DefaultRetryConfig 返回默认重试配置
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:    3,
		InitialDelay:  1 * time.Second,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
	}
}

// TimeoutConfig 通用超时配置
type TimeoutConfig struct {
	Connect time.Duration `json:"connect" yaml:"connect"` // 连接超时
	Read    time.Duration `json:"read" yaml:"read"`       // 读取超时
	Write   time.Duration `json:"write" yaml:"write"`     // 写入超时
}

// DefaultTimeoutConfig 返回默认超时配置
func DefaultTimeoutConfig() *TimeoutConfig {
	return &TimeoutConfig{
		Connect: 10 * time.Second,
		Read:    30 * time.Second,
		Write:   30 * time.Second,
	}
}

// ConnectionConfig 通用连接配置
type ConnectionConfig struct {
	URL               string        `json:"url" yaml:"url"`                               // 连接URL
	MaxRetries        int           `json:"max_retries" yaml:"max_retries"`               // 最大重试次数
	ReconnectInterval time.Duration `json:"reconnect_interval" yaml:"reconnect_interval"` // 重连间隔
	Timeout           time.Duration `json:"timeout" yaml:"timeout"`                       // 超时时间
}

// DefaultConnectionConfig 返回默认连接配置
func DefaultConnectionConfig() *ConnectionConfig {
	return &ConnectionConfig{
		MaxRetries:        3,
		ReconnectInterval: 5 * time.Second,
		Timeout:           30 * time.Second,
	}
}

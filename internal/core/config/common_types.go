// Package config 提供通用配置类型
package config

import (
	"task-processor/internal/core/config/types"
	"time"
)

// RetryConfig 通用重试配置 (类型别名,使用types包中的定义)
type RetryConfig = types.RetryConfig

// DefaultRetryConfig 返回默认重试配置
func DefaultRetryConfig() *RetryConfig {
	return types.DefaultRetryConfig()
}

// TimeoutConfig 通用超时配置
type TimeoutConfig struct {
	ConnectTimeout time.Duration `yaml:"connectTimeout" json:"connectTimeout"` // 连接超时
	ReadTimeout    time.Duration `yaml:"readTimeout" json:"readTimeout"`       // 读取超时
	WriteTimeout   time.Duration `yaml:"writeTimeout" json:"writeTimeout"`     // 写入超时
	IdleTimeout    time.Duration `yaml:"idleTimeout" json:"idleTimeout"`       // 空闲超时
}

// DefaultTimeoutConfig 返回默认超时配置
func DefaultTimeoutConfig() *TimeoutConfig {
	return &TimeoutConfig{
		ConnectTimeout: 10 * time.Second,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    90 * time.Second,
	}
}

// HTTPClientConfig 通用HTTP客户端配置
type HTTPClientConfig struct {
	BaseURL         string            `yaml:"baseURL" json:"baseURL"`                 // 基础URL
	Timeout         time.Duration     `yaml:"timeout" json:"timeout"`                 // 请求超时
	MaxRetries      int               `yaml:"maxRetries" json:"maxRetries"`           // 最大重试次数
	RetryDelay      time.Duration     `yaml:"retryDelay" json:"retryDelay"`           // 重试延迟
	MaxIdleConns    int               `yaml:"maxIdleConns" json:"maxIdleConns"`       // 最大空闲连接数
	MaxConnsPerHost int               `yaml:"maxConnsPerHost" json:"maxConnsPerHost"` // 每个主机最大连接数
	Headers         map[string]string `yaml:"headers" json:"headers"`                 // 自定义请求头
}

// DefaultHTTPClientConfig 返回默认HTTP客户端配置
func DefaultHTTPClientConfig() *HTTPClientConfig {
	return &HTTPClientConfig{
		Timeout:         30 * time.Second,
		MaxRetries:      3,
		RetryDelay:      1 * time.Second,
		MaxIdleConns:    100,
		MaxConnsPerHost: 10,
		Headers:         make(map[string]string),
	}
}

// CacheConfig 通用缓存配置
type CacheConfig struct {
	Enabled         bool          `yaml:"enabled" json:"enabled"`                 // 是否启用缓存
	TTL             time.Duration `yaml:"ttl" json:"ttl"`                         // 缓存过期时间
	MaxSize         int           `yaml:"maxSize" json:"maxSize"`                 // 最大缓存条目数
	CleanupInterval time.Duration `yaml:"cleanupInterval" json:"cleanupInterval"` // 清理间隔
}

// DefaultCacheConfig 返回默认缓存配置
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		Enabled:         true,
		TTL:             5 * time.Minute,
		MaxSize:         1000,
		CleanupInterval: 10 * time.Minute,
	}
}

// RateLimitConfig 通用限流配置
type RateLimitConfig struct {
	Enabled           bool    `yaml:"enabled" json:"enabled"`                     // 是否启用限流
	RequestsPerSecond float64 `yaml:"requestsPerSecond" json:"requestsPerSecond"` // 每秒请求数
	BurstSize         int     `yaml:"burstSize" json:"burstSize"`                 // 突发大小
}

// DefaultRateLimitConfig 返回默认限流配置
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		Enabled:           true,
		RequestsPerSecond: 10.0,
		BurstSize:         20,
	}
}

// LogConfig 通用日志配置
type LogConfig struct {
	Level      string `yaml:"level" json:"level"`           // 日志级别: DEBUG, INFO, WARN, ERROR
	Format     string `yaml:"format" json:"format"`         // 日志格式: text, json
	Output     string `yaml:"output" json:"output"`         // 输出目标: stdout, file
	FilePath   string `yaml:"filePath" json:"filePath"`     // 日志文件路径
	MaxSize    int    `yaml:"maxSize" json:"maxSize"`       // 单个日志文件最大大小(MB)
	MaxBackups int    `yaml:"maxBackups" json:"maxBackups"` // 保留的旧日志文件数量
	MaxAge     int    `yaml:"maxAge" json:"maxAge"`         // 保留日志文件的最大天数
	Compress   bool   `yaml:"compress" json:"compress"`     // 是否压缩旧日志文件
}

// DefaultLogConfig 返回默认日志配置
func DefaultLogConfig() *LogConfig {
	return &LogConfig{
		Level:      "INFO",
		Format:     "text",
		Output:     "stdout",
		FilePath:   "logs/app.log",
		MaxSize:    100,
		MaxBackups: 3,
		MaxAge:     7,
		Compress:   true,
	}
}

// MonitoringConfig 通用监控配置
// 注意: 此配置预留用于未来的监控功能集成
// 可用于集成Prometheus、Grafana等监控系统
type MonitoringConfig struct {
	Enabled             bool          `yaml:"enabled" json:"enabled"`                         // 是否启用监控
	MetricsEnabled      bool          `yaml:"metricsEnabled" json:"metricsEnabled"`           // 是否启用指标收集
	TracingEnabled      bool          `yaml:"tracingEnabled" json:"tracingEnabled"`           // 是否启用链路追踪
	HealthCheckInterval time.Duration `yaml:"healthCheckInterval" json:"healthCheckInterval"` // 健康检查间隔
	MetricsPort         int           `yaml:"metricsPort" json:"metricsPort"`                 // 指标端口
}

// DefaultMonitoringConfig 返回默认监控配置
func DefaultMonitoringConfig() *MonitoringConfig {
	return &MonitoringConfig{
		Enabled:             true,
		MetricsEnabled:      true,
		TracingEnabled:      false,
		HealthCheckInterval: 30 * time.Second,
		MetricsPort:         9090,
	}
}

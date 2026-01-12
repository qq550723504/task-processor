package client

import (
	"time"
)

// Config TEMU API配置
type Config struct {
	BaseURL        string
	RequestTimeout time.Duration
	RetryCount     int
	MaxTimeout     time.Duration // 最大超时时间
	RetryInterval  time.Duration // 重试间隔
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		BaseURL:        "https://seller.temu.com",
		RequestTimeout: 30 * time.Second,  // 单次请求超时
		MaxTimeout:     120 * time.Second, // 最大超时时间（包含重试）
		RetryCount:     3,
		RetryInterval:  2 * time.Second, // 重试间隔
	}
}

// NewConfigFromSettings 从配置文件创建配置
func NewConfigFromSettings(timeout, maxTimeout, retryInterval int, retryCount int) *Config {
	config := DefaultConfig()

	if timeout > 0 {
		config.RequestTimeout = time.Duration(timeout) * time.Second
	}
	if maxTimeout > 0 {
		config.MaxTimeout = time.Duration(maxTimeout) * time.Second
	}
	if retryInterval > 0 {
		config.RetryInterval = time.Duration(retryInterval) * time.Second
	}
	if retryCount > 0 {
		config.RetryCount = retryCount
	}

	return config
}

// GetDefaultHeaders 获取默认请求头
func GetDefaultHeaders() map[string]string {
	return map[string]string{
		"accept":             "application/json, text/plain, */*",
		"accept-language":    "zh-CN,zh;q=0.9",
		"priority":           "u=1, i",
		"sec-ch-ua":          "\"Chromium\";v=\"140\", \"Not=A?Brand\";v=\"24\", \"Google Chrome\";v=\"140\"",
		"sec-ch-ua-mobile":   "?0",
		"sec-ch-ua-platform": "\"Windows\"",
		"sec-fetch-dest":     "empty",
		"sec-fetch-mode":     "cors",
		"sec-fetch-site":     "same-origin",
		"user-agent":         "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36",
	}
}

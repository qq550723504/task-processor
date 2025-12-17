package client

import (
	"time"
)

// Config TEMU API配置
type Config struct {
	BaseURL        string
	RequestTimeout time.Duration
	RetryCount     int
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		BaseURL:        "https://seller.temu.com",
		RequestTimeout: 30 * time.Second,
		RetryCount:     3,
	}
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

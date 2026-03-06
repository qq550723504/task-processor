package config

import (
	"testing"
	"time"
)

// TestDefaultRetryConfig 测试默认重试配置
func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	if config == nil {
		t.Fatal("DefaultRetryConfig 返回 nil")
	}

	// 验证默认值
	if config.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, 期望 3", config.MaxRetries)
	}

	if config.InitialDelay != 1*time.Second {
		t.Errorf("InitialDelay = %v, 期望 1s", config.InitialDelay)
	}

	if config.MaxDelay != 30*time.Second {
		t.Errorf("MaxDelay = %v, 期望 30s", config.MaxDelay)
	}

	if config.BackoffFactor != 2.0 {
		t.Errorf("BackoffFactor = %v, 期望 2.0", config.BackoffFactor)
	}
}

// TestRetryConfigCustom 测试自定义重试配置
func TestRetryConfigCustom(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:    5,
		InitialDelay:  2 * time.Second,
		MaxDelay:      60 * time.Second,
		BackoffFactor: 1.5,
	}

	if config.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, 期望 5", config.MaxRetries)
	}

	if config.InitialDelay != 2*time.Second {
		t.Errorf("InitialDelay = %v, 期望 2s", config.InitialDelay)
	}

	if config.MaxDelay != 60*time.Second {
		t.Errorf("MaxDelay = %v, 期望 60s", config.MaxDelay)
	}

	if config.BackoffFactor != 1.5 {
		t.Errorf("BackoffFactor = %v, 期望 1.5", config.BackoffFactor)
	}
}

// TestDefaultTimeoutConfig 测试默认超时配置
func TestDefaultTimeoutConfig(t *testing.T) {
	config := DefaultTimeoutConfig()

	if config == nil {
		t.Fatal("DefaultTimeoutConfig 返回 nil")
	}

	// 验证默认值
	if config.Connect != 10*time.Second {
		t.Errorf("Connect = %v, 期望 10s", config.Connect)
	}

	if config.Read != 30*time.Second {
		t.Errorf("Read = %v, 期望 30s", config.Read)
	}

	if config.Write != 30*time.Second {
		t.Errorf("Write = %v, 期望 30s", config.Write)
	}
}

// TestTimeoutConfigCustom 测试自定义超时配置
func TestTimeoutConfigCustom(t *testing.T) {
	config := &TimeoutConfig{
		Connect: 5 * time.Second,
		Read:    15 * time.Second,
		Write:   20 * time.Second,
	}

	if config.Connect != 5*time.Second {
		t.Errorf("Connect = %v, 期望 5s", config.Connect)
	}

	if config.Read != 15*time.Second {
		t.Errorf("Read = %v, 期望 15s", config.Read)
	}

	if config.Write != 20*time.Second {
		t.Errorf("Write = %v, 期望 20s", config.Write)
	}
}

// TestDefaultConnectionConfig 测试默认连接配置
func TestDefaultConnectionConfig(t *testing.T) {
	config := DefaultConnectionConfig()

	if config == nil {
		t.Fatal("DefaultConnectionConfig 返回 nil")
	}

	// 验证默认值
	if config.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, 期望 3", config.MaxRetries)
	}

	if config.ReconnectInterval != 5*time.Second {
		t.Errorf("ReconnectInterval = %v, 期望 5s", config.ReconnectInterval)
	}

	if config.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, 期望 30s", config.Timeout)
	}

	// URL应该为空字符串
	if config.URL != "" {
		t.Errorf("URL = %q, 期望空字符串", config.URL)
	}
}

// TestConnectionConfigCustom 测试自定义连接配置
func TestConnectionConfigCustom(t *testing.T) {
	config := &ConnectionConfig{
		URL:               "amqp://localhost:5672",
		MaxRetries:        5,
		ReconnectInterval: 10 * time.Second,
		Timeout:           60 * time.Second,
	}

	if config.URL != "amqp://localhost:5672" {
		t.Errorf("URL = %q, 期望 'amqp://localhost:5672'", config.URL)
	}

	if config.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, 期望 5", config.MaxRetries)
	}

	if config.ReconnectInterval != 10*time.Second {
		t.Errorf("ReconnectInterval = %v, 期望 10s", config.ReconnectInterval)
	}

	if config.Timeout != 60*time.Second {
		t.Errorf("Timeout = %v, 期望 60s", config.Timeout)
	}
}

// TestRetryConfigZeroValues 测试零值配置
func TestRetryConfigZeroValues(t *testing.T) {
	config := &RetryConfig{}

	// 零值应该是有效的
	if config.MaxRetries != 0 {
		t.Errorf("MaxRetries = %d, 期望 0", config.MaxRetries)
	}

	if config.InitialDelay != 0 {
		t.Errorf("InitialDelay = %v, 期望 0", config.InitialDelay)
	}

	if config.MaxDelay != 0 {
		t.Errorf("MaxDelay = %v, 期望 0", config.MaxDelay)
	}

	if config.BackoffFactor != 0 {
		t.Errorf("BackoffFactor = %v, 期望 0", config.BackoffFactor)
	}
}

// TestTimeoutConfigZeroValues 测试零值配置
func TestTimeoutConfigZeroValues(t *testing.T) {
	config := &TimeoutConfig{}

	// 零值应该是有效的
	if config.Connect != 0 {
		t.Errorf("Connect = %v, 期望 0", config.Connect)
	}

	if config.Read != 0 {
		t.Errorf("Read = %v, 期望 0", config.Read)
	}

	if config.Write != 0 {
		t.Errorf("Write = %v, 期望 0", config.Write)
	}
}

// TestConnectionConfigZeroValues 测试零值配置
func TestConnectionConfigZeroValues(t *testing.T) {
	config := &ConnectionConfig{}

	// 零值应该是有效的
	if config.URL != "" {
		t.Errorf("URL = %q, 期望空字符串", config.URL)
	}

	if config.MaxRetries != 0 {
		t.Errorf("MaxRetries = %d, 期望 0", config.MaxRetries)
	}

	if config.ReconnectInterval != 0 {
		t.Errorf("ReconnectInterval = %v, 期望 0", config.ReconnectInterval)
	}

	if config.Timeout != 0 {
		t.Errorf("Timeout = %v, 期望 0", config.Timeout)
	}
}

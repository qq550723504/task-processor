package temu

import (
	"testing"
	"time"
)

func TestAPIClient_initHTTPClient(t *testing.T) {
	config := DefaultConfig()
	client := &APIClient{
		config: config,
		logger: nil, // 测试时可以为nil
	}

	// 测试初始化HTTP客户端
	client.initHTTPClient()

	if client.client == nil {
		t.Error("HTTP客户端初始化失败")
	}

	// 验证TLS配置
	tlsConfig := client.getTLSConfig()
	if tlsConfig == nil {
		t.Error("TLS配置获取失败")
	}

	if tlsConfig.MinVersion != 0x0303 { // TLS 1.2
		t.Error("TLS最小版本配置错误")
	}

	if tlsConfig.MaxVersion != 0x0304 { // TLS 1.3
		t.Error("TLS最大版本配置错误")
	}

	if len(tlsConfig.CipherSuites) == 0 {
		t.Error("密码套件配置为空")
	}
}

func TestAPIClient_getDefaultHeaders(t *testing.T) {
	client := &APIClient{}
	headers := client.getDefaultHeaders()

	expectedHeaders := []string{
		"Accept",
		"Accept-Encoding",
		"Accept-Language",
		"Cache-Control",
		"Pragma",
		"Priority",
		"Sec-Ch-Ua",
		"Sec-Ch-Ua-Mobile",
		"Sec-Ch-Ua-Platform",
		"Sec-Fetch-Dest",
		"Sec-Fetch-Mode",
		"Sec-Fetch-Site",
		"Upgrade-Insecure-Requests",
	}

	for _, header := range expectedHeaders {
		if _, exists := headers[header]; !exists {
			t.Errorf("缺少默认请求头: %s", header)
		}
	}
}

func TestAPIClient_validateRequest(t *testing.T) {
	client := &APIClient{}

	// 测试有效请求
	err := client.validateRequest("POST", "/api/test")
	if err != nil {
		t.Errorf("有效请求验证失败: %v", err)
	}

	// 测试空方法
	err = client.validateRequest("", "/api/test")
	if err == nil {
		t.Error("空方法应该验证失败")
	}

	// 测试空URL
	err = client.validateRequest("POST", "")
	if err == nil {
		t.Error("空URL应该验证失败")
	}
}

func TestConfig_DefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.BaseURL != "https://seller.temu.com" {
		t.Errorf("默认BaseURL错误: %s", config.BaseURL)
	}

	if config.RequestTimeout != 30*time.Second {
		t.Errorf("默认超时时间错误: %v", config.RequestTimeout)
	}

	if config.RetryCount <= 0 {
		t.Errorf("默认重试次数错误: %d", config.RetryCount)
	}
}

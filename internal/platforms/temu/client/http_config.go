// Package client 提供TEMU平台的HTTP客户端配置功能
package client

import (
	"crypto/tls"
	"time"

	"github.com/imroc/req/v3"
)

// HTTPConfigManager HTTP配置管理器
type HTTPConfigManager struct {
	config   *Config
	proxyURL string
}

// NewHTTPConfigManager 创建HTTP配置管理器
func NewHTTPConfigManager(config *Config, proxyURL string) *HTTPConfigManager {
	return &HTTPConfigManager{
		config:   config,
		proxyURL: proxyURL,
	}
}

// InitHTTPClient 初始化HTTP客户端 - 参考TEMU项目实现
func (h *HTTPConfigManager) InitHTTPClient() *req.Client {
	client := req.C().
		SetTLSFingerprintChrome().
		EnableAutoDecompress().
		SetTLSClientConfig(h.getTLSConfig()).
		SetCommonHeaders(h.getDefaultHeaders()).
		SetCommonRetryCount(3).
		SetCommonRetryInterval(func(resp *req.Response, attempt int) time.Duration {
			// 动态退避策略
			baseDelay := time.Duration(attempt*attempt) * time.Second
			return baseDelay
		}).
		SetCommonRetryCondition(func(resp *req.Response, err error) bool {
			// 网络错误重试
			if err != nil {
				return true
			}
			// HTTP错误重试
			if resp != nil && (resp.StatusCode >= 500 || resp.StatusCode == 429) {
				return true
			}
			return false
		}).
		SetTimeout(h.config.RequestTimeout)

	// 如果配置了代理，则设置代理
	if h.proxyURL != "" {
		client = client.SetProxyURL(h.proxyURL)
	}

	return client
}

// getTLSConfig 获取TLS配置 - 参考TEMU项目
func (h *HTTPConfigManager) getTLSConfig() *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS12,
		MaxVersion:         tls.VersionTLS13,
		CipherSuites: []uint16{
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		},
	}
}

// getDefaultHeaders 获取默认请求头 - 参考TEMU项目
func (h *HTTPConfigManager) getDefaultHeaders() map[string]string {
	return map[string]string{
		"Accept":                    "application/json, text/plain, */*",
		"Accept-Encoding":           "gzip, deflate, br",
		"Accept-Language":           "zh-CN,zh;q=0.9,en-US;q=0.8,en;q=0.7",
		"Cache-Control":             "no-cache",
		"Pragma":                    "no-cache",
		"Priority":                  "u=1, i",
		"Sec-Ch-Ua":                 `"Not A;Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`,
		"Sec-Ch-Ua-Mobile":          "?0",
		"Sec-Ch-Ua-Platform":        `"Windows"`,
		"Sec-Fetch-Dest":            "empty",
		"Sec-Fetch-Mode":            "cors",
		"Sec-Fetch-Site":            "same-origin",
		"Upgrade-Insecure-Requests": "1",
	}
}

// Package client 提供TEMU平台HTTP管理功能
package client

import (
	"crypto/tls"
	"fmt"
	"time"

	"github.com/imroc/req/v3"
	"github.com/sirupsen/logrus"
)

// HTTPManager HTTP管理器
type HTTPManager struct {
	proxyURL string
	logger   *logrus.Entry
	config   *Config // 添加配置引用
}

// NewHTTPManager 创建新的HTTP管理器
func NewHTTPManager(proxyURL string, logger *logrus.Entry) *HTTPManager {
	return &HTTPManager{
		proxyURL: proxyURL,
		logger:   logger,
		config:   DefaultConfig(), // 使用默认配置
	}
}

// CreateClient 创建HTTP客户端
func (h *HTTPManager) CreateClient() *req.Client {
	client := req.C().
		SetTLSFingerprintChrome().
		EnableAutoDecompress().
		SetTLSClientConfig(h.getTLSConfig()).
		SetCommonHeaders(h.getDefaultHeaders()).
		SetCommonRetryCount(h.config.RetryCount).
		SetCommonRetryInterval(func(resp *req.Response, attempt int) time.Duration {
			// 使用配置的重试间隔，并应用指数退避
			baseDelay := h.config.RetryInterval * time.Duration(attempt)
			h.logger.Infof("重试第%d次，等待%v", attempt, baseDelay)
			return baseDelay
		}).
		SetCommonRetryCondition(func(resp *req.Response, err error) bool {
			// 网络错误重试
			if err != nil {
				h.logger.WithError(err).Warn("网络错误，准备重试")
				return true
			}
			// HTTP错误重试
			if resp != nil && (resp.StatusCode >= 500 || resp.StatusCode == 429) {
				h.logger.Warnf("HTTP错误 %d，准备重试", resp.StatusCode)
				return true
			}
			return false
		}).
		SetTimeout(h.config.MaxTimeout) // 使用配置文件中的超时设置

	// 如果配置了代理，则设置代理
	if h.proxyURL != "" {
		h.logger.Infof("使用代理地址: %s", h.proxyURL)
		client = client.SetProxyURL(h.proxyURL)
	}

	return client
}

// SendRequest 发送HTTP请求
func (h *HTTPManager) SendRequest(client *req.Client, method, url string, headers map[string]string, body any, formFields map[string]string, fileFields map[string]any) (*req.Response, error) {
	// 验证请求参数
	if err := h.validateRequest(method, url); err != nil {
		return nil, fmt.Errorf("请求参数验证失败: %w", err)
	}

	// 创建动态请求
	r := client.R()

	// 设置请求头
	for key, value := range headers {
		r.SetHeader(key, value)
	}

	// 如果有文件字段，则使用multipart/form-data格式
	if len(fileFields) > 0 {
		return h.sendMultipartRequest(r, method, url, formFields, fileFields)
	} else {
		return h.sendJSONRequest(r, method, url, headers, body)
	}
}

// sendMultipartRequest 发送multipart请求
func (h *HTTPManager) sendMultipartRequest(r *req.Request, method, url string, formFields map[string]string, fileFields map[string]any) (*req.Response, error) {
	// 添加表单字段
	if len(formFields) > 0 {
		r.SetFormData(formFields)
	}

	// 添加文件字段
	for fieldName, fileData := range fileFields {
		if fileInfo, ok := fileData.(map[string]any); ok {
			filename, _ := fileInfo["filename"].(string)
			content, _ := fileInfo["content"].([]byte)
			r.SetFileBytes(fieldName, filename, content)
		}
	}

	// 根据方法类型发送请求
	switch method {
	case "GET":
		return r.Get(url)
	case "POST":
		return r.Post(url)
	case "PUT":
		return r.Put(url)
	case "DELETE":
		return r.Delete(url)
	default:
		return r.Send(method, url)
	}
}

// sendJSONRequest 发送JSON请求
func (h *HTTPManager) sendJSONRequest(r *req.Request, method, url string, headers map[string]string, body any) (*req.Response, error) {
	// 如果没有设置Content-Type，则默认为JSON
	if _, exists := headers["content-type"]; !exists && body != nil {
		r.SetHeader("content-type", "application/json;charset=UTF-8")
	}

	// 根据方法类型发送请求
	switch method {
	case "GET":
		return r.Get(url)
	case "POST":
		return r.SetBody(body).Post(url)
	case "PUT":
		return r.SetBody(body).Put(url)
	case "DELETE":
		return r.SetBody(body).Delete(url)
	default:
		return r.SetBody(body).Send(method, url)
	}
}

// getTLSConfig 获取TLS配置
func (h *HTTPManager) getTLSConfig() *tls.Config {
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

// getDefaultHeaders 获取默认请求头
func (h *HTTPManager) getDefaultHeaders() map[string]string {
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

// validateRequest 验证请求参数
func (h *HTTPManager) validateRequest(method, url string) error {
	if method == "" {
		return fmt.Errorf("请求方法不能为空")
	}

	if url == "" {
		return fmt.Errorf("请求URL不能为空")
	}

	return nil
}

// IsSuccess 检查响应是否成功
func (h *HTTPManager) IsSuccess(response *req.Response) bool {
	if response == nil {
		return false
	}
	return response.StatusCode >= 200 && response.StatusCode < 300
}

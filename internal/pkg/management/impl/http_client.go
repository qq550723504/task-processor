// Package impl 提供HTTP客户端功能
package impl

import (
	"crypto/tls"
	"math/rand"
	"strings"
	"time"

	"github.com/imroc/req/v3"
	"github.com/sirupsen/logrus"
)

// HTTPClient HTTP客户端封装
type HTTPClient struct {
	client  *req.Client
	timeout time.Duration
}

// NewHTTPClient 创建新的HTTP客户端
func NewHTTPClient(timeout time.Duration) *HTTPClient {
	// 创建增强的HTTP客户端
	client := req.C().
		SetTLSFingerprintChrome().
		SetTimeout(timeout).
		// 设置更真实的TLS配置
		SetTLSClientConfig(&tls.Config{
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
		}).
		// 基础请求头，会在每次请求时动态调整
		SetCommonHeaders(map[string]string{
			"Accept":                    "image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8",
			"Accept-Encoding":           "gzip, deflate, br",
			"Accept-Language":           "en-US,en;q=0.9,zh-CN;q=0.8,zh;q=0.7",
			"Cache-Control":             "no-cache",
			"Pragma":                    "no-cache",
			"Sec-Ch-Ua":                 `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`,
			"Sec-Ch-Ua-Mobile":          "?0",
			"Sec-Ch-Ua-Platform":        `"Windows"`,
			"Sec-Fetch-Dest":            "image",
			"Sec-Fetch-Mode":            "no-cors",
			"Sec-Fetch-Site":            "cross-site",
			"Upgrade-Insecure-Requests": "1",
		}).
		// 增强重试策略
		SetCommonRetryCount(5).
		SetCommonRetryInterval(func(resp *req.Response, attempt int) time.Duration {
			// 动态退避策略：基础延迟 + 随机抖动
			baseDelay := time.Duration(attempt*attempt) * time.Second
			jitter := time.Duration(rand.Intn(1000)) * time.Millisecond
			return baseDelay + jitter
		}).
		SetCommonRetryCondition(func(resp *req.Response, err error) bool {
			return isRetryableError(resp, err)
		}).
		SetCommonRetryHook(func(resp *req.Response, err error) {
			logRetryAttempt(resp, err)
		})

	return &HTTPClient{
		client:  client,
		timeout: timeout,
	}
}

// GetClient 获取底层HTTP客户端
func (c *HTTPClient) GetClient() *req.Client {
	return c.client
}

// GetTimeout 获取超时时间
func (c *HTTPClient) GetTimeout() time.Duration {
	return c.timeout
}

// isRetryableError 判断是否为可重试的错误
func isRetryableError(resp *req.Response, err error) bool {
	// 网络错误重试
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "timeout") ||
			strings.Contains(errStr, "deadline exceeded") ||
			strings.Contains(errStr, "connection reset") ||
			strings.Contains(errStr, "broken pipe") ||
			strings.Contains(errStr, "network is unreachable") ||
			strings.Contains(errStr, "no such host") ||
			strings.Contains(errStr, "connection refused") ||
			strings.Contains(errStr, "i/o timeout") ||
			strings.Contains(errStr, "EOF") {
			return true
		}
	}
	// HTTP错误重试
	if resp != nil {
		// 使用安全的方式访问 StatusCode
		statusCode := 0
		getStatusCodeSafely := func() (int, bool) {
			defer func() {
				if r := recover(); r != nil {
					logrus.Warnf("   ⚠️  访问响应状态码时发生恐慌: %v", r)
				}
			}()
			return resp.StatusCode, true
		}

		if sc, ok := getStatusCodeSafely(); ok {
			statusCode = sc
			// 5xx服务器错误
			if statusCode >= 500 {
				return true
			}
			// 特定的风控相关状态码
			if statusCode == 429 || statusCode == 403 {
				return true
			}
		} else {
			logrus.Warnf("   ⚠️  无法安全获取响应状态码")
		}
	}
	return false
}

// logRetryAttempt 记录重试尝试
func logRetryAttempt(resp *req.Response, err error) {
	// 添加空指针检查
	if resp != nil {
		// 使用最安全的方式访问 StatusCode
		statusCode := 0
		getStatusCodeSafely := func() (int, bool) {
			defer func() {
				if r := recover(); r != nil {
					logrus.Warnf("   ⚠️  访问响应状态码时发生恐慌: %v", r)
				}
			}()
			return resp.StatusCode, true
		}

		if sc, ok := getStatusCodeSafely(); ok {
			statusCode = sc
			if statusCode == 429 {
				logrus.Infof("   🚨 触发速率限制: 状态码=429")
			} else if statusCode == 403 {
				logrus.Infof("   🚨 访问被拒绝: 状态码=403")
			} else if statusCode >= 500 && statusCode <= 999 { // 确保是有效的HTTP状态码范围
				logrus.Infof("   🚨 服务器错误: 状态码=%d", statusCode)
			}
		} else {
			logrus.Warnf("   ⚠️  无法安全获取响应状态码")
		}
	} else if err != nil {
		logrus.Infof("   🚨 请求错误: %v", err)
	}
	// 如果resp和err都为nil，记录警告信息
	if resp == nil && err == nil {
		logrus.Warnf("   ⚠️  重试钩子被调用，但resp和err都为nil")
	}
}

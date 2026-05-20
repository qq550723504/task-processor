package management

import (
	"math/rand"
	"strings"
	"task-processor/internal/core/logger"
	"task-processor/internal/pkg/httpclient"
	"time"

	"github.com/imroc/req/v3"
)

// HTTPClient HTTP客户端封装
type HTTPClient struct {
	client  *req.Client
	timeout time.Duration
}

// NewHTTPClient 创建新的HTTP客户端
func NewHTTPClient(timeout time.Duration, insecureSkipVerify bool) *HTTPClient {
	client := httpclient.Build(httpclient.ClientConfig{
		Timeout:            timeout,
		RetryCount:         5,
		InsecureSkipVerify: insecureSkipVerify,
		Headers: map[string]string{
			"Accept":          "image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8",
			"Accept-Encoding": "gzip, deflate, br",
			"Accept-Language": "en-US,en;q=0.9,zh-CN;q=0.8,zh;q=0.7",
			"Cache-Control":   "no-cache",
			"Pragma":          "no-cache",
		},
		RetryInterval: func(resp *req.Response, attempt int) time.Duration {
			base := time.Duration(attempt*attempt) * time.Second
			jitter := time.Duration(rand.Intn(1000)) * time.Millisecond
			return base + jitter
		},
		RetryCondition: func(resp *req.Response, err error) bool {
			return isRetryableError(resp, err)
		},
		RetryHook: func(resp *req.Response, err error) {
			logRetryAttempt(resp, err)
		},
	})

	return &HTTPClient{client: client, timeout: timeout}
}

// GetClient 获取底层HTTP客户端
func (c *HTTPClient) GetClient() *req.Client {
	return c.client
}

// GetTimeout 获取超时时间
func (c *HTTPClient) GetTimeout() time.Duration {
	return c.timeout
}

func isRetryableError(resp *req.Response, err error) bool {
	if err != nil {
		errStr := err.Error()
		retryKeywords := []string{
			"timeout", "deadline exceeded", "connection reset", "broken pipe",
			"network is unreachable", "no such host", "connection refused", "i/o timeout", "EOF",
		}
		for _, kw := range retryKeywords {
			if strings.Contains(errStr, kw) {
				return true
			}
		}
	}
	if resp != nil && resp.Response != nil {
		sc := resp.StatusCode
		if sc >= 500 || sc == 429 || sc == 403 {
			return true
		}
	}
	return false
}

func logRetryAttempt(resp *req.Response, err error) {
	if resp != nil && resp.Response != nil {
		sc := resp.StatusCode
		if sc == 429 {
			logger.GetGlobalLogger("infra/clients").Infof("   🚨 触发速率限制: 状态码=429")
		} else if sc == 403 {
			logger.GetGlobalLogger("infra/clients").Infof("   🚨 访问被拒绝: 状态码=403")
		} else if sc >= 500 {
			logger.GetGlobalLogger("infra/clients").Infof("   🚨 服务器错误: 状态码=%d", sc)
		}
	} else if err != nil {
		logger.GetGlobalLogger("infra/clients").Infof("   🚨 请求错误: %v", err)
	}
}

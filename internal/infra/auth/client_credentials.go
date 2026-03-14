// Package auth 提供认证功能
package auth

import (
	"net/http"
	"task-processor/internal/pkg/httpclient"
	"time"

	"github.com/sirupsen/logrus"
)

// ClientCredentialsAuthClient 客户端凭证模式授权客户端
type ClientCredentialsAuthClient struct {
	baseURL      string
	clientID     string
	clientSecret string
	tenantID     string
	httpClient   *http.Client
	accessToken  string
	expiresAt    time.Time
	logger       *logrus.Logger
}

// NewClientCredentialsAuthClient 创建客户端凭证模式授权客户端
func NewClientCredentialsAuthClient(baseURL, clientID, clientSecret, tenantID string, logger *logrus.Logger) *ClientCredentialsAuthClient {
	// 使用统一的HTTP客户端工厂
	httpClient := httpclient.NewSimple()

	return &ClientCredentialsAuthClient{
		baseURL:      baseURL,
		clientID:     clientID,
		clientSecret: clientSecret,
		tenantID:     tenantID,
		httpClient:   httpClient,
		logger:       logger,
	}
}

// GetAccessToken 获取访问令牌（如果已过期则自动刷新）
func (c *ClientCredentialsAuthClient) GetAccessToken() (string, error) {
	// 如果token还有效，直接返回
	if c.isTokenValid() {
		return c.accessToken, nil
	}

	// 否则获取新token
	return c.fetchAccessToken()
}

// GetTenantID 获取租户ID
func (c *ClientCredentialsAuthClient) GetTenantID() string {
	return c.tenantID
}

// isTokenValid 检查token是否有效
func (c *ClientCredentialsAuthClient) isTokenValid() bool {
	return c.accessToken != "" && time.Now().Before(c.expiresAt)
}

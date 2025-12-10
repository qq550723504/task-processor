// Package api 提供Amazon SP-API客户端
package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// Client Amazon SP-API客户端
type Client struct {
	httpClient    *http.Client
	baseURL       string
	authManager   *AuthManager
	region        string
	marketplaceID string
	logger        *logrus.Entry
}

// Config 客户端配置
type Config struct {
	Region        string
	MarketplaceID string
	ClientID      string
	ClientSecret  string
	RefreshToken  string
	BaseURL       string
	Sandbox       bool // 是否使用沙盒环境
}

// NewClient 创建Amazon SP-API客户端
func NewClient(cfg *Config) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = getRegionEndpoint(cfg.Region, cfg.Sandbox)
	}

	// 创建认证管理器
	authManager := NewAuthManager(cfg.ClientID, cfg.ClientSecret, cfg.RefreshToken)

	logger := logrus.WithFields(logrus.Fields{
		"component": "AmazonAPIClient",
		"region":    cfg.Region,
		"sandbox":   cfg.Sandbox,
	})

	if cfg.Sandbox {
		logger.Warn("⚠️  使用沙盒环境 - 所有操作不会影响真实数据")
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL:       cfg.BaseURL,
		authManager:   authManager,
		region:        cfg.Region,
		marketplaceID: cfg.MarketplaceID,
		logger:        logger,
	}
}

// getRegionEndpoint 获取区域端点
func getRegionEndpoint(region string, sandbox bool) string {
	// 生产环境端点
	prodEndpoints := map[string]string{
		"us-east-1": "https://sellingpartnerapi-na.amazon.com",
		"eu-west-1": "https://sellingpartnerapi-eu.amazon.com",
		"us-west-2": "https://sellingpartnerapi-fe.amazon.com",
	}

	// 沙盒环境端点
	sandboxEndpoints := map[string]string{
		"us-east-1": "https://sandbox.sellingpartnerapi-na.amazon.com",
		"eu-west-1": "https://sandbox.sellingpartnerapi-eu.amazon.com",
		"us-west-2": "https://sandbox.sellingpartnerapi-fe.amazon.com",
	}

	endpoints := prodEndpoints
	if sandbox {
		endpoints = sandboxEndpoints
	}

	if endpoint, exists := endpoints[region]; exists {
		return endpoint
	}

	// 默认使用北美端点
	if sandbox {
		return sandboxEndpoints["us-east-1"]
	}
	return prodEndpoints["us-east-1"]
}

// GetAccessToken 获取访问令牌（自动刷新）
func (c *Client) GetAccessToken(ctx context.Context) (string, error) {
	return c.authManager.GetAccessToken(ctx)
}

// SetAccessToken 设置访问令牌（用于测试）
func (c *Client) SetAccessToken(token string, expiresIn int) {
	c.authManager.SetAccessToken(token, expiresIn)
}

// GetMarketplaceID 获取市场ID
func (c *Client) GetMarketplaceID() string {
	return c.marketplaceID
}

// buildURL 构建完整URL
func (c *Client) buildURL(path string) string {
	return fmt.Sprintf("%s%s", c.baseURL, path)
}

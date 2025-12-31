// Package api 提供Amazon SP-API客户端
package api

import (
	"context"
	"fmt"
	"net/http"
	"task-processor/internal/utils"
	"time"

	"github.com/sirupsen/logrus"
)

// Client Amazon SP-API客户端
type Client struct {
	httpClient     *http.Client
	baseURL        string
	authManager    *AuthManager
	awsSigner      *AWSSigner
	region         string
	marketplaceID  string
	sellerID       string
	logger         *logrus.Entry
	rateLimits     *APIRateLimits
	circuitBreaker *CircuitBreaker
}

// Config 客户端配置
type Config struct {
	Region         string
	MarketplaceID  string
	SellerID       string // 卖家ID
	ClientID       string
	ClientSecret   string
	RefreshToken   string
	AWSAccessKeyID string // AWS访问密钥ID（可选，用于签名）
	AWSSecretKey   string // AWS密钥（可选，用于签名）
	BaseURL        string
}

// NewClient 创建Amazon SP-API客户端
func NewClient(cfg *Config) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = getRegionEndpoint(cfg.Region)
	}

	// 创建认证管理器
	authManager := NewAuthManager(cfg.ClientID, cfg.ClientSecret, cfg.RefreshToken)

	logger := logrus.WithFields(logrus.Fields{
		"component": "AmazonAPIClient",
		"region":    cfg.Region,
	})

	logger.Info("✅ 使用生产环境 - 所有操作将影响真实数据")

	// 创建AWS签名器（如果提供了AWS凭证）
	var awsSigner *AWSSigner
	if cfg.AWSAccessKeyID != "" && cfg.AWSSecretKey != "" {
		awsSigner = NewAWSSigner(cfg.AWSAccessKeyID, cfg.AWSSecretKey, cfg.Region)
		logger.Info("✅ AWS签名器已启用")
	} else {
		logger.Warn("⚠️  未配置AWS凭证，将仅使用LWA令牌认证")
	}

	return &Client{
		httpClient:     utils.CreateSimpleHTTPClient(),
		baseURL:        cfg.BaseURL,
		authManager:    authManager,
		awsSigner:      awsSigner,
		region:         cfg.Region,
		marketplaceID:  cfg.MarketplaceID,
		sellerID:       cfg.SellerID,
		logger:         logger,
		rateLimits:     NewAPIRateLimits(),
		circuitBreaker: NewCircuitBreaker(5, 3, 60*time.Second),
	}
}

// getRegionEndpoint 获取区域端点
func getRegionEndpoint(region string) string {
	// 生产环境端点
	prodEndpoints := map[string]string{
		"us-east-1": "https://sellingpartnerapi-na.amazon.com",
		"eu-west-1": "https://sellingpartnerapi-eu.amazon.com",
		"us-west-2": "https://sellingpartnerapi-fe.amazon.com",
	}

	endpoints := prodEndpoints
	if endpoint, exists := endpoints[region]; exists {
		return endpoint
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

// GetMarketplaceID 获取市场ID（已废弃，请使用 GetMarketplaceID(ctx)）
func (c *Client) GetMarketplaceID() string {
	return c.marketplaceID
}

// buildURL 构建完整URL
func (c *Client) buildURL(path string) string {
	return fmt.Sprintf("%s%s", c.baseURL, path)
}

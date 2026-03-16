// Package openai 提供带缓存的OpenAI客户端
package openai

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

// CachedClient 带缓存的OpenAI客户端装饰器
type CachedClient struct {
	client  *Client
	cache   Cache
	logger  *logrus.Entry
	ttl     time.Duration
	enabled bool
}

// Cache 缓存接口
type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

// CachedClientConfig 缓存客户端配置
type CachedClientConfig struct {
	Client    *Client
	Cache     Cache
	TTL       time.Duration // 缓存过期时间,默认24小时
	Enabled   bool          // 是否启用缓存
	KeyPrefix string        // 缓存键前缀,默认"openai"
}

// NewCachedClient 创建带缓存的OpenAI客户端
func NewCachedClient(config *CachedClientConfig) (*CachedClient, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if config.Client == nil {
		return nil, fmt.Errorf("client cannot be nil")
	}

	// 默认配置
	if config.TTL == 0 {
		config.TTL = 24 * time.Hour
	}
	if config.KeyPrefix == "" {
		config.KeyPrefix = "openai"
	}

	return &CachedClient{
		client:  config.Client,
		cache:   config.Cache,
		logger:  logrus.WithField("component", "CachedOpenAIClient"),
		ttl:     config.TTL,
		enabled: config.Enabled && config.Cache != nil,
	}, nil
}

// CreateChatCompletion 创建聊天完成(带缓存)
func (c *CachedClient) CreateChatCompletion(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error) {
	// 如果缓存未启用,直接调用
	if !c.enabled {
		return c.client.CreateChatCompletion(ctx, req)
	}

	// 生成缓存键
	cacheKey := c.generateCacheKey(req)

	// 尝试从缓存获取
	cached, err := c.cache.Get(ctx, cacheKey)
	if err == nil && cached != "" {
		c.logger.Debugf("缓存命中: %s", cacheKey)

		var resp ChatCompletionResponse
		if unmarshalErr := json.Unmarshal([]byte(cached), &resp); unmarshalErr == nil {
			return &resp, nil
		}
	}

	// 缓存未命中,调用API
	c.logger.Debugf("缓存未命中: %s", cacheKey)
	resp, err := c.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, err
	}

	// 保存到缓存
	if respData, marshalErr := json.Marshal(resp); marshalErr == nil {
		if setErr := c.cache.Set(ctx, cacheKey, string(respData), c.ttl); setErr != nil {
			c.logger.Warnf("保存缓存失败: %v", setErr)
		}
	}

	return resp, nil
}

// generateCacheKey 生成缓存键
func (c *CachedClient) generateCacheKey(req *ChatCompletionRequest) string {
	// 序列化请求参数
	data := map[string]any{
		"model":    req.Model,
		"messages": req.Messages,
	}
	if req.Temperature != nil {
		data["temperature"] = *req.Temperature
	}
	if req.Seed != nil {
		data["seed"] = *req.Seed
	}
	if req.MaxTokens != nil {
		data["max_tokens"] = *req.MaxTokens
	}

	jsonData, _ := json.Marshal(data)

	// 计算SHA256哈希
	hash := sha256.Sum256(jsonData)
	hashStr := hex.EncodeToString(hash[:])

	return fmt.Sprintf("openai:chat:%s", hashStr)
}

// ClearCache 清除指定请求的缓存
func (c *CachedClient) ClearCache(ctx context.Context, req *ChatCompletionRequest) error {
	if !c.enabled {
		return nil
	}
	cacheKey := c.generateCacheKey(req)
	return c.cache.Delete(ctx, cacheKey)
}

// SetTTL 设置缓存过期时间
func (c *CachedClient) SetTTL(ttl time.Duration) {
	c.ttl = ttl
}

// Enable 启用缓存
func (c *CachedClient) Enable() {
	c.enabled = c.cache != nil
}

// Disable 禁用缓存
func (c *CachedClient) Disable() {
	c.enabled = false
}

// IsEnabled 检查缓存是否启用
func (c *CachedClient) IsEnabled() bool {
	return c.enabled
}

// GetStats 获取客户端统计信息
func (c *CachedClient) GetStats() map[string]any {
	stats := c.client.GetStats()
	stats["cache_enabled"] = c.enabled
	stats["cache_ttl"] = c.ttl.String()
	return stats
}

// Close 关闭客户端
func (c *CachedClient) Close() error {
	return c.client.Close()
}

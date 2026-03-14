// Package productjson 提供产品JSON生成的应用层实现
package productjson

import (
	"context"
	"task-processor/internal/domain/productjson"
	"time"
)

// RedisClient Redis客户端接口
type RedisClient interface {
	Push(ctx context.Context, key string, value string) error
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

// WebScraper 网页抓取器接口
type WebScraper interface {
	Scrape(ctx context.Context, url string) (*productjson.ScrapedData, error)
}

// WebScraperConfig 网页抓取器配置
type WebScraperConfig struct {
	Timeout    int64
	MaxRetries int
	RetryDelay int64
	UserAgent  string
}

// MetricsCollector 指标收集器接口
type MetricsCollector interface {
	RecordCacheHit(cacheType string)
	RecordCacheMiss(cacheType string)
	RecordCacheError(cacheType string)
	RecordCacheOperation(operation string, cacheType string)
}

// LLMManager LLM 管理器接口
type LLMManager interface {
	GetClient(clientName string) (LLMClient, error)
	GetDefaultClient() LLMClient
}

// LLMClient LLM 客户端接口
type LLMClient interface {
	Generate(ctx context.Context, prompt string) (string, error)
	AnalyzeImage(ctx context.Context, imageURL string, prompt string) (string, error)
}

// package productenrich 提供产品JSON生成的应用层实现
package productenrich

import (
	"context"
	"encoding/json"
	"task-processor/internal/pkg/hashx"
	"time"

	"task-processor/internal/core/logger"

	"github.com/sirupsen/logrus"
)

// LLMScoreCache LLM 评分缓存接口
type LLMScoreCache interface {
	// GetTextScore 获取文本评分缓存
	GetTextScore(ctx context.Context, text string) (float64, bool)
	// GetTextScoreResult 获取文本评分缓存及其 prompt lineage
	GetTextScoreResult(ctx context.Context, text string) (*CachedLLMScore, bool)
	// SetTextScore 设置文本评分缓存
	SetTextScore(ctx context.Context, text string, score float64, ttl time.Duration) error
	// SetTextScoreResult 设置文本评分缓存及其 prompt lineage
	SetTextScoreResult(ctx context.Context, text string, result *CachedLLMScore, ttl time.Duration) error
	// GetImageScore 获取图片评分缓存
	GetImageScore(ctx context.Context, imageURL string) (float64, bool)
	// GetImageScoreResult 获取图片评分缓存及其 prompt lineage
	GetImageScoreResult(ctx context.Context, imageURL string) (*CachedLLMScore, bool)
	// SetImageScore 设置图片评分缓存
	SetImageScore(ctx context.Context, imageURL string, score float64, ttl time.Duration) error
	// SetImageScoreResult 设置图片评分缓存及其 prompt lineage
	SetImageScoreResult(ctx context.Context, imageURL string, result *CachedLLMScore, ttl time.Duration) error
}

type CachedLLMScore struct {
	Score  float64              `json:"score"`
	Prompt *PromptObservability `json:"prompt,omitempty"`
}

// llmScoreCache LLM 评分缓存实现
type llmScoreCache struct {
	redisClient RedisClient
	metrics     MetricsCollector
}

// NewLLMScoreCache 创建 LLM 评分缓存
func NewLLMScoreCache(redisClient RedisClient, metrics MetricsCollector) LLMScoreCache {
	return &llmScoreCache{
		redisClient: redisClient,
		metrics:     metrics,
	}
}

// GetTextScore 获取文本评分缓存
func (c *llmScoreCache) GetTextScore(ctx context.Context, text string) (float64, bool) {
	result, found := c.GetTextScoreResult(ctx, text)
	if !found || result == nil {
		return 0, false
	}
	return result.Score, true
}

// SetTextScore 设置文本评分缓存
func (c *llmScoreCache) SetTextScore(ctx context.Context, text string, score float64, ttl time.Duration) error {
	return c.SetTextScoreResult(ctx, text, &CachedLLMScore{Score: score}, ttl)
}

func (c *llmScoreCache) GetTextScoreResult(ctx context.Context, text string) (*CachedLLMScore, bool) {
	return c.getScore(ctx, c.getTextScoreCacheKey(text), "llm_text_score")
}

func (c *llmScoreCache) SetTextScoreResult(ctx context.Context, text string, result *CachedLLMScore, ttl time.Duration) error {
	return c.setScore(ctx, c.getTextScoreCacheKey(text), "llm_text_score", result, ttl)
}

// GetImageScore 获取图片评分缓存
func (c *llmScoreCache) GetImageScore(ctx context.Context, imageURL string) (float64, bool) {
	result, found := c.GetImageScoreResult(ctx, imageURL)
	if !found || result == nil {
		return 0, false
	}
	return result.Score, true
}

// SetImageScore 设置图片评分缓存
func (c *llmScoreCache) SetImageScore(ctx context.Context, imageURL string, score float64, ttl time.Duration) error {
	return c.SetImageScoreResult(ctx, imageURL, &CachedLLMScore{Score: score}, ttl)
}

func (c *llmScoreCache) GetImageScoreResult(ctx context.Context, imageURL string) (*CachedLLMScore, bool) {
	return c.getScore(ctx, c.getImageScoreCacheKey(imageURL), "llm_image_score")
}

func (c *llmScoreCache) SetImageScoreResult(ctx context.Context, imageURL string, result *CachedLLMScore, ttl time.Duration) error {
	return c.setScore(ctx, c.getImageScoreCacheKey(imageURL), "llm_image_score", result, ttl)
}

// getScore 通用缓存读取
func (c *llmScoreCache) getScore(ctx context.Context, cacheKey, metricLabel string) (*CachedLLMScore, bool) {
	if c.redisClient == nil {
		return nil, false
	}

	if c.metrics != nil {
		c.metrics.RecordCacheOperation("get", metricLabel)
	}

	cached, err := c.redisClient.Get(ctx, cacheKey)
	if err != nil || cached == "" {
		if c.metrics != nil {
			c.metrics.RecordCacheMiss(metricLabel)
		}
		if err != nil {
			logrus.WithError(err).Debug("cache miss for score")
		}
		return nil, false
	}

	var scoreData CachedLLMScore
	if err := json.Unmarshal([]byte(cached), &scoreData); err != nil {
		if c.metrics != nil {
			c.metrics.RecordCacheMiss(metricLabel)
		}
		logrus.WithError(err).Error("failed to unmarshal cached score")
		return nil, false
	}
	if scoreData.Prompt != nil {
		scoreData.Prompt = scoreData.Prompt.Clone()
	}

	if c.metrics != nil {
		c.metrics.RecordCacheHit(metricLabel)
	}
	logger.GetGlobalLogger("productenrich/llm_score_cache.go").WithFields(logrus.Fields{
		"score":         scoreData.Score,
		"has_prompt":    scoreData.Prompt != nil,
		"prompt_source": promptSource(scoreData.Prompt),
	}).Debug("cache hit for score")
	return &scoreData, true
}

// setScore 通用缓存写入
func (c *llmScoreCache) setScore(ctx context.Context, cacheKey, metricLabel string, result *CachedLLMScore, ttl time.Duration) error {
	if c.redisClient == nil {
		return nil
	}

	if c.metrics != nil {
		c.metrics.RecordCacheOperation("set", metricLabel)
	}

	if result == nil {
		result = &CachedLLMScore{}
	}
	scoreData := CachedLLMScore{
		Score: result.Score,
	}
	if result.Prompt != nil {
		scoreData.Prompt = result.Prompt.Clone()
	}

	data, err := json.Marshal(scoreData)
	if err != nil {
		logrus.WithError(err).Error("failed to marshal score")
		return err
	}

	if err := c.redisClient.Set(ctx, cacheKey, string(data), ttl); err != nil {
		logrus.WithError(err).Error("failed to set cache for score")
		return err
	}

	logger.GetGlobalLogger("productenrich/llm_score_cache.go").WithFields(logrus.Fields{
		"score":         scoreData.Score,
		"ttl":           ttl,
		"has_prompt":    scoreData.Prompt != nil,
		"prompt_source": promptSource(scoreData.Prompt),
	}).Debug("cached score")
	return nil
}

// getTextScoreCacheKey 获取文本评分缓存键
func (c *llmScoreCache) getTextScoreCacheKey(text string) string {
	return "llm_score:text:" + hashx.MD5(text)
}

// getImageScoreCacheKey 获取图片评分缓存键
func (c *llmScoreCache) getImageScoreCacheKey(imageURL string) string {
	return "llm_score:image:" + hashx.MD5(imageURL)
}

func promptSource(prompt *PromptObservability) string {
	if prompt == nil {
		return ""
	}
	return prompt.PromptSource
}

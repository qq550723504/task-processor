// Package productjson 提供产品JSON生成的应用层实现
package productjson

import (
	"context"
	"encoding/json"
	"task-processor/internal/pkg/hashx"
	"time"

	"github.com/sirupsen/logrus"
)

// LLMScoreCache LLM 评分缓存接口
type LLMScoreCache interface {
	// GetTextScore 获取文本评分缓存
	GetTextScore(ctx context.Context, text string) (float64, bool)
	// SetTextScore 设置文本评分缓存
	SetTextScore(ctx context.Context, text string, score float64, ttl time.Duration) error
	// GetImageScore 获取图片评分缓存
	GetImageScore(ctx context.Context, imageURL string) (float64, bool)
	// SetImageScore 设置图片评分缓存
	SetImageScore(ctx context.Context, imageURL string, score float64, ttl time.Duration) error
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
	if c.redisClient == nil {
		return 0, false
	}

	// 记录缓存操作
	c.metrics.RecordCacheOperation("get", "llm_text_score")

	cacheKey := c.getTextScoreCacheKey(text)
	cached, err := c.redisClient.Get(ctx, cacheKey)
	if err != nil {
		c.metrics.RecordCacheMiss("llm_text_score")
		logrus.WithError(err).Debug("cache miss for text score")
		return 0, false
	}

	if cached == "" {
		c.metrics.RecordCacheMiss("llm_text_score")
		return 0, false
	}

	var scoreData struct {
		Score float64 `json:"score"`
	}

	if err := json.Unmarshal([]byte(cached), &scoreData); err != nil {
		c.metrics.RecordCacheMiss("llm_text_score")
		logrus.WithError(err).Error("failed to unmarshal cached text score")
		return 0, false
	}

	c.metrics.RecordCacheHit("llm_text_score")
	logrus.WithField("score", scoreData.Score).Debug("cache hit for text score")

	return scoreData.Score, true
}

// SetTextScore 设置文本评分缓存
func (c *llmScoreCache) SetTextScore(ctx context.Context, text string, score float64, ttl time.Duration) error {
	if c.redisClient == nil {
		return nil
	}

	c.metrics.RecordCacheOperation("set", "llm_text_score")

	scoreData := struct {
		Score float64 `json:"score"`
	}{
		Score: score,
	}

	data, err := json.Marshal(scoreData)
	if err != nil {
		logrus.WithError(err).Error("failed to marshal text score")
		return err
	}

	cacheKey := c.getTextScoreCacheKey(text)
	if err := c.redisClient.Set(ctx, cacheKey, string(data), ttl); err != nil {
		logrus.WithError(err).Error("failed to set cache for text score")
		return err
	}

	logrus.WithFields(logrus.Fields{
		"score": score,
		"ttl":   ttl,
	}).Debug("cached text score")

	return nil
}

// GetImageScore 获取图片评分缓存
func (c *llmScoreCache) GetImageScore(ctx context.Context, imageURL string) (float64, bool) {
	if c.redisClient == nil {
		return 0, false
	}

	c.metrics.RecordCacheOperation("get", "llm_image_score")

	cacheKey := c.getImageScoreCacheKey(imageURL)
	cached, err := c.redisClient.Get(ctx, cacheKey)
	if err != nil {
		c.metrics.RecordCacheMiss("llm_image_score")
		logrus.WithError(err).Debug("cache miss for image score")
		return 0, false
	}

	if cached == "" {
		c.metrics.RecordCacheMiss("llm_image_score")
		return 0, false
	}

	var scoreData struct {
		Score float64 `json:"score"`
	}

	if err := json.Unmarshal([]byte(cached), &scoreData); err != nil {
		c.metrics.RecordCacheMiss("llm_image_score")
		logrus.WithError(err).Error("failed to unmarshal cached image score")
		return 0, false
	}

	c.metrics.RecordCacheHit("llm_image_score")
	logrus.WithField("score", scoreData.Score).Debug("cache hit for image score")

	return scoreData.Score, true
}

// SetImageScore 设置图片评分缓存
func (c *llmScoreCache) SetImageScore(ctx context.Context, imageURL string, score float64, ttl time.Duration) error {
	if c.redisClient == nil {
		return nil
	}

	c.metrics.RecordCacheOperation("set", "llm_image_score")

	scoreData := struct {
		Score float64 `json:"score"`
	}{
		Score: score,
	}

	data, err := json.Marshal(scoreData)
	if err != nil {
		logrus.WithError(err).Error("failed to marshal image score")
		return err
	}

	cacheKey := c.getImageScoreCacheKey(imageURL)
	if err := c.redisClient.Set(ctx, cacheKey, string(data), ttl); err != nil {
		logrus.WithError(err).Error("failed to set cache for image score")
		return err
	}

	logrus.WithFields(logrus.Fields{
		"score": score,
		"ttl":   ttl,
	}).Debug("cached image score")

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

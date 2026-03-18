// package productenrich 提供产品JSON生成的应用层实现
package productenrich

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
	return c.getScore(ctx, c.getTextScoreCacheKey(text), "llm_text_score")
}

// SetTextScore 设置文本评分缓存
func (c *llmScoreCache) SetTextScore(ctx context.Context, text string, score float64, ttl time.Duration) error {
	return c.setScore(ctx, c.getTextScoreCacheKey(text), "llm_text_score", score, ttl)
}

// GetImageScore 获取图片评分缓存
func (c *llmScoreCache) GetImageScore(ctx context.Context, imageURL string) (float64, bool) {
	return c.getScore(ctx, c.getImageScoreCacheKey(imageURL), "llm_image_score")
}

// SetImageScore 设置图片评分缓存
func (c *llmScoreCache) SetImageScore(ctx context.Context, imageURL string, score float64, ttl time.Duration) error {
	return c.setScore(ctx, c.getImageScoreCacheKey(imageURL), "llm_image_score", score, ttl)
}

// getScore 通用缓存读取
func (c *llmScoreCache) getScore(ctx context.Context, cacheKey, metricLabel string) (float64, bool) {
	if c.redisClient == nil {
		return 0, false
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
		return 0, false
	}

	var scoreData struct {
		Score float64 `json:"score"`
	}
	if err := json.Unmarshal([]byte(cached), &scoreData); err != nil {
		if c.metrics != nil {
			c.metrics.RecordCacheMiss(metricLabel)
		}
		logrus.WithError(err).Error("failed to unmarshal cached score")
		return 0, false
	}

	if c.metrics != nil {
		c.metrics.RecordCacheHit(metricLabel)
	}
	logrus.WithField("score", scoreData.Score).Debug("cache hit for score")
	return scoreData.Score, true
}

// setScore 通用缓存写入
func (c *llmScoreCache) setScore(ctx context.Context, cacheKey, metricLabel string, score float64, ttl time.Duration) error {
	if c.redisClient == nil {
		return nil
	}

	if c.metrics != nil {
		c.metrics.RecordCacheOperation("set", metricLabel)
	}

	scoreData := struct {
		Score float64 `json:"score"`
	}{Score: score}

	data, err := json.Marshal(scoreData)
	if err != nil {
		logrus.WithError(err).Error("failed to marshal score")
		return err
	}

	if err := c.redisClient.Set(ctx, cacheKey, string(data), ttl); err != nil {
		logrus.WithError(err).Error("failed to set cache for score")
		return err
	}

	logrus.WithFields(logrus.Fields{"score": score, "ttl": ttl}).Debug("cached score")
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

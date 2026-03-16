// Package productjson 提供产品JSON生成的应用层实现
package productjson

import (
	"context"
	"encoding/json"
	"time"

	"task-processor/internal/domain/productjson"
	"task-processor/internal/pkg/hashx"

	"github.com/sirupsen/logrus"
)

// ValidationCache 验证缓存接口
type ValidationCache interface {
	// GetImageValidation 获取图片验证缓存
	GetImageValidation(ctx context.Context, imageURL string) (*productjson.ImageInfo, bool)
	// SetImageValidation 设置图片验证缓存
	SetImageValidation(ctx context.Context, imageURL string, info *productjson.ImageInfo, ttl time.Duration) error
}

// validationCache 验证缓存实现
type validationCache struct {
	redisClient RedisClient
	metrics     MetricsCollector
}

// NewValidationCache 创建验证缓存
func NewValidationCache(redisClient RedisClient, metrics MetricsCollector) ValidationCache {
	return &validationCache{
		redisClient: redisClient,
		metrics:     metrics,
	}
}

// GetImageValidation 获取图片验证缓存
func (c *validationCache) GetImageValidation(ctx context.Context, imageURL string) (*productjson.ImageInfo, bool) {
	if c.redisClient == nil {
		return nil, false
	}

	// 记录缓存操作
	c.metrics.RecordCacheOperation("get", "image_validation")

	cacheKey := c.getImageCacheKey(imageURL)
	cached, err := c.redisClient.Get(ctx, cacheKey)
	if err != nil {
		// 记录缓存未命中
		c.metrics.RecordCacheMiss("image_validation")
		logrus.WithError(err).WithField("url", imageURL).Debug("cache miss for image validation")
		return nil, false
	}

	if cached == "" {
		// 记录缓存未命中
		c.metrics.RecordCacheMiss("image_validation")
		return nil, false
	}

	var info productjson.ImageInfo
	if err := json.Unmarshal([]byte(cached), &info); err != nil {
		// 记录缓存未命中（解析失败）
		c.metrics.RecordCacheMiss("image_validation")
		logrus.WithError(err).WithField("url", imageURL).Error("failed to unmarshal cached image info")
		return nil, false
	}

	// 记录缓存命中
	c.metrics.RecordCacheHit("image_validation")
	logrus.WithFields(logrus.Fields{
		"url":      imageURL,
		"is_valid": info.IsValid,
	}).Debug("cache hit for image validation")

	return &info, true
}

// SetImageValidation 设置图片验证缓存
func (c *validationCache) SetImageValidation(ctx context.Context, imageURL string, info *productjson.ImageInfo, ttl time.Duration) error {
	if c.redisClient == nil {
		return nil
	}

	// 记录缓存操作
	c.metrics.RecordCacheOperation("set", "image_validation")

	data, err := json.Marshal(info)
	if err != nil {
		logrus.WithError(err).WithField("url", imageURL).Error("failed to marshal image info")
		return err
	}

	cacheKey := c.getImageCacheKey(imageURL)
	if err := c.redisClient.Set(ctx, cacheKey, string(data), ttl); err != nil {
		logrus.WithError(err).WithField("url", imageURL).Error("failed to set cache for image validation")
		return err
	}

	logrus.WithFields(logrus.Fields{
		"url":      imageURL,
		"is_valid": info.IsValid,
		"ttl":      ttl,
	}).Debug("cached image validation result")

	return nil
}

// getImageCacheKey 获取图片缓存键
func (c *validationCache) getImageCacheKey(imageURL string) string {
	return "validation:image:" + hashx.MD5(imageURL)
}

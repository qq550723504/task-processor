// Package handlers 提供属性处理缓存管理，提升处理性能
package handlers

import (
	"fmt"
	"sync"
	"time"

	"task-processor/internal/platforms/temu/types"
)

// PropertyCache 属性缓存接口
type PropertyCache interface {
	// 特征缓存
	GetFeature(key string) (PropertyFeature, bool)
	SetFeature(key string, feature PropertyFeature)

	// 模板属性缓存
	GetTemplateProperty(pid int) (types.TemplateRespGoodsProperty, bool)
	SetTemplateProperty(pid int, templateProp types.TemplateRespGoodsProperty)

	// 清理缓存
	Clear()
	ClearExpired()
}

// InMemoryPropertyCache 内存属性缓存实现
type InMemoryPropertyCache struct {
	featureCache  map[string]*CacheItem[PropertyFeature]
	templateCache map[int]*CacheItem[types.TemplateRespGoodsProperty]
	mutex         sync.RWMutex
	ttl           time.Duration
}

// CacheItem 缓存项
type CacheItem[T any] struct {
	Value     T
	ExpiresAt time.Time
}

// NewPropertyCache 创建新的属性缓存
func NewPropertyCache() PropertyCache {
	return &InMemoryPropertyCache{
		featureCache:  make(map[string]*CacheItem[PropertyFeature]),
		templateCache: make(map[int]*CacheItem[types.TemplateRespGoodsProperty]),
		ttl:           30 * time.Minute,
	}
}

// NewPropertyCacheWithTTL 创建带TTL的属性缓存
func NewPropertyCacheWithTTL(ttl time.Duration) PropertyCache {
	return &InMemoryPropertyCache{
		featureCache:  make(map[string]*CacheItem[PropertyFeature]),
		templateCache: make(map[int]*CacheItem[types.TemplateRespGoodsProperty]),
		ttl:           ttl,
	}
}

// GetFeature 获取属性特征缓存
func (c *InMemoryPropertyCache) GetFeature(key string) (PropertyFeature, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	item, exists := c.featureCache[key]
	if !exists {
		return PropertyFeature{}, false
	}

	// 检查是否过期
	if time.Now().After(item.ExpiresAt) {
		// 异步清理过期项
		go func() {
			c.mutex.Lock()
			delete(c.featureCache, key)
			c.mutex.Unlock()
		}()
		return PropertyFeature{}, false
	}

	return item.Value, true
}

// SetFeature 设置属性特征缓存
func (c *InMemoryPropertyCache) SetFeature(key string, feature PropertyFeature) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.featureCache[key] = &CacheItem[PropertyFeature]{
		Value:     feature,
		ExpiresAt: time.Now().Add(c.ttl),
	}
}

// GetTemplateProperty 获取模板属性缓存
func (c *InMemoryPropertyCache) GetTemplateProperty(pid int) (types.TemplateRespGoodsProperty, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	item, exists := c.templateCache[pid]
	if !exists {
		return types.TemplateRespGoodsProperty{}, false
	}

	// 检查是否过期
	if time.Now().After(item.ExpiresAt) {
		// 异步清理过期项
		go func() {
			c.mutex.Lock()
			delete(c.templateCache, pid)
			c.mutex.Unlock()
		}()
		return types.TemplateRespGoodsProperty{}, false
	}

	return item.Value, true
}

// SetTemplateProperty 设置模板属性缓存
func (c *InMemoryPropertyCache) SetTemplateProperty(pid int, templateProp types.TemplateRespGoodsProperty) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.templateCache[pid] = &CacheItem[types.TemplateRespGoodsProperty]{
		Value:     templateProp,
		ExpiresAt: time.Now().Add(c.ttl),
	}
}

// Clear 清空所有缓存
func (c *InMemoryPropertyCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.featureCache = make(map[string]*CacheItem[PropertyFeature])
	c.templateCache = make(map[int]*CacheItem[types.TemplateRespGoodsProperty])
}

// ClearExpired 清理过期缓存
func (c *InMemoryPropertyCache) ClearExpired() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	now := time.Now()

	// 清理过期的特征缓存
	for key, item := range c.featureCache {
		if now.After(item.ExpiresAt) {
			delete(c.featureCache, key)
		}
	}

	// 清理过期的模板属性缓存
	for pid, item := range c.templateCache {
		if now.After(item.ExpiresAt) {
			delete(c.templateCache, pid)
		}
	}
}

// GenerateFeatureCacheKey 生成特征缓存键
func GenerateFeatureCacheKey(pid, controlType int, valueUnit string) string {
	return fmt.Sprintf("feature_%d_%d_%s", pid, controlType, valueUnit)
}

// CacheStats 缓存统计信息
type CacheStats struct {
	FeatureCacheSize  int
	TemplateCacheSize int
	FeatureHitRate    float64
	TemplateHitRate   float64
}

// GetStats 获取缓存统计信息
func (c *InMemoryPropertyCache) GetStats() CacheStats {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return CacheStats{
		FeatureCacheSize:  len(c.featureCache),
		TemplateCacheSize: len(c.templateCache),
		// TODO: 实现命中率统计
	}
}

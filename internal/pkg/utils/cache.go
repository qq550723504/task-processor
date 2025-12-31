// Package utils 提供工具方法
package utils

import (
	"sync"
	"time"
)

// CacheItem 缓存项
type CacheItem struct {
	Value      interface{}
	Expiration time.Time
}

// IsExpired 检查是否过期
func (item *CacheItem) IsExpired() bool {
	return time.Now().After(item.Expiration)
}

// MemoryCache 内存缓存
type MemoryCache struct {
	items   map[string]*CacheItem
	mutex   sync.RWMutex
	janitor *janitor
}

// NewMemoryCache 创建内存缓存
func NewMemoryCache(cleanupInterval time.Duration) *MemoryCache {
	cache := &MemoryCache{
		items: make(map[string]*CacheItem),
	}

	// 启动清理协程
	if cleanupInterval > 0 {
		cache.janitor = &janitor{
			interval: cleanupInterval,
			stop:     make(chan bool),
		}
		go cache.janitor.run(cache)
	}

	return cache
}

// Set 设置缓存项
func (c *MemoryCache) Set(key string, value interface{}, duration time.Duration) {
	expiration := time.Now().Add(duration)

	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.items[key] = &CacheItem{
		Value:      value,
		Expiration: expiration,
	}
}

// Get 获取缓存项
func (c *MemoryCache) Get(key string) (interface{}, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return nil, false
	}

	if item.IsExpired() {
		return nil, false
	}

	return item.Value, true
}

// Delete 删除缓存项
func (c *MemoryCache) Delete(key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.items, key)
}

// Clear 清空缓存
func (c *MemoryCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.items = make(map[string]*CacheItem)
}

// Size 获取缓存大小
func (c *MemoryCache) Size() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return len(c.items)
}

// Keys 获取所有键
func (c *MemoryCache) Keys() []string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	keys := make([]string, 0, len(c.items))
	for key := range c.items {
		keys = append(keys, key)
	}

	return keys
}

// cleanup 清理过期项
func (c *MemoryCache) cleanup() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for key, item := range c.items {
		if item.IsExpired() {
			delete(c.items, key)
		}
	}
}

// Close 关闭缓存
func (c *MemoryCache) Close() {
	if c.janitor != nil {
		c.janitor.stop <- true
	}
}

// janitor 清理工
type janitor struct {
	interval time.Duration
	stop     chan bool
}

// run 运行清理任务
func (j *janitor) run(cache *MemoryCache) {
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cache.cleanup()
		case <-j.stop:
			return
		}
	}
}

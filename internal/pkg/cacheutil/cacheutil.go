// Package cacheutil 提供内存缓存工具
package cacheutil

import (
	"sync"
	"time"
)

// Item 缓存项
type Item struct {
	Value      any
	Expiration time.Time
}

// IsExpired 检查是否过期
func (item *Item) IsExpired() bool {
	return time.Now().After(item.Expiration)
}

// MemoryCache 内存缓存
type MemoryCache struct {
	items   map[string]*Item
	mutex   sync.RWMutex
	janitor *janitor
}

// NewMemoryCache 创建内存缓存
func NewMemoryCache(cleanupInterval time.Duration) *MemoryCache {
	c := &MemoryCache{items: make(map[string]*Item)}
	if cleanupInterval > 0 {
		c.janitor = &janitor{interval: cleanupInterval, stop: make(chan bool)}
		go c.janitor.run(c)
	}
	return c
}

// Set 设置缓存项
func (c *MemoryCache) Set(key string, value any, duration time.Duration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.items[key] = &Item{Value: value, Expiration: time.Now().Add(duration)}
}

// Get 获取缓存项
func (c *MemoryCache) Get(key string) (any, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	item, ok := c.items[key]
	if !ok || item.IsExpired() {
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
	c.items = make(map[string]*Item)
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
	for k := range c.items {
		keys = append(keys, k)
	}
	return keys
}

// Close 关闭缓存
func (c *MemoryCache) Close() {
	if c.janitor != nil {
		c.janitor.stop <- true
	}
}

func (c *MemoryCache) cleanup() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	for k, item := range c.items {
		if item.IsExpired() {
			delete(c.items, k)
		}
	}
}

type janitor struct {
	interval time.Duration
	stop     chan bool
}

func (j *janitor) run(c *MemoryCache) {
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.cleanup()
		case <-j.stop:
			return
		}
	}
}

// Package di 提供实例缓存实现
package di

import (
	"sync"
)

// cacheImpl 实例缓存实现
type cacheImpl struct {
	mu        sync.RWMutex
	instances map[string]any
}

// NewInstanceCache 创建实例缓存
func NewInstanceCache() InstanceCache {
	return &cacheImpl{
		instances: make(map[string]any),
	}
}

// Get 获取缓存的实例
func (c *cacheImpl) Get(name string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	instance, exists := c.instances[name]
	return instance, exists
}

// Set 设置缓存实例
func (c *cacheImpl) Set(name string, instance any) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.instances[name] = instance
}

// Delete 删除缓存实例
func (c *cacheImpl) Delete(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.instances, name)
}

// Clear 清空缓存
func (c *cacheImpl) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.instances = make(map[string]any)
}

// Package di 提供依赖注入容器实现
package di

import (
	"fmt"
	"sync"
)

// containerImpl 依赖注入容器实现
type containerImpl struct {
	services map[string]*serviceEntry
	cache    map[string]any
	mu       sync.RWMutex
	closed   bool
}

type serviceEntry struct {
	factory   Factory
	singleton bool
}

// NewContainer 创建新的依赖注入容器
func NewContainer() Container {
	return &containerImpl{
		services: make(map[string]*serviceEntry),
		cache:    make(map[string]any),
	}
}

// Register 注册服务工厂
func (c *containerImpl) Register(name string, factory Factory) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("容器已关闭")
	}
	if _, exists := c.services[name]; exists {
		return fmt.Errorf("服务 %s 已经注册", name)
	}

	c.services[name] = &serviceEntry{factory: factory, singleton: false}
	return nil
}

// RegisterSingleton 注册单例服务
func (c *containerImpl) RegisterSingleton(name string, factory Factory) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("容器已关闭")
	}
	if _, exists := c.services[name]; exists {
		return fmt.Errorf("服务 %s 已经注册", name)
	}

	c.services[name] = &serviceEntry{factory: factory, singleton: true}
	return nil
}

// Get 获取服务实例
func (c *containerImpl) Get(name string) (any, error) {
	c.mu.RLock()
	entry, exists := c.services[name]
	if !exists {
		c.mu.RUnlock()
		return nil, fmt.Errorf("服务 %s 未注册", name)
	}

	if entry.singleton {
		if instance, ok := c.cache[name]; ok {
			c.mu.RUnlock()
			return instance, nil
		}
	}
	c.mu.RUnlock()

	// 创建实例（不持锁，避免死锁）
	instance, err := entry.factory(c)
	if err != nil {
		return nil, fmt.Errorf("创建服务 %s 失败: %w", name, err)
	}

	if entry.singleton {
		c.mu.Lock()
		c.cache[name] = instance
		c.mu.Unlock()
	}

	return instance, nil
}

// Has 检查服务是否已注册
func (c *containerImpl) Has(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, exists := c.services[name]
	return exists
}

// Close 关闭容器，清理资源
func (c *containerImpl) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.cache = make(map[string]any)
	c.services = make(map[string]*serviceEntry)
	c.closed = true
	return nil
}

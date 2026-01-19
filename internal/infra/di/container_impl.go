// Package di 提供依赖注入容器实现
package di

import (
	"fmt"
	"reflect"
	"sync"
)

// containerImpl 依赖注入容器实现
type containerImpl struct {
	registry ServiceRegistry
	cache    InstanceCache
	mu       sync.RWMutex
	closed   bool
}

// NewContainer 创建新的依赖注入容器
func NewContainer() Container {
	return &containerImpl{
		registry: NewServiceRegistry(),
		cache:    NewInstanceCache(),
	}
}

// Register 注册服务工厂
func (c *containerImpl) Register(name string, factory Factory) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("容器已关闭")
	}

	return c.registry.Register(name, factory, false)
}

// RegisterSingleton 注册单例服务
func (c *containerImpl) RegisterSingleton(name string, factory Factory) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("容器已关闭")
	}

	return c.registry.Register(name, factory, true)
}

// Get 获取服务实例
func (c *containerImpl) Get(name string) (any, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, fmt.Errorf("容器已关闭")
	}

	// 获取服务注册信息
	factory, singleton, err := c.registry.Get(name)
	if err != nil {
		return nil, err
	}

	// 如果是单例，先检查缓存
	if singleton {
		if instance, exists := c.cache.Get(name); exists {
			return instance, nil
		}
	}

	// 创建实例
	instance, err := factory(c)
	if err != nil {
		return nil, fmt.Errorf("创建服务 %s 失败: %w", name, err)
	}

	// 如果是单例，缓存实例
	if singleton {
		c.cache.Set(name, instance)
	}

	return instance, nil
}

// GetByType 根据类型获取服务实例
func (c *containerImpl) GetByType(serviceType reflect.Type) (any, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, fmt.Errorf("容器已关闭")
	}

	// 遍历所有注册的服务，找到匹配的类型
	services := c.registry.List()
	for _, name := range services {
		instance, err := c.Get(name)
		if err != nil {
			continue
		}

		instanceType := reflect.TypeOf(instance)
		if instanceType == serviceType || instanceType.Implements(serviceType) {
			return instance, nil
		}
	}

	return nil, fmt.Errorf("未找到类型 %s 的服务", serviceType.String())
}

// Has 检查服务是否已注册
func (c *containerImpl) Has(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	_, _, err := c.registry.Get(name)
	return err == nil
}

// Close 关闭容器，清理资源
func (c *containerImpl) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	// 清理缓存
	c.cache.Clear()

	// 清理注册表
	c.registry.Clear()

	c.closed = true
	return nil
}

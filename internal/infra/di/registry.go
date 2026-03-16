// Package di 提供服务注册表实现
package di

import (
	"fmt"
	"sync"
)

// serviceEntry 服务注册条目
type serviceEntry struct {
	factory   Factory
	singleton bool
}

// registryImpl 服务注册表实现
type registryImpl struct {
	mu       sync.RWMutex
	services map[string]*serviceEntry
}

// NewServiceRegistry 创建服务注册表
func NewServiceRegistry() ServiceRegistry {
	return &registryImpl{
		services: make(map[string]*serviceEntry),
	}
}

// Register 注册服务
func (r *registryImpl) Register(name string, factory Factory, singleton bool) error {
	if name == "" {
		return fmt.Errorf("服务名称不能为空")
	}

	if factory == nil {
		return fmt.Errorf("服务工厂不能为空")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.services[name]; exists {
		return fmt.Errorf("服务 %s 已经注册", name)
	}

	r.services[name] = &serviceEntry{
		factory:   factory,
		singleton: singleton,
	}

	return nil
}

// Get 获取服务
func (r *registryImpl) Get(name string) (Factory, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, exists := r.services[name]
	if !exists {
		return nil, false, fmt.Errorf("服务 %s 未注册", name)
	}

	return entry.factory, entry.singleton, nil
}

// List 列出所有已注册的服务
func (r *registryImpl) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.services))
	for name := range r.services {
		names = append(names, name)
	}

	return names
}

// Clear 清空注册表
func (r *registryImpl) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.services = make(map[string]*serviceEntry)
}

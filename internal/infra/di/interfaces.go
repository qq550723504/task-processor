// Package di 提供依赖注入容器接口定义
package di

import (
	"reflect"
)

// Container 依赖注入容器接口
type Container interface {
	// Register 注册服务工厂
	Register(name string, factory Factory) error

	// RegisterSingleton 注册单例服务
	RegisterSingleton(name string, factory Factory) error

	// Get 获取服务实例
	Get(name string) (any, error)

	// GetByType 根据类型获取服务实例
	GetByType(serviceType reflect.Type) (any, error)

	// Has 检查服务是否已注册
	Has(name string) bool

	// Close 关闭容器，清理资源
	Close() error
}

// Factory 服务工厂函数
type Factory func(c Container) (any, error)

// ServiceRegistry 服务注册表接口
type ServiceRegistry interface {
	// Register 注册服务
	Register(name string, factory Factory, singleton bool) error

	// Get 获取服务
	Get(name string) (Factory, bool, error)

	// List 列出所有已注册的服务
	List() []string

	// Clear 清空注册表
	Clear()
}

// InstanceCache 实例缓存接口
type InstanceCache interface {
	// Get 获取缓存的实例
	Get(name string) (any, bool)

	// Set 设置缓存实例
	Set(name string, instance any)

	// Delete 删除缓存实例
	Delete(name string)

	// Clear 清空缓存
	Clear()
}

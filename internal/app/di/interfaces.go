// Package di 提供轻量级依赖注入容器
package di

// ContainerReader 只读容器接口，用于只需要获取服务的消费者
type ContainerReader interface {
	Get(name string) (any, error)
	Has(name string) bool
}

// Container 依赖注入容器完整接口
type Container interface {
	ContainerReader
	Register(name string, factory Factory) error
	RegisterSingleton(name string, factory Factory) error
	Close() error
}

// Factory 服务工厂函数
type Factory func(c Container) (any, error)

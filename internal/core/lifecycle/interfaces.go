// Package lifecycle 提供统一的组件生命周期管理接口
package lifecycle

import (
	"context"
)

// Component 组件接口，所有需要生命周期管理的组件都应实现此接口
type Component interface {
	// Name 返回组件名称
	Name() string

	// Dependencies 返回依赖的组件名称列表
	Dependencies() []string

	// Priority 返回组件优先级，数值越小优先级越高
	Priority() int

	// Start 启动组件
	Start(ctx context.Context) error

	// Stop 停止组件
	Stop(ctx context.Context) error

	// IsRunning 检查组件是否正在运行
	IsRunning() bool
}

// LifecycleManager 生命周期管理器接口
type LifecycleManager interface {
	// Register 注册组件
	Register(component Component) error

	// StartAll 启动所有组件
	StartAll(ctx context.Context) error

	// StopAll 停止所有组件
	StopAll(ctx context.Context) error

	// GetStatus 获取所有组件状态
	GetStatus() map[string]ComponentStatus

	// GetComponent 根据名称获取组件
	GetComponent(name string) (Component, bool)
}

// ComponentStatus 组件状态
type ComponentStatus struct {
	Name         string   `json:"name"`
	Running      bool     `json:"running"`
	Dependencies []string `json:"dependencies"`
	Priority     int      `json:"priority"`
	Error        string   `json:"error,omitempty"`
}

// HealthChecker 健康检查接口
type HealthChecker interface {
	// Check 执行健康检查
	Check(ctx context.Context) error

	// Name 返回检查器名称
	Name() string
}

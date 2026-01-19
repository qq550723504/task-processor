// Package lifecycle 提供基础组件实现
package lifecycle

import (
	"context"
	"sync"
)

// BaseComponent 基础组件实现，提供通用的生命周期管理功能
type BaseComponent struct {
	name         string
	dependencies []string
	priority     int
	running      bool
	mu           sync.RWMutex
}

// NewBaseComponent 创建基础组件
func NewBaseComponent(name string, dependencies []string, priority int) *BaseComponent {
	return &BaseComponent{
		name:         name,
		dependencies: dependencies,
		priority:     priority,
	}
}

// Name 返回组件名称
func (c *BaseComponent) Name() string {
	return c.name
}

// Dependencies 返回依赖的组件名称列表
func (c *BaseComponent) Dependencies() []string {
	return c.dependencies
}

// Priority 返回组件优先级
func (c *BaseComponent) Priority() int {
	return c.priority
}

// IsRunning 检查组件是否正在运行
func (c *BaseComponent) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running
}

// SetRunning 设置运行状态
func (c *BaseComponent) SetRunning(running bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.running = running
}

// Start 启动组件（基础实现，子类应重写）
func (c *BaseComponent) Start(ctx context.Context) error {
	c.SetRunning(true)
	return nil
}

// Stop 停止组件（基础实现，子类应重写）
func (c *BaseComponent) Stop(ctx context.Context) error {
	c.SetRunning(false)
	return nil
}

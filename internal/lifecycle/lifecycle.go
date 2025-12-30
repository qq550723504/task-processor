// Package lifecycle 提供组件生命周期管理
package lifecycle

import (
	"context"
	"sync"
	"time"

	"task-processor/internal/errors"

	"github.com/sirupsen/logrus"
)

// Component 组件接口，所有需要生命周期管理的组件都应实现此接口
type Component interface {
	// Name 返回组件名称
	Name() string

	// Start 启动组件
	Start(ctx context.Context) error

	// Stop 停止组件
	Stop(ctx context.Context) error

	// IsRunning 检查组件是否正在运行
	IsRunning() bool
}

// Manager 生命周期管理器
type Manager struct {
	components []Component
	logger     *logrus.Logger
	mu         sync.RWMutex
	running    bool
}

// NewManager 创建新的生命周期管理器
func NewManager(logger *logrus.Logger) *Manager {
	return &Manager{
		components: make([]Component, 0),
		logger:     logger,
	}
}

// Register 注册组件
func (m *Manager) Register(component Component) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.components = append(m.components, component)
	m.logger.Infof("注册组件: %s", component.Name())
}

// StartAll 启动所有组件
func (m *Manager) StartAll(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return errors.New(errors.ErrCodeSystem, "生命周期管理器已在运行")
	}

	m.logger.Info("开始启动所有组件...")

	for i, component := range m.components {
		m.logger.Infof("启动组件 [%d/%d]: %s", i+1, len(m.components), component.Name())

		if err := component.Start(ctx); err != nil {
			// 启动失败，回滚已启动的组件
			m.logger.Errorf("组件 %s 启动失败: %v", component.Name(), err)
			m.rollbackStartedComponents(ctx, i-1)
			return errors.Wrapf(err, errors.ErrCodeSystem, "组件 %s 启动失败", component.Name())
		}

		m.logger.Infof("组件 %s 启动成功", component.Name())
	}

	m.running = true
	m.logger.Infof("所有组件启动完成，共 %d 个组件", len(m.components))
	return nil
}

// StopAll 停止所有组件
func (m *Manager) StopAll(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		m.logger.Info("生命周期管理器未运行，无需停止")
		return nil
	}

	m.logger.Info("开始停止所有组件...")

	// 反向停止组件（后启动的先停止）
	var lastError error
	for i := len(m.components) - 1; i >= 0; i-- {
		component := m.components[i]
		m.logger.Infof("停止组件 [%d/%d]: %s", len(m.components)-i, len(m.components), component.Name())

		if err := component.Stop(ctx); err != nil {
			m.logger.Errorf("组件 %s 停止失败: %v", component.Name(), err)
			lastError = err
			// 继续停止其他组件，不要因为一个组件停止失败就中断
		} else {
			m.logger.Infof("组件 %s 停止成功", component.Name())
		}
	}

	m.running = false

	if lastError != nil {
		return errors.Wrap(lastError, errors.ErrCodeSystem, "部分组件停止失败")
	}

	m.logger.Info("所有组件停止完成")
	return nil
}

// StopAllWithTimeout 带超时的停止所有组件
func (m *Manager) StopAllWithTimeout(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- m.StopAll(ctx)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return errors.New(errors.ErrCodeTimeout, "停止组件超时")
	}
}

// IsRunning 检查管理器是否正在运行
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// GetComponents 获取所有组件
func (m *Manager) GetComponents() []Component {
	m.mu.RLock()
	defer m.mu.RUnlock()

	components := make([]Component, len(m.components))
	copy(components, m.components)
	return components
}

// GetComponentStatus 获取组件状态
func (m *Manager) GetComponentStatus() map[string]bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := make(map[string]bool)
	for _, component := range m.components {
		status[component.Name()] = component.IsRunning()
	}
	return status
}

// rollbackStartedComponents 回滚已启动的组件
func (m *Manager) rollbackStartedComponents(ctx context.Context, lastIndex int) {
	m.logger.Warn("开始回滚已启动的组件...")

	for i := lastIndex; i >= 0; i-- {
		component := m.components[i]
		m.logger.Infof("回滚组件: %s", component.Name())

		if err := component.Stop(ctx); err != nil {
			m.logger.Errorf("回滚组件 %s 失败: %v", component.Name(), err)
		}
	}

	m.logger.Info("组件回滚完成")
}

// BaseComponent 基础组件实现，其他组件可以嵌入此结构
type BaseComponent struct {
	name    string
	running bool
	mu      sync.RWMutex
}

// NewBaseComponent 创建基础组件
func NewBaseComponent(name string) *BaseComponent {
	return &BaseComponent{
		name: name,
	}
}

// Name 返回组件名称
func (c *BaseComponent) Name() string {
	return c.name
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

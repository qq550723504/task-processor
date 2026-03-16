// Package lifecycle 提供生命周期管理器实现
package lifecycle

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/sirupsen/logrus"
)

// managerImpl 生命周期管理器实现
type managerImpl struct {
	logger     *logrus.Logger
	components map[string]Component
	mu         sync.RWMutex
}

// NewLifecycleManager 创建生命周期管理器
func NewLifecycleManager(logger *logrus.Logger) LifecycleManager {
	return &managerImpl{
		logger:     logger,
		components: make(map[string]Component),
	}
}

// Register 注册组件
func (m *managerImpl) Register(component Component) error {
	if component == nil {
		return fmt.Errorf("组件不能为空")
	}

	name := component.Name()
	if name == "" {
		return fmt.Errorf("组件名称不能为空")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.components[name]; exists {
		return fmt.Errorf("组件 %s 已经注册", name)
	}

	m.components[name] = component
	m.logger.Debugf("组件 %s 注册成功", name)
	return nil
}

// StartAll 启动所有组件
func (m *managerImpl) StartAll(ctx context.Context) error {
	m.mu.RLock()
	components := make([]Component, 0, len(m.components))
	for _, component := range m.components {
		components = append(components, component)
	}
	m.mu.RUnlock()

	// 按优先级和依赖关系排序
	sortedComponents, err := m.sortComponentsByDependencies(components)
	if err != nil {
		return fmt.Errorf("组件依赖关系排序失败: %w", err)
	}

	m.logger.Infof("开始启动 %d 个组件...", len(sortedComponents))

	// 按顺序启动组件
	for _, component := range sortedComponents {
		if component.IsRunning() {
			m.logger.Debugf("组件 %s 已经在运行", component.Name())
			continue
		}

		m.logger.Infof("启动组件: %s", component.Name())
		if err := component.Start(ctx); err != nil {
			m.logger.Errorf("启动组件 %s 失败: %v", component.Name(), err)
			return fmt.Errorf("启动组件 %s 失败: %w", component.Name(), err)
		}
		m.logger.Infof("✅ 组件 %s 启动成功", component.Name())
	}

	m.logger.Info("所有组件启动完成")
	return nil
}

// StopAll 停止所有组件
func (m *managerImpl) StopAll(ctx context.Context) error {
	m.mu.RLock()
	components := make([]Component, 0, len(m.components))
	for _, component := range m.components {
		components = append(components, component)
	}
	m.mu.RUnlock()

	// 按优先级和依赖关系排序（逆序停止）
	sortedComponents, err := m.sortComponentsByDependencies(components)
	if err != nil {
		m.logger.Warnf("组件依赖关系排序失败，按注册顺序停止: %v", err)
		sortedComponents = components
	}

	// 逆序停止组件
	m.logger.Infof("开始停止 %d 个组件...", len(sortedComponents))
	var lastError error

	for i := len(sortedComponents) - 1; i >= 0; i-- {
		component := sortedComponents[i]
		if !component.IsRunning() {
			m.logger.Debugf("组件 %s 已经停止", component.Name())
			continue
		}

		m.logger.Infof("停止组件: %s", component.Name())
		if err := component.Stop(ctx); err != nil {
			m.logger.Errorf("停止组件 %s 失败: %v", component.Name(), err)
			lastError = err
		} else {
			m.logger.Infof("✅ 组件 %s 停止成功", component.Name())
		}
	}

	if lastError != nil {
		return fmt.Errorf("停止组件时发生错误: %w", lastError)
	}

	m.logger.Info("所有组件停止完成")
	return nil
}

// GetStatus 获取所有组件状态
func (m *managerImpl) GetStatus() map[string]ComponentStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := make(map[string]ComponentStatus)
	for name, component := range m.components {
		status[name] = ComponentStatus{
			Name:         name,
			Running:      component.IsRunning(),
			Dependencies: component.Dependencies(),
			Priority:     component.Priority(),
		}
	}

	return status
}

// GetComponent 根据名称获取组件
func (m *managerImpl) GetComponent(name string) (Component, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	component, exists := m.components[name]
	return component, exists
}

// sortComponentsByDependencies 按依赖关系和优先级排序组件
func (m *managerImpl) sortComponentsByDependencies(components []Component) ([]Component, error) {
	// 创建组件映射
	componentMap := make(map[string]Component)
	for _, component := range components {
		componentMap[component.Name()] = component
	}

	// 检查依赖关系是否存在
	for _, component := range components {
		for _, dep := range component.Dependencies() {
			if _, exists := componentMap[dep]; !exists {
				return nil, fmt.Errorf("组件 %s 依赖的组件 %s 不存在", component.Name(), dep)
			}
		}
	}

	// 拓扑排序
	visited := make(map[string]bool)
	visiting := make(map[string]bool)
	result := make([]Component, 0, len(components))

	var visit func(string) error
	visit = func(name string) error {
		if visiting[name] {
			return fmt.Errorf("检测到循环依赖，涉及组件: %s", name)
		}
		if visited[name] {
			return nil
		}

		visiting[name] = true
		component := componentMap[name]

		// 先访问依赖的组件
		for _, dep := range component.Dependencies() {
			if err := visit(dep); err != nil {
				return err
			}
		}

		visiting[name] = false
		visited[name] = true
		result = append(result, component)
		return nil
	}

	// 访问所有组件
	for _, component := range components {
		if err := visit(component.Name()); err != nil {
			return nil, err
		}
	}

	// 按优先级排序同级组件
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Priority() < result[j].Priority()
	})

	return result, nil
}

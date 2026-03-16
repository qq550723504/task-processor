// Package scheduler 提供统一的任务调度管理功能
package scheduler

import (
	"context"
	"fmt"
	"sync"
	"task-processor/internal/core/logger"

	"github.com/sirupsen/logrus"
)

// Manager 调度器管理器
type Manager struct {
	registry          *Registry
	executors         map[string]*TaskExecutor // key: taskID
	dependencyManager *DependencyManager
	monitorService    *MonitorService
	mutex             sync.RWMutex
	ctx               context.Context
	cancel            context.CancelFunc
	logger            *logrus.Entry
}

// NewManager 创建新的调度器管理器
func NewManager(ctx context.Context) *Manager {
	managerCtx, cancel := context.WithCancel(ctx)

	// 创建依赖管理器并注册默认依赖关系
	depManager := NewDependencyManager()
	for _, dep := range GetDefaultDependencies() {
		depManager.RegisterDependency(dep)
	}

	manager := &Manager{
		registry:          NewRegistry(),
		executors:         make(map[string]*TaskExecutor),
		dependencyManager: depManager,
		ctx:               managerCtx,
		cancel:            cancel,
		logger: logger.GetGlobalLogger("scheduler_manager").WithField(
			logger.FieldComponent, "scheduler_manager",
		),
	}

	// 创建监控服务
	manager.monitorService = NewMonitorService(manager)

	return manager
}

// RegisterFactory 注册任务工厂
func (m *Manager) RegisterFactory(factory TaskFactory) error {
	return m.registry.Register(factory)
}

// CreateAndStartTask 创建并启动任务
func (m *Manager) CreateAndStartTask(config TaskConfig) error {
	// 获取任务工厂
	factory, err := m.registry.GetFactory(config.Platform)
	if err != nil {
		return fmt.Errorf("获取任务工厂失败: %w", err)
	}

	// 创建任务
	task, err := factory.CreateTask(m.ctx, config)
	if err != nil {
		return fmt.Errorf("创建任务失败: %w", err)
	}

	taskID := task.GetID()

	// 检查任务是否已存在
	m.mutex.Lock()
	if _, exists := m.executors[taskID]; exists {
		m.mutex.Unlock()
		return fmt.Errorf("任务 %s 已存在", taskID)
	}

	// 创建任务执行器
	executor := NewTaskExecutor(m.ctx, task, m.dependencyManager)
	m.executors[taskID] = executor
	m.mutex.Unlock()

	// 启动任务
	if config.AutoStart {
		executor.Start()
		m.logger.WithFields(logrus.Fields{
			"task_id":   taskID,
			"platform":  config.Platform,
			"task_type": config.TaskType,
		}).Info("成功创建并启动任务")
	} else {
		m.logger.WithFields(logrus.Fields{
			"task_id":    taskID,
			"platform":   config.Platform,
			"task_type":  config.TaskType,
			"auto_start": false,
		}).Info("成功创建任务（未启动）")
	}

	return nil
}

// StartTask 启动任务
func (m *Manager) StartTask(taskID string) error {
	m.mutex.RLock()
	executor, exists := m.executors[taskID]
	m.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("任务 %s 不存在", taskID)
	}

	executor.Start()
	m.logger.WithField("task_id", taskID).Info("启动任务")
	return nil
}

// StopTask 停止任务
func (m *Manager) StopTask(taskID string) error {
	m.mutex.RLock()
	executor, exists := m.executors[taskID]
	m.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("任务 %s 不存在", taskID)
	}

	executor.Stop()
	m.logger.WithField("task_id", taskID).Info("停止任务")
	return nil
}

// RemoveTask 移除任务
func (m *Manager) RemoveTask(taskID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	executor, exists := m.executors[taskID]
	if !exists {
		return fmt.Errorf("任务 %s 不存在", taskID)
	}

	// 先停止任务
	executor.Stop()

	// 从映射中删除
	delete(m.executors, taskID)

	m.logger.WithField("task_id", taskID).Info("移除任务")
	return nil
}

// ListTasks 列出所有任务
func (m *Manager) ListTasks() []Task {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	tasks := make([]Task, 0, len(m.executors))
	for _, executor := range m.executors {
		tasks = append(tasks, executor.GetTask())
	}

	return tasks
}

// GetTask 获取任务
func (m *Manager) GetTask(taskID string) (Task, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	executor, exists := m.executors[taskID]
	if !exists {
		return nil, fmt.Errorf("任务 %s 不存在", taskID)
	}

	return executor.GetTask(), nil
}

// StopAll 停止所有任务
func (m *Manager) StopAll() {
	m.logger.Info("停止所有任务")
	m.cancel()

	m.mutex.Lock()
	defer m.mutex.Unlock()

	for taskID, executor := range m.executors {
		executor.Stop()
		m.logger.WithField("task_id", taskID).Info("已停止任务")
	}

	m.executors = make(map[string]*TaskExecutor)
	m.logger.Info("所有任务已停止")
}

// GetTaskCount 获取任务数量
func (m *Manager) GetTaskCount() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return len(m.executors)
}

// GetRegistry 获取注册表
func (m *Manager) GetRegistry() *Registry {
	return m.registry
}

// GetDependencyManager 获取依赖管理器
func (m *Manager) GetDependencyManager() *DependencyManager {
	return m.dependencyManager
}

// GetMonitorService 获取监控服务
func (m *Manager) GetMonitorService() *MonitorService {
	return m.monitorService
}

// Package scheduler 提供任务调度相关功能
package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// TaskDependency 任务依赖关系
type TaskDependency struct {
	TaskType     TaskType      // 当前任务类型
	DependsOn    []TaskType    // 依赖的任务类型列表
	WaitTimeout  time.Duration // 等待依赖任务完成的超时时间
	RetryOnError bool          // 依赖任务失败时是否重试
}

// TaskExecutionStatus 任务执行状态
type TaskExecutionStatus struct {
	TaskID      string
	TaskType    TaskType
	StoreID     int64
	LastRunTime time.Time
	LastStatus  string // "success", "failed", "running"
	Error       error
	mutex       sync.RWMutex
}

// DependencyManager 依赖管理器
type DependencyManager struct {
	dependencies map[TaskType]*TaskDependency
	statuses     map[string]*TaskExecutionStatus // key: platform:taskType:storeID
	mutex        sync.RWMutex
	logger       *logrus.Logger
}

// NewDependencyManager 创建依赖管理器
func NewDependencyManager() *DependencyManager {
	return &DependencyManager{
		dependencies: make(map[TaskType]*TaskDependency),
		statuses:     make(map[string]*TaskExecutionStatus),
		logger:       logrus.StandardLogger(),
	}
}

// RegisterDependency 注册任务依赖关系
func (dm *DependencyManager) RegisterDependency(dep *TaskDependency) {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()
	dm.dependencies[dep.TaskType] = dep
}

// GetDependencies 获取任务的依赖列表
func (dm *DependencyManager) GetDependencies(taskType TaskType) []TaskType {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	if dep, exists := dm.dependencies[taskType]; exists {
		return dep.DependsOn
	}
	return nil
}

// UpdateTaskStatus 更新任务执行状态
func (dm *DependencyManager) UpdateTaskStatus(platform string, taskType TaskType, storeID int64, status string, err error) {
	key := fmt.Sprintf("%s:%s:%d", platform, taskType, storeID)

	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	if _, exists := dm.statuses[key]; !exists {
		dm.statuses[key] = &TaskExecutionStatus{
			TaskID:   key,
			TaskType: taskType,
			StoreID:  storeID,
		}
	}

	taskStatus := dm.statuses[key]
	taskStatus.mutex.Lock()
	taskStatus.LastRunTime = time.Now()
	taskStatus.LastStatus = status
	taskStatus.Error = err
	taskStatus.mutex.Unlock()
}

// CanExecute 检查任务是否可以执行(依赖是否满足)
func (dm *DependencyManager) CanExecute(ctx context.Context, platform string, taskType TaskType, storeID int64) (bool, error) {
	dependencies := dm.GetDependencies(taskType)
	if len(dependencies) == 0 {
		return true, nil // 没有依赖,可以直接执行
	}

	dm.logger.WithFields(logrus.Fields{
		"platform":     platform,
		"task_type":    taskType,
		"store_id":     storeID,
		"dependencies": dependencies,
	}).Debug("检查任务依赖")

	// 检查所有依赖任务的状态
	for _, depType := range dependencies {
		key := fmt.Sprintf("%s:%s:%d", platform, depType, storeID)

		dm.mutex.RLock()
		status, exists := dm.statuses[key]
		dm.mutex.RUnlock()

		if !exists {
			return false, fmt.Errorf("依赖任务 %s 尚未执行", depType)
		}

		status.mutex.RLock()
		lastStatus := status.LastStatus
		lastRunTime := status.LastRunTime
		status.mutex.RUnlock()

		// 检查依赖任务是否成功完成
		if lastStatus != "success" {
			return false, fmt.Errorf("依赖任务 %s 状态异常: %s", depType, lastStatus)
		}

		// 检查依赖任务是否在合理时间内完成
		dep := dm.dependencies[taskType]
		if time.Since(lastRunTime) > dep.WaitTimeout {
			return false, fmt.Errorf("依赖任务 %s 执行时间过久", depType)
		}
	}

	return true, nil
}

// WaitForDependencies 等待依赖任务完成
func (dm *DependencyManager) WaitForDependencies(ctx context.Context, platform string, taskType TaskType, storeID int64) error {
	dependencies := dm.GetDependencies(taskType)
	if len(dependencies) == 0 {
		return nil
	}

	dep := dm.dependencies[taskType]
	timeout := dep.WaitTimeout
	if timeout == 0 {
		timeout = 60 * time.Minute // 默认60分钟超时
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(600 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("等待依赖任务超时")
		case <-ticker.C:
			canExecute, err := dm.CanExecute(ctx, platform, taskType, storeID)
			if err != nil {
				dm.logger.WithError(err).Warn("依赖检查失败")
				continue
			}
			if canExecute {
				return nil
			}
		}
	}
}

// GetDefaultDependencies 获取默认的任务依赖配置
func GetDefaultDependencies() []*TaskDependency {
	return []*TaskDependency{
		{
			TaskType:     TaskTypePricing,
			DependsOn:    []TaskType{}, // 核价任务无依赖
			WaitTimeout:  0,
			RetryOnError: false,
		},
		{
			TaskType:     TaskTypeProductSync,
			DependsOn:    []TaskType{}, // 产品同步无依赖
			WaitTimeout:  0,
			RetryOnError: false,
		},
		{
			TaskType: TaskTypeInventory,
			//DependsOn: []TaskType{TaskTypeProductSync}, // 库存监控依赖产品同步
			DependsOn:    []TaskType{}, // 库存监控依赖产品同步
			WaitTimeout:  90 * time.Minute,
			RetryOnError: true,
		},
		{
			TaskType: TaskTypeActivity,
			//DependsOn: []TaskType{TaskTypeProductSync, TaskTypeInventory}, // 活动报名依赖产品同步和库存监控
			DependsOn:    []TaskType{}, // 活动报名依赖产品同步和库存监控
			WaitTimeout:  180 * time.Minute,
			RetryOnError: true,
		},
	}
}

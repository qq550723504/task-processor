package openai

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// ContextManager 管理OpenAI API调用的上下文和超时
type ContextManager struct {
	defaultTimeout time.Duration
	maxTimeout     time.Duration
	activeContexts map[string]*ContextInfo
	mutex          sync.RWMutex
	logger         *logrus.Entry
}

// ContextInfo 上下文信息
type ContextInfo struct {
	ID        string
	StartTime time.Time
	Timeout   time.Duration
	Cancel    context.CancelFunc
	TaskType  string
	Metadata  map[string]interface{}
}

// ContextConfig 上下文配置
type ContextConfig struct {
	DefaultTimeout time.Duration
	MaxTimeout     time.Duration
}

// NewContextManager 创建新的上下文管理器
func NewContextManager(config *ContextConfig) *ContextManager {
	return &ContextManager{
		defaultTimeout: config.DefaultTimeout,
		maxTimeout:     config.MaxTimeout,
		activeContexts: make(map[string]*ContextInfo),
		logger:         logrus.WithField("component", "OpenAIContextManager"),
	}
}

// CreateContext 创建带超时控制的上下文
func (cm *ContextManager) CreateContext(parent context.Context, taskType string, timeout time.Duration) (context.Context, string, error) {
	// 验证超时时间
	if timeout <= 0 {
		timeout = cm.defaultTimeout
	}
	if timeout > cm.maxTimeout {
		timeout = cm.maxTimeout
		cm.logger.Warnf("超时时间被限制为最大值: %v", cm.maxTimeout)
	}

	// 创建上下文
	ctx, cancel := context.WithTimeout(parent, timeout)
	
	// 生成唯一ID
	contextID := fmt.Sprintf("%s_%d", taskType, time.Now().UnixNano())
	
	// 创建上下文信息
	contextInfo := &ContextInfo{
		ID:        contextID,
		StartTime: time.Now(),
		Timeout:   timeout,
		Cancel:    cancel,
		TaskType:  taskType,
		Metadata:  make(map[string]interface{}),
	}

	// 注册上下文
	cm.mutex.Lock()
	cm.activeContexts[contextID] = contextInfo
	cm.mutex.Unlock()

	// 启动监控goroutine
	go cm.monitorContext(contextID)

	cm.logger.WithFields(logrus.Fields{
		"context_id": contextID,
		"task_type":  taskType,
		"timeout":    timeout,
	}).Debug("创建OpenAI API上下文")

	return ctx, contextID, nil
}

// ReleaseContext 释放上下文
func (cm *ContextManager) ReleaseContext(contextID string) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if contextInfo, exists := cm.activeContexts[contextID]; exists {
		// 取消上下文
		contextInfo.Cancel()
		
		// 记录执行时间
		duration := time.Since(contextInfo.StartTime)
		cm.logger.WithFields(logrus.Fields{
			"context_id": contextID,
			"task_type":  contextInfo.TaskType,
			"duration":   duration,
		}).Debug("释放OpenAI API上下文")

		// 从活跃列表中移除
		delete(cm.activeContexts, contextID)
	}
}

// monitorContext 监控上下文状态
func (cm *ContextManager) monitorContext(contextID string) {
	defer func() {
		if r := recover(); r != nil {
			cm.logger.Errorf("上下文监控panic: %v", r)
		}
	}()

	cm.mutex.RLock()
	contextInfo, exists := cm.activeContexts[contextID]
	cm.mutex.RUnlock()

	if !exists {
		return
	}

	// 等待上下文完成或超时
	ticker := time.NewTicker(time.Second * 10) // 每10秒检查一次
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cm.mutex.RLock()
			info, stillExists := cm.activeContexts[contextID]
			cm.mutex.RUnlock()

			if !stillExists {
				return // 上下文已被释放
			}

			// 检查是否接近超时
			elapsed := time.Since(info.StartTime)
			remaining := info.Timeout - elapsed

			if remaining < time.Second*30 { // 剩余30秒时警告
				cm.logger.WithFields(logrus.Fields{
					"context_id": contextID,
					"task_type":  info.TaskType,
					"elapsed":    elapsed,
					"remaining":  remaining,
				}).Warn("OpenAI API调用即将超时")
			}

			if remaining <= 0 {
				cm.logger.WithFields(logrus.Fields{
					"context_id": contextID,
					"task_type":  info.TaskType,
					"elapsed":    elapsed,
				}).Error("OpenAI API调用超时")
				return
			}

		case <-time.After(contextInfo.Timeout + time.Minute):
			// 超时后1分钟仍未清理，强制清理
			cm.ReleaseContext(contextID)
			return
		}
	}
}

// GetActiveContexts 获取活跃上下文列表
func (cm *ContextManager) GetActiveContexts() []ContextInfo {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	contexts := make([]ContextInfo, 0, len(cm.activeContexts))
	for _, info := range cm.activeContexts {
		contexts = append(contexts, *info)
	}

	return contexts
}

// CancelLongRunningContexts 取消长时间运行的上下文
func (cm *ContextManager) CancelLongRunningContexts(maxDuration time.Duration) int {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	cancelCount := 0
	now := time.Now()

	for contextID, info := range cm.activeContexts {
		if now.Sub(info.StartTime) > maxDuration {
			cm.logger.WithFields(logrus.Fields{
				"context_id": contextID,
				"task_type":  info.TaskType,
				"duration":   now.Sub(info.StartTime),
			}).Warn("取消长时间运行的OpenAI API上下文")

			info.Cancel()
			delete(cm.activeContexts, contextID)
			cancelCount++
		}
	}

	return cancelCount
}

// GetStats 获取上下文管理器统计信息
func (cm *ContextManager) GetStats() map[string]interface{} {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	stats := map[string]interface{}{
		"active_contexts":  len(cm.activeContexts),
		"default_timeout":  cm.defaultTimeout,
		"max_timeout":      cm.maxTimeout,
	}

	// 按任务类型统计
	taskTypes := make(map[string]int)
	for _, info := range cm.activeContexts {
		taskTypes[info.TaskType]++
	}
	stats["task_types"] = taskTypes

	return stats
}

// Cleanup 清理所有上下文
func (cm *ContextManager) Cleanup() {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	for contextID, info := range cm.activeContexts {
		cm.logger.WithFields(logrus.Fields{
			"context_id": contextID,
			"task_type":  info.TaskType,
		}).Info("清理OpenAI API上下文")

		info.Cancel()
	}

	cm.activeContexts = make(map[string]*ContextInfo)
	cm.logger.Info("所有OpenAI API上下文已清理")
}
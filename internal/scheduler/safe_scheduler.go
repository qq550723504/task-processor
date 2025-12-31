// Package scheduler 提供安全的调度器实现
package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"task-processor/internal/core/logger"
	"task-processor/internal/utils"

	"github.com/sirupsen/logrus"
)

// SafeScheduler 安全的调度器，使用统一的goroutine管理
type SafeScheduler struct {
	goroutineManager *utils.GoroutineManager
	logger           *logrus.Entry
	tasks            map[string]*ScheduledTask
	mutex            sync.RWMutex
	ctx              context.Context
	cancel           context.CancelFunc
}

// ScheduledTask 调度任务
type ScheduledTask struct {
	ID       string
	Name     string
	Interval time.Duration
	Fn       func(ctx context.Context) error
	Enabled  bool
}

// NewSafeScheduler 创建安全调度器
func NewSafeScheduler(ctx context.Context) *SafeScheduler {
	schedulerCtx, cancel := context.WithCancel(ctx)
	logger := logger.GetGlobalLogger("safe_scheduler")

	return &SafeScheduler{
		goroutineManager: utils.NewGoroutineManager(schedulerCtx, logger),
		logger:           logger,
		tasks:            make(map[string]*ScheduledTask),
		ctx:              schedulerCtx,
		cancel:           cancel,
	}
}

// AddTask 添加调度任务
func (s *SafeScheduler) AddTask(task *ScheduledTask) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.tasks[task.ID] = task
	s.logger.WithFields(logrus.Fields{
		"task_id":   task.ID,
		"task_name": task.Name,
		"interval":  task.Interval,
	}).Info("添加调度任务")
}

// RemoveTask 移除调度任务
func (s *SafeScheduler) RemoveTask(taskID string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if task, exists := s.tasks[taskID]; exists {
		delete(s.tasks, taskID)
		s.logger.WithFields(logrus.Fields{
			"task_id":   taskID,
			"task_name": task.Name,
		}).Info("移除调度任务")
	}
}

// Start 启动调度器
func (s *SafeScheduler) Start() error {
	s.mutex.RLock()
	tasks := make([]*ScheduledTask, 0, len(s.tasks))
	for _, task := range s.tasks {
		if task.Enabled {
			tasks = append(tasks, task)
		}
	}
	s.mutex.RUnlock()

	s.logger.Infof("启动调度器，共有 %d 个启用的任务", len(tasks))

	// 为每个任务启动一个周期性goroutine
	for _, task := range tasks {
		s.startTask(task)
	}

	return nil
}

// Stop 停止调度器
func (s *SafeScheduler) Stop() error {
	s.logger.Info("停止调度器")
	s.cancel()

	// 等待所有goroutine完成，最多等待30秒
	if err := s.goroutineManager.WaitWithTimeout(30 * time.Second); err != nil {
		s.logger.Warnf("等待goroutine完成超时: %v", err)
	}

	return nil
}

// GetStatus 获取调度器状态
func (s *SafeScheduler) GetStatus() map[string]interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	goroutineStatus := s.goroutineManager.GetStatus()

	return map[string]interface{}{
		"total_tasks":        len(s.tasks),
		"running_goroutines": s.goroutineManager.GetRunningCount(),
		"goroutine_status":   goroutineStatus,
		"tasks":              s.getTasksStatus(),
	}
}

// startTask 启动单个任务
func (s *SafeScheduler) startTask(task *ScheduledTask) {
	taskLogger := s.logger.WithFields(logrus.Fields{
		"task_id":   task.ID,
		"task_name": task.Name,
	})

	s.goroutineManager.StartPeriodic(task.ID, task.Interval, func(ctx context.Context) error {
		taskLogger.Debug("执行调度任务")

		start := time.Now()
		err := task.Fn(ctx)
		duration := time.Since(start)

		if err != nil {
			taskLogger.WithFields(logrus.Fields{
				"error":    err,
				"duration": duration,
			}).Error("调度任务执行失败")
			return err
		}

		taskLogger.WithField("duration", duration).Debug("调度任务执行成功")
		return nil
	})
}

// getTasksStatus 获取任务状态
func (s *SafeScheduler) getTasksStatus() []map[string]interface{} {
	var status []map[string]interface{}

	for _, task := range s.tasks {
		status = append(status, map[string]interface{}{
			"id":       task.ID,
			"name":     task.Name,
			"interval": task.Interval.String(),
			"enabled":  task.Enabled,
		})
	}

	return status
}

// EnableTask 启用任务
func (s *SafeScheduler) EnableTask(taskID string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if task, exists := s.tasks[taskID]; exists {
		if !task.Enabled {
			task.Enabled = true
			s.startTask(task)
			s.logger.WithField("task_id", taskID).Info("启用调度任务")
		}
		return nil
	}

	return fmt.Errorf("任务不存在: %s", taskID)
}

// DisableTask 禁用任务
func (s *SafeScheduler) DisableTask(taskID string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if task, exists := s.tasks[taskID]; exists {
		task.Enabled = false
		s.goroutineManager.Stop(taskID)
		s.logger.WithField("task_id", taskID).Info("禁用调度任务")
		return nil
	}

	return fmt.Errorf("任务不存在: %s", taskID)
}

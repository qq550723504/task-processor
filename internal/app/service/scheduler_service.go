// Package service 提供调度服务功能
package service

import (
	"context"

	"task-processor/internal/app/scheduler"
	"task-processor/internal/pkg/management"

	"github.com/sirupsen/logrus"
)

// SchedulerService 调度服务接口
// 负责管理所有周期性调度任务（核价、产品同步、库存同步、活动报名等）
type SchedulerService interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	GetStatus() map[string]any
}

// schedulerServiceImpl 调度服务实现
type schedulerServiceImpl struct {
	logger           *logrus.Logger
	managementClient *management.ClientManager
	schedulerManager *scheduler.Manager
	ctx              context.Context
	cancel           context.CancelFunc
	running          bool
}

// NewSchedulerService 创建调度服务
func NewSchedulerService(logger *logrus.Logger) SchedulerService {
	return &schedulerServiceImpl{
		logger: logger,
	}
}

// Start 启动调度服务
func (s *schedulerServiceImpl) Start(ctx context.Context) error {
	if s.running {
		return nil
	}

	s.logger.Info("🚀 开始启动调度服务...")

	// 创建上下文
	s.ctx, s.cancel = context.WithCancel(ctx)

	// 初始化资源
	if err := s.initializeResources(); err != nil {
		return err
	}

	// 启动所有调度任务
	if err := s.startScheduledTasks(); err != nil {
		return err
	}

	s.running = true
	s.logger.Info("✅ 调度服务启动完成")

	return nil
}

// Stop 停止调度服务
func (s *schedulerServiceImpl) Stop(ctx context.Context) error {
	if !s.running {
		return nil
	}

	s.logger.Info("🛑 开始停止调度服务...")

	// 停止调度器
	if s.schedulerManager != nil {
		s.schedulerManager.StopAll()
		s.logger.Info("✅ 调度器已停止")
	}

	// 取消上下文
	if s.cancel != nil {
		s.cancel()
	}

	s.running = false
	s.logger.Info("✅ 调度服务已停止")

	return nil
}

// GetStatus 获取调度服务状态
func (s *schedulerServiceImpl) GetStatus() map[string]any {
	status := map[string]any{
		"running": s.running,
	}

	if s.schedulerManager != nil {
		tasks := s.schedulerManager.ListTasks()
		taskStatus := make([]map[string]any, 0, len(tasks))
		for _, task := range tasks {
			taskStatus = append(taskStatus, map[string]any{
				"id":       task.GetID(),
				"type":     task.GetType(),
				"platform": task.GetPlatform(),
				"status":   task.GetStatus(),
				"interval": task.GetInterval().String(),
			})
		}
		status["tasks"] = taskStatus
		status["task_count"] = s.schedulerManager.GetTaskCount()
	}

	return status
}

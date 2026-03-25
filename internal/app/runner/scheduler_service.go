// Package runner 提供处理器和调度器的运行管理功能
package runner

import (
	"context"

	"task-processor/internal/app/scheduler"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/infra/rabbitmq"

	"github.com/sirupsen/logrus"
)

// SchedulerService 调度服务接口
type SchedulerService interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	GetStatus() map[string]any
}

// schedulerServiceImpl 调度服务实现
type schedulerServiceImpl struct {
	logger              *logrus.Logger
	managementClient    *management.ClientManager
	config              *config.Config
	amazonProcessor     amazonCrawler
	rabbitmqClient      *rabbitmq.Client
	temuFactoryCreator  TaskFactoryCreator
	sheinFactoryCreator TaskFactoryCreator
	schedulerManager    *scheduler.Manager
	ctx                 context.Context
	cancel              context.CancelFunc
	running             bool
}

// NewSchedulerService 创建调度服务
func NewSchedulerService(logger *logrus.Logger, managementClient *management.ClientManager, cfg *config.Config) SchedulerService {
	return &schedulerServiceImpl{
		logger:           logger,
		managementClient: managementClient,
		config:           cfg,
	}
}

// NewSchedulerServiceWithDependencies 创建调度服务并显式注入依赖。
func NewSchedulerServiceWithDependencies(
	logger *logrus.Logger,
	managementClient *management.ClientManager,
	cfg *config.Config,
	amazonProcessor amazonCrawler,
	rabbitmqClient *rabbitmq.Client,
	deps SchedulerDependencies,
) SchedulerService {
	return &schedulerServiceImpl{
		logger:              logger,
		managementClient:    managementClient,
		config:              cfg,
		amazonProcessor:     amazonProcessor,
		rabbitmqClient:      rabbitmqClient,
		temuFactoryCreator:  deps.TemuFactoryCreator,
		sheinFactoryCreator: deps.SheinFactoryCreator,
	}
}

// NewSchedulerServiceWithAmazon 创建调度服务（带 Amazon 处理器）。
func NewSchedulerServiceWithAmazon(
	logger *logrus.Logger,
	managementClient *management.ClientManager,
	cfg *config.Config,
	amazonProcessor amazonCrawler,
	rabbitmqClient *rabbitmq.Client,
) SchedulerService {
	return NewSchedulerServiceWithDependencies(
		logger,
		managementClient,
		cfg,
		amazonProcessor,
		rabbitmqClient,
		BuildDefaultSchedulerDependencies(managementClient, amazonProcessor, rabbitmqClient),
	)
}

// Start 启动调度服务
func (s *schedulerServiceImpl) Start(ctx context.Context) error {
	if s.running {
		return nil
	}

	s.logger.Info("开始启动调度服务")
	s.ctx, s.cancel = context.WithCancel(ctx)

	if err := s.initializeResources(); err != nil {
		return err
	}
	if err := s.startScheduledTasks(); err != nil {
		return err
	}

	s.running = true
	s.logger.Info("调度服务启动完成")
	return nil
}

// Stop 停止调度服务
func (s *schedulerServiceImpl) Stop(ctx context.Context) error {
	_ = ctx
	if !s.running {
		return nil
	}

	s.logger.Info("开始停止调度服务")

	if s.schedulerManager != nil {
		s.schedulerManager.StopAll()
		s.logger.Info("调度器已停止")
	}
	if s.cancel != nil {
		s.cancel()
	}

	s.running = false
	s.logger.Info("调度服务已停止")
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

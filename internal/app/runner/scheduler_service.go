package runner

import (
	"context"

	"task-processor/internal/app/scheduler"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/rabbitmq"

	"github.com/sirupsen/logrus"
)

type SchedulerService interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	GetStatus() map[string]any
}

type schedulerServiceImpl struct {
	logger              *logrus.Logger
	storeRuntime        schedulerStoreRuntime
	config              *config.Config
	rabbitmqClient      *rabbitmq.Client
	temuFactoryCreator  TaskFactoryCreator
	sheinFactoryCreator TaskFactoryCreator
	schedulerManager    *scheduler.Manager
	ctx                 context.Context
	cancel              context.CancelFunc
	running             bool
}

func NewSchedulerServiceWithDependencies(
	logger *logrus.Logger,
	runtimeProvider SchedulerRuntimeProvider,
	cfg *config.Config,
	rabbitmqClient *rabbitmq.Client,
	deps SchedulerDependencies,
) SchedulerService {
	return &schedulerServiceImpl{
		logger:              logger,
		storeRuntime:        schedulerStoreRuntimeAdapter{runtime: runtimeProvider},
		config:              cfg,
		rabbitmqClient:      rabbitmqClient,
		temuFactoryCreator:  deps.TemuFactoryCreator,
		sheinFactoryCreator: deps.SheinFactoryCreator,
	}
}

func (s *schedulerServiceImpl) Start(ctx context.Context) error {
	if s.running {
		return nil
	}

	s.logger.Info("start scheduler service")
	s.ctx, s.cancel = context.WithCancel(ctx)

	if err := s.initializeResources(); err != nil {
		return err
	}
	if err := s.startScheduledTasks(); err != nil {
		return err
	}

	s.running = true
	s.logger.Info("scheduler service started")
	return nil
}

func (s *schedulerServiceImpl) Stop(ctx context.Context) error {
	_ = ctx
	if !s.running {
		return nil
	}

	s.logger.Info("stop scheduler service")

	if s.schedulerManager != nil {
		s.schedulerManager.StopAll()
		s.logger.Info("scheduler manager stopped")
	}
	if s.cancel != nil {
		s.cancel()
	}

	s.running = false
	s.logger.Info("scheduler service stopped")
	return nil
}

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

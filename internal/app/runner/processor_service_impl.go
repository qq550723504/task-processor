// Package runner 提供处理器和调度器的运行管理功能
package runner

import (
	"context"
	"sync"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/core/errors"
	"task-processor/internal/core/lifecycle"
	"task-processor/internal/core/logger"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/infra/monitoring"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/shein/pipeline"
	"task-processor/internal/temu"

	"github.com/sirupsen/logrus"
)

// processorServiceImpl 处理器服务实现
type processorServiceImpl struct {
	logger           *logrus.Logger
	lifecycleManager lifecycle.LifecycleManager

	// 处理器组件
	temuProcessor    *temu.TemuProcessor
	sheinProcessor   *pipeline.SheinProcessor
	schedulerService SchedulerService

	// 监控组件
	metricsCollector *monitoring.MetricsCollector
	healthChecker    *monitoring.HealthChecker

	// 共享资源（通过依赖注入获取）
	managementClient        *management.ClientManager
	rawJSONDataClient       rawJSONDataClientProvider
	processorRuntime        processorRuntimeProvider
	schedulerRuntime        SchedulerRuntimeProvider
	schedulerFactoryRuntime schedulerFactoryRuntimeProvider
	crawlSource             crawlSource
	rabbitmqClient          *rabbitmq.Client
	temuProcessorCreator    TemuProcessorCreator
	sheinProcessorCreator   SheinProcessorCreator

	// 生命周期管理
	ctx     context.Context
	cancel  context.CancelFunc
	running bool
	mu      sync.RWMutex
}

// startSchedulerService 启动调度服务
func (s *processorServiceImpl) startSchedulerService(ctx context.Context, cfg *config.Config) error {
	log := logger.GetGlobalLogger("service.processor")
	log.Info("启动调度服务...")

	// 创建调度服务（通过依赖注入，不再使用全局状态）
	// Reuse the shared RabbitMQ client so scheduler-triggered fetches can use distributed crawling.
	if s.schedulerRuntime == nil {
		return errors.New(errors.ErrCodeSystem, "调度运行时未注入")
	}
	if s.schedulerFactoryRuntime == nil {
		return errors.New(errors.ErrCodeSystem, "调度工厂运行时未注入")
	}
	s.schedulerService = NewSchedulerServiceWithDependencies(
		s.logger,
		s.schedulerRuntime,
		cfg,
		s.rabbitmqClient,
		buildSchedulerDependencies(s.schedulerFactoryRuntime, cfg, s.crawlSource, s.rabbitmqClient),
	)

	// 启动调度服务
	if err := s.schedulerService.Start(ctx); err != nil {
		return errors.Wrap(err, errors.ErrCodeSystem, "调度服务启动失败")
	}

	log.Info("✅ 调度服务启动完成")
	return nil
}

// initializeMonitoring 初始化监控组件
func (s *processorServiceImpl) initializeMonitoring(cfg *config.Config) {
	log := logger.GetGlobalLogger("service.processor")
	log.Info("初始化监控组件...")

	// 创建指标收集器
	s.metricsCollector = monitoring.NewMetricsCollector(s.logger, 30*time.Second)

	// 创建健康检查器
	s.healthChecker = monitoring.NewHealthChecker(s.logger, 60*time.Second)

	// 注册健康检查
	s.registerHealthChecks(cfg)

	// 注册到生命周期管理器
	s.lifecycleManager.Register(s.metricsCollector)
	s.lifecycleManager.Register(s.healthChecker)

	// 启动业务指标收集
	go s.collectBusinessMetrics()

	log.Info("✅ 监控组件初始化完成")
}

// registerHealthChecks 注册健康检查
func (s *processorServiceImpl) registerHealthChecks(cfg *config.Config) {
	s.healthChecker.RegisterCheck(&ConfigHealthCheck{config: cfg})

	if s.processorRuntime != nil {
		s.healthChecker.RegisterCheck(&ProcessorRuntimeHealthCheck{
			runtime: s.processorRuntime,
		})
	}

	for _, module := range s.processorModules() {
		processor := module.get(s)
		if processor == nil {
			continue
		}
		s.healthChecker.RegisterCheck(&ProcessorHealthCheck{
			name:      module.name,
			processor: processor,
		})
	}
}

// collectBusinessMetrics 收集业务指标
func (s *processorServiceImpl) collectBusinessMetrics() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	log := logger.GetGlobalLogger("service.processor")

	for {
		select {
		case <-s.ctx.Done():
			log.Info("业务指标收集停止")
			return
		case <-ticker.C:
			s.updateBusinessMetrics()
		}
	}
}

// updateBusinessMetrics 更新业务指标
func (s *processorServiceImpl) updateBusinessMetrics() {
	for _, module := range s.processorModules() {
		processor := module.get(s)
		if processor == nil {
			continue
		}
		workerPool := processor.GetWorkerPool()
		if workerPool != nil {
			stats := workerPool.GetQueueStats()
			s.metricsCollector.SetGauge("queue_size", float64(stats.QueueSize),
				map[string]string{"platform": module.name}, "队列大小")
			s.metricsCollector.SetGauge("queue_usage_percent", stats.UsagePercent,
				map[string]string{"platform": module.name}, "队列使用率")
		}
	}

	s.metricsCollector.SetGauge("processor_running", func() float64 {
		if s.running {
			return 1
		}
		return 0
	}(), nil, "处理器运行状态")
}

// Package service 提供处理器服务实现
package service

import (
	"context"
	"sync"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/core/errors"
	"task-processor/internal/core/lifecycle"
	"task-processor/internal/domain/task"
	"task-processor/internal/infra/monitoring"
	"task-processor/internal/pkg/management"
	"task-processor/internal/platforms/shein/service/pipeline"
	"task-processor/internal/platforms/temu"

	"github.com/sirupsen/logrus"
)

// processorServiceImpl 处理器服务实现
type processorServiceImpl struct {
	logger           *logrus.Logger
	lifecycleManager *lifecycle.Manager

	// 处理器组件
	temuProcessor  *temu.TemuProcessor
	sheinProcessor *pipeline.SheinProcessor
	taskFetcher    *task.TaskFetcher
	pricingService PricingService

	// 监控组件
	metricsCollector *monitoring.MetricsCollector
	healthChecker    *monitoring.HealthChecker

	// 共享资源
	managementClient *management.ClientManager

	// 生命周期管理
	ctx     context.Context
	cancel  context.CancelFunc
	running bool
	mu      sync.RWMutex
}

// startTaskFetcher 启动任务获取器
func (s *processorServiceImpl) startTaskFetcher(cfg *config.Config) error {
	s.logger.Info("启动任务获取器...")

	// 收集所有平台的任务提交器
	submitters := make(map[string]task.TaskSubmitter)

	if s.temuProcessor != nil {
		submitters["temu"] = NewTaskSubmitterAdapter(s.temuProcessor, "temu", s.logger)
	}

	if s.sheinProcessor != nil {
		submitters["shein"] = NewTaskSubmitterAdapter(s.sheinProcessor, "shein", s.logger)
	}

	if len(submitters) == 0 {
		s.logger.Warn("没有可用的平台处理器，跳过任务获取器启动")
		return nil
	}

	// 创建任务获取器
	s.taskFetcher = task.NewUnifiedTaskFetcher(
		cfg,
		s.managementClient,
		submitters,
		s.logger,
	)

	// 注册到生命周期管理器
	s.lifecycleManager.Register(s.taskFetcher)

	s.logger.Info("✅ 任务获取器创建完成")
	return nil
}

// startPricingService 启动核价服务
func (s *processorServiceImpl) startPricingService(ctx context.Context, cfg *config.Config) error {
	s.logger.Info("启动核价服务...")

	// 设置全局资源
	SetGlobalConfig(cfg)

	// 创建核价服务
	s.pricingService = NewPricingService(s.logger)

	// 启动核价服务
	if err := s.pricingService.Start(ctx); err != nil {
		return errors.Wrap(err, errors.ErrCodeSystem, "核价服务启动失败")
	}

	s.logger.Info("✅ 核价服务启动完成")
	return nil
}

// initializeMonitoring 初始化监控组件
func (s *processorServiceImpl) initializeMonitoring(cfg *config.Config) error {
	s.logger.Info("初始化监控组件...")

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

	s.logger.Info("✅ 监控组件初始化完成")
	return nil
}

// registerHealthChecks 注册健康检查
func (s *processorServiceImpl) registerHealthChecks(cfg *config.Config) {
	// 注册配置健康检查
	s.healthChecker.RegisterCheck(&ConfigHealthCheck{config: cfg})

	// 注册管理客户端健康检查
	if s.managementClient != nil {
		s.healthChecker.RegisterCheck(&ManagementClientHealthCheck{
			client: s.managementClient,
		})
	}

	// 注册处理器健康检查
	if s.temuProcessor != nil {
		s.healthChecker.RegisterCheck(&ProcessorHealthCheck{
			name:      "temu",
			processor: s.temuProcessor,
		})
	}

	if s.sheinProcessor != nil {
		s.healthChecker.RegisterCheck(&ProcessorHealthCheck{
			name:      "shein",
			processor: s.sheinProcessor,
		})
	}
}

// collectBusinessMetrics 收集业务指标
func (s *processorServiceImpl) collectBusinessMetrics() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("业务指标收集停止")
			return
		case <-ticker.C:
			s.updateBusinessMetrics()
		}
	}
}

// updateBusinessMetrics 更新业务指标
func (s *processorServiceImpl) updateBusinessMetrics() {
	// 收集处理器状态指标
	if s.temuProcessor != nil {
		workerPool := s.temuProcessor.GetWorkerPool()
		if workerPool != nil {
			stats := workerPool.GetQueueStats()
			s.metricsCollector.SetGauge("queue_size", float64(stats.QueueSize),
				map[string]string{"platform": "temu"}, "队列大小")
			s.metricsCollector.SetGauge("queue_usage_percent", stats.UsagePercent,
				map[string]string{"platform": "temu"}, "队列使用率")
		}
	}

	if s.sheinProcessor != nil {
		workerPool := s.sheinProcessor.GetWorkerPool()
		if workerPool != nil {
			stats := workerPool.GetQueueStats()
			s.metricsCollector.SetGauge("queue_size", float64(stats.QueueSize),
				map[string]string{"platform": "shein"}, "队列大小")
			s.metricsCollector.SetGauge("queue_usage_percent", stats.UsagePercent,
				map[string]string{"platform": "shein"}, "队列使用率")
		}
	}

	// 收集系统运行状态
	s.metricsCollector.SetGauge("processor_running", func() float64 {
		if s.running {
			return 1
		}
		return 0
	}(), nil, "处理器运行状态")
}

// ConfigHealthCheck 配置健康检查
type ConfigHealthCheck struct {
	config *config.Config
}

func (c *ConfigHealthCheck) Name() string {
	return "config"
}

func (c *ConfigHealthCheck) Check(ctx context.Context) error {
	if c.config == nil {
		return errors.New(errors.ErrCodeConfig, "配置未加载")
	}

	// 验证关键配置项
	if c.config.Worker.Concurrency <= 0 {
		return errors.New(errors.ErrCodeConfig, "工作池并发数配置无效")
	}

	if c.config.Management.BaseURL == "" {
		return errors.New(errors.ErrCodeConfig, "管理系统URL未配置")
	}

	return nil
}

// ManagementClientHealthCheck 管理客户端健康检查
type ManagementClientHealthCheck struct {
	client *management.ClientManager
}

func (m *ManagementClientHealthCheck) Name() string {
	return "management_client"
}

func (m *ManagementClientHealthCheck) Check(ctx context.Context) error {
	if m.client == nil {
		return errors.New(errors.ErrCodeExternalAPI, "管理客户端未初始化")
	}

	// 这里可以添加实际的连接测试
	// 比如调用一个简单的API来验证连接
	return nil
}

// ProcessorHealthCheck 处理器健康检查
type ProcessorHealthCheck struct {
	name      string
	processor PlatformProcessor
}

func (p *ProcessorHealthCheck) Name() string {
	return "processor_" + p.name
}

func (p *ProcessorHealthCheck) Check(ctx context.Context) error {
	if p.processor == nil {
		return errors.Newf(errors.ErrCodeSystem, "%s处理器未初始化", p.name)
	}

	// 检查工作池状态
	workerPool := p.processor.GetWorkerPool()
	if workerPool == nil {
		return errors.Newf(errors.ErrCodeSystem, "%s处理器工作池未初始化", p.name)
	}

	// 检查队列状态
	stats := workerPool.GetQueueStats()
	if stats.UsagePercent > 95 {
		return errors.Newf(errors.ErrCodeResourceLimit, "%s处理器队列使用率过高: %.1f%%", p.name, stats.UsagePercent)
	}

	return nil
}

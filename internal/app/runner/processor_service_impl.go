// Package service 提供处理器服务实现
package runner

import (
	"context"
	"sync"
	"time"

	"task-processor/internal/app/task"
	"task-processor/internal/core/config"
	"task-processor/internal/core/errors"
	"task-processor/internal/core/lifecycle"
	"task-processor/internal/core/logger"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/infra/monitoring"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/platforms/shein/pipeline"
	"task-processor/internal/platforms/temu"

	"github.com/sirupsen/logrus"
)

// processorServiceImpl 处理器服务实现
type processorServiceImpl struct {
	logger           *logrus.Logger
	lifecycleManager lifecycle.LifecycleManager

	// 处理器组件
	temuProcessor    *temu.TemuProcessor
	sheinProcessor   *pipeline.SheinProcessor
	taskFetcher      *task.TaskFetcher
	schedulerService SchedulerService

	// 监控组件
	metricsCollector *monitoring.MetricsCollector
	healthChecker    *monitoring.HealthChecker

	// 共享资源（通过依赖注入获取）
	managementClient *management.ClientManager
	amazonProcessor  *amazon.AmazonProcessor

	// 生命周期管理
	ctx     context.Context
	cancel  context.CancelFunc
	running bool
	mu      sync.RWMutex
}

// startTaskFetcher 启动任务获取器
func (s *processorServiceImpl) startTaskFetcher(cfg *config.Config) error {
	log := logger.GetGlobalLogger("service.processor")
	log.Info("启动任务获取器...")

	// 收集所有平台的任务提交器
	submitters := make(map[string]task.TaskSubmitter)

	log.WithFields(map[string]interface{}{
		"temu_available":  s.temuProcessor != nil,
		"shein_available": s.sheinProcessor != nil,
	}).Info("检查处理器状态")

	if s.temuProcessor != nil {
		submitters["temu"] = task.NewTaskSubmitterAdapter(s.temuProcessor, "temu", s.logger)
		log.Info("✅ TEMU任务提交器已注册")
	}

	if s.sheinProcessor != nil {
		submitters["shein"] = task.NewTaskSubmitterAdapter(s.sheinProcessor, "shein", s.logger)
		log.Info("✅ SHEIN任务提交器已注册")
	}

	if len(submitters) == 0 {
		log.Warn("没有可用的平台处理器，跳过任务获取器启动")
		return nil
	}

	log.WithField("platforms", getMapKeys(submitters)).Info("创建任务获取器")

	// 创建任务获取器
	s.taskFetcher = task.NewUnifiedTaskFetcher(
		cfg,
		s.managementClient,
		submitters,
		s.logger,
	)

	// 注册到生命周期管理器
	s.lifecycleManager.Register(s.taskFetcher)

	log.Info("✅ 任务获取器创建完成")
	return nil
}

// getMapKeys 获取map的所有键
func getMapKeys(m map[string]task.TaskSubmitter) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// startSchedulerService 启动调度服务
func (s *processorServiceImpl) startSchedulerService(ctx context.Context, cfg *config.Config) error {
	log := logger.GetGlobalLogger("service.processor")
	log.Info("启动调度服务...")

	// 创建调度服务（通过依赖注入，不再使用全局状态）
	s.schedulerService = NewSchedulerServiceWithAmazon(s.logger, s.managementClient, cfg, s.amazonProcessor)

	// 启动调度服务
	if err := s.schedulerService.Start(ctx); err != nil {
		return errors.Wrap(err, errors.ErrCodeSystem, "调度服务启动失败")
	}

	log.Info("✅ 调度服务启动完成")
	return nil
}

// initializeMonitoring 初始化监控组件
func (s *processorServiceImpl) initializeMonitoring(cfg *config.Config) error {
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
	processor task.PlatformProcessor
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


// Package service 提供处理器服务实现
package service

import (
	"context"
	"sync"
	"time"

	"task-processor/internal/auth"
	"task-processor/internal/common/management"
	"task-processor/internal/core/config"
	"task-processor/internal/core/errors"
	"task-processor/internal/core/lifecycle"
	"task-processor/internal/monitoring"
	"task-processor/internal/platforms/shein"
	"task-processor/internal/platforms/temu"
	"task-processor/internal/task"

	"github.com/sirupsen/logrus"
)

// processorServiceImpl 处理器服务实现
type processorServiceImpl struct {
	logger           *logrus.Logger
	lifecycleManager *lifecycle.Manager

	// 处理器组件
	temuProcessor  *temu.TemuProcessor
	sheinProcessor *shein.SheinProcessor
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

// StartProcessors 启动所有处理器
func (s *processorServiceImpl) StartProcessors(ctx context.Context, cfg *config.Config, authClient *auth.ClientCredentialsAuthClient) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return errors.New(errors.ErrCodeSystem, "处理器已经在运行中")
	}

	s.logger.Info("🚀 开始启动任务处理器...")

	// 创建上下文
	s.ctx, s.cancel = context.WithCancel(ctx)

	// 初始化共享资源
	if err := s.initializeSharedResources(cfg, authClient); err != nil {
		return errors.Wrap(err, errors.ErrCodeSystem, "初始化共享资源失败")
	}

	// 启动处理器
	if err := s.startProcessors(ctx, cfg); err != nil {
		return errors.Wrap(err, errors.ErrCodeSystem, "启动处理器失败")
	}

	// 启动任务获取器
	if err := s.startTaskFetcher(cfg); err != nil {
		return errors.Wrap(err, errors.ErrCodeSystem, "启动任务获取器失败")
	}

	// 启动核价服务
	if err := s.startPricingService(ctx, cfg); err != nil {
		return errors.Wrap(err, errors.ErrCodeSystem, "启动核价服务失败")
	}

	// 初始化监控组件
	if err := s.initializeMonitoring(cfg); err != nil {
		return errors.Wrap(err, errors.ErrCodeSystem, "初始化监控组件失败")
	}

	// 启动所有组件
	if err := s.lifecycleManager.StartAll(s.ctx); err != nil {
		return errors.Wrap(err, errors.ErrCodeSystem, "启动组件失败")
	}

	s.running = true
	s.logger.Info("✅ 所有任务处理器启动完成")

	// 启动状态监控
	go s.startStatusMonitor()

	return nil
}

// StopProcessors 停止所有处理器
func (s *processorServiceImpl) StopProcessors() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		s.logger.Info("处理器服务未运行，无需停止")
		return nil
	}

	s.logger.Info("🛑 开始停止任务处理器...")

	// 取消上下文
	if s.cancel != nil {
		s.cancel()
	}

	// 使用生命周期管理器停止所有组件
	stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var lastError error

	// 停止生命周期管理器管理的组件
	if err := s.lifecycleManager.StopAll(stopCtx); err != nil {
		s.logger.Errorf("停止生命周期管理器组件失败: %v", err)
		lastError = err
	}

	// 停止其他处理器
	s.stopAllProcessors(stopCtx)

	// 停止核价服务
	if s.pricingService != nil {
		if err := s.pricingService.Stop(stopCtx); err != nil {
			s.logger.Errorf("停止核价服务失败: %v", err)
			lastError = err
		}
	}

	s.running = false

	if lastError != nil {
		s.logger.Warn("⚠️ 部分组件停止时发生错误，但主要组件已停止")
		return errors.Wrap(lastError, errors.ErrCodeSystem, "部分组件停止失败")
	}

	s.logger.Info("✅ 所有任务处理器已停止")
	return nil
}

// GetStatus 获取处理器状态
func (s *processorServiceImpl) GetStatus() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := map[string]any{
		"running": s.running,
		"processors": map[string]any{
			"temu":  s.temuProcessor != nil,
			"shein": s.sheinProcessor != nil,
		},
		"taskFetcher": s.taskFetcher != nil && s.taskFetcher.IsRunning(),
		"components":  s.lifecycleManager.GetComponentStatus(),
	}

	// 添加核价服务状态
	if s.pricingService != nil {
		status["pricingService"] = s.pricingService.GetStatus()
	}

	// 添加监控组件状态
	if s.metricsCollector != nil {
		status["metricsCollector"] = s.metricsCollector.IsRunning()
		// 添加当前指标
		status["metrics"] = s.metricsCollector.GetMetrics()
	}

	if s.healthChecker != nil {
		status["healthChecker"] = s.healthChecker.IsRunning()
	}

	return status
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

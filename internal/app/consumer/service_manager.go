// Package consumer 提供服务管理功能
package consumer

import (
	"context"
	"fmt"
	"sync"

	"task-processor/internal/core/config"
	"task-processor/internal/core/lifecycle"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/infra/worker"

	"github.com/sirupsen/logrus"
)

// ServiceManager 编排 messaging 包内所有子服务的生命周期。
// 它不持有子服务的具体类型，而是通过 lifecycle.LifecycleManager 统一管理。
type ServiceManager struct {
	config          *config.RabbitMQConfig
	logger          *logrus.Logger
	lifecycleMgr    lifecycle.LifecycleManager
	shutdownCoord   *ShutdownCoordinator
	rabbitmqService *RabbitMQService // 保留引用，用于 RegisterProcessor / GetClient

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	started bool
	mu      sync.RWMutex
}

// NewServiceManager 创建服务管理器。
func NewServiceManager(rabbitmqConfig *config.RabbitMQConfig, logger *logrus.Logger) (*ServiceManager, error) {
	if rabbitmqConfig == nil {
		return nil, fmt.Errorf("RabbitMQ配置为空")
	}

	logger.Info("初始化服务管理器...")

	rabbitmqService := NewRabbitMQService(rabbitmqConfig, logger)

	return &ServiceManager{
		config:          rabbitmqConfig,
		logger:          logger,
		lifecycleMgr:    lifecycle.NewLifecycleManager(logger),
		rabbitmqService: rabbitmqService,
	}, nil
}

// RegisterProcessor 注册任务处理器，必须在 Start 之前调用。
func (sm *ServiceManager) RegisterProcessor(platform string, processor worker.Processor) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.started {
		return fmt.Errorf("服务已启动，无法注册新的处理器")
	}
	return sm.rabbitmqService.RegisterProcessor(platform, processor)
}

// Start 初始化并启动所有子服务。
func (sm *ServiceManager) Start(ctx context.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.started {
		return fmt.Errorf("服务管理器已启动")
	}

	sm.logger.Info("启动服务管理器...")
	sm.ctx, sm.cancel = context.WithCancel(ctx)

	if err := sm.registerComponents(); err != nil {
		return fmt.Errorf("注册组件失败: %w", err)
	}

	if len(sm.config.Consumer.Queues) > 0 {
		sm.rabbitmqService.SetQueueConfigs(sm.config.Consumer.Queues)
	}

	if err := sm.lifecycleMgr.StartAll(sm.ctx); err != nil {
		return fmt.Errorf("启动组件失败: %w", err)
	}

	sm.wg.Add(1)
	go sm.shutdownCoord.HandleSignals(sm.ctx, &sm.wg, sm.cancel)

	sm.started = true
	sm.logger.Info("服务管理器启动完成")
	return nil
}

// registerComponents 构造各子服务并注册到 lifecycleMgr 和 shutdownCoord。
func (sm *ServiceManager) registerComponents() error {
	cfg := sm.config

	// 结果上报器
	reporterCfg := ReporterConfig{
		ReportURL:   cfg.ResultReporter.ReportURL,
		NodeID:      cfg.ResultReporter.NodeID,
		Timeout:     cfg.ResultReporter.Timeout,
		BufferSize:  cfg.ResultReporter.BufferSize,
		RetryConfig: cfg.ResultReporter.Retry,
	}
	reporter := NewResultReporter(reporterCfg, sm.logger)

	// 负载监控
	monitorCfg := rabbitmq.MonitorConfig{
		UpdateInterval: cfg.LoadMonitor.UpdateInterval,
		EnableCPU:      cfg.LoadMonitor.EnableCPU,
		EnableMemory:   cfg.LoadMonitor.EnableMemory,
		EnableTasks:    cfg.LoadMonitor.EnableTasks,
	}
	loadMonitor := rabbitmq.NewLoadMonitor(monitorCfg, sm.logger)

	// HTTP 服务器
	httpServer := NewHTTPServerManager(cfg, loadMonitor, sm.rabbitmqService, sm.logger)

	// 按启动顺序构建组件列表
	components := []lifecycle.Component{
		newReporterComponent(reporter),
		newLoadMonitorComponent(loadMonitor, sm.logger),
		newRabbitMQComponent(sm.rabbitmqService),
		newHTTPServerComponent(httpServer),
	}

	for _, c := range components {
		if err := sm.lifecycleMgr.Register(c); err != nil {
			return fmt.Errorf("注册组件 %s 失败: %w", c.Name(), err)
		}
	}

	sm.shutdownCoord = NewShutdownCoordinator(components, cfg.Node.ShutdownTimeout, sm.logger)
	return nil
}

// Stop 优雅停止所有服务。
func (sm *ServiceManager) Stop(ctx context.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if !sm.started {
		sm.logger.Info("服务管理器未启动，无需停止")
		return nil
	}

	if sm.shutdownCoord != nil {
		sm.shutdownCoord.GracefulShutdown(context.Background())
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		sm.wg.Wait()
	}()

	select {
	case <-done:
		sm.logger.Info("服务管理器停止完成")
	case <-ctx.Done():
		sm.logger.Warn("等待服务管理器停止超时")
		return fmt.Errorf("停止服务管理器超时")
	}

	sm.started = false
	return nil
}

// Wait 阻塞直到收到信号并完成关闭。
func (sm *ServiceManager) Wait() {
	sm.wg.Wait()
}

// IsStarted 返回服务管理器是否已启动。
func (sm *ServiceManager) IsStarted() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.started
}

// GetConfig 返回当前 RabbitMQ 配置。
func (sm *ServiceManager) GetConfig() *config.RabbitMQConfig {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.config
}

// GetStats 返回所有子服务的统计信息。
func (sm *ServiceManager) GetStats() map[string]any {
	status := sm.lifecycleMgr.GetStatus()
	stats := make(map[string]any, len(status))
	for name, s := range status {
		stats[name] = s
	}
	// 补充 rabbitmq 详细统计
	if sm.rabbitmqService != nil {
		stats["rabbitmq_detail"] = sm.rabbitmqService.GetStats()
	}
	return stats
}

// GetClient 返回 RabbitMQ 客户端，供外部注册器使用。
func (sm *ServiceManager) GetClient() *rabbitmq.Client {
	if sm.rabbitmqService == nil {
		return nil
	}
	return sm.rabbitmqService.GetClient()
}

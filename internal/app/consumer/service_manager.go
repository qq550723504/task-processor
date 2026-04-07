// Package consumer 提供服务管理功能
package consumer

import (
	"context"
	"fmt"
	"strings"
	"sync"

	apptask "task-processor/internal/app/task"
	"task-processor/internal/core/config"
	"task-processor/internal/core/lifecycle"
	"task-processor/internal/infra/clients/management/api"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/infra/worker"

	"github.com/sirupsen/logrus"
)

// ServiceManager 编排 messaging 包内所有子服务的生命周期。
// 它不持有子服务的具体类型，而是通过 lifecycle.LifecycleManager 统一管理。
type ServiceManager struct {
	config           *config.RabbitMQConfig
	logger           *logrus.Logger
	lifecycleMgr     lifecycle.LifecycleManager
	shutdownCoord    *ShutdownCoordinator
	rabbitmqService  *RabbitMQService // 保留引用，用于 RegisterProcessor / GetClient
	schedulerService SchedulerService // 可选，由外部通过 SetSchedulerService 注入

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

// SetSchedulerService 注入调度服务，必须在 Start 之前调用。
func (sm *ServiceManager) SetSchedulerService(svc SchedulerService) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.schedulerService = svc
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
	components := sm.buildManagedComponents()

	for _, c := range components {
		if err := sm.lifecycleMgr.Register(c); err != nil {
			return fmt.Errorf("注册组件 %s 失败: %w", c.Name(), err)
		}
	}

	sm.shutdownCoord = NewShutdownCoordinator(components, sm.config.Node.ShutdownTimeout, sm.logger)
	return nil
}

func (sm *ServiceManager) buildManagedComponents() []lifecycle.Component {
	reporter := sm.buildResultReporter()
	loadMonitor := sm.buildLoadMonitor()
	httpServer := sm.buildHTTPServerManager(loadMonitor)

	components := []lifecycle.Component{
		newReporterComponent(reporter),
		newLoadMonitorComponent(loadMonitor, sm.logger),
		newRabbitMQComponent(sm.rabbitmqService),
		newHTTPServerComponent(httpServer),
	}

	if sm.schedulerService != nil {
		components = append(components, newSchedulerComponent(sm.schedulerService))
		sm.logger.Info("调度服务已注入，将随服务管理器启动")
	}

	return components
}

func (sm *ServiceManager) buildResultReporter() *ResultReporter {
	cfg := sm.config
	if strings.TrimSpace(cfg.ResultReporter.ReportURL) == "" {
		sm.logger.Info("结果上报器未配置 reportURL，当前节点跳过异步结果上报，依赖任务状态直连回写")
		return nil
	}

	reporterCfg := ReporterConfig{
		ReportURL:   cfg.ResultReporter.ReportURL,
		NodeID:      cfg.ResultReporter.NodeID,
		Timeout:     cfg.ResultReporter.Timeout,
		BufferSize:  cfg.ResultReporter.BufferSize,
		RetryConfig: cfg.ResultReporter.Retry,
	}

	return NewResultReporter(reporterCfg, sm.logger)
}

func (sm *ServiceManager) buildLoadMonitor() *rabbitmq.LoadMonitor {
	cfg := sm.config
	monitorCfg := rabbitmq.MonitorConfig{
		UpdateInterval: cfg.LoadMonitor.UpdateInterval,
		EnableCPU:      cfg.LoadMonitor.EnableCPU,
		EnableMemory:   cfg.LoadMonitor.EnableMemory,
		EnableTasks:    cfg.LoadMonitor.EnableTasks,
	}

	return rabbitmq.NewLoadMonitor(monitorCfg, sm.logger)
}

func (sm *ServiceManager) buildHTTPServerManager(loadMonitor *rabbitmq.LoadMonitor) *HTTPServerManager {
	return NewHTTPServerManager(sm.config, loadMonitor, sm.rabbitmqService, sm.GetStats, sm.logger)
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

// SetStoreComponents 注入店铺亲和性所需的组件，必须在 Start 之前调用。
func (sm *ServiceManager) SetStoreComponents(
	storeAPI api.StoreAPI,
	ownedStores []int64,
	deduplicator *apptask.DeduplicationManager,
) {
	sm.rabbitmqService.SetComponents(nil, storeAPI, ownedStores, deduplicator)
}

// SetStoreAssignmentProvider injects a dynamic store-assignment resolver for dedicated queue nodes.
func (sm *ServiceManager) SetStoreAssignmentProvider(provider StoreAssignmentProvider) {
	sm.rabbitmqService.SetStoreAssignmentProvider(provider)
}

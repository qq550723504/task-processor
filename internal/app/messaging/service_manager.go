// Package messaging 提供服务管理功能
package messaging

import (
	"context"
	"fmt"
	"sync"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/infra/worker"

	"github.com/sirupsen/logrus"
)

// ServiceManager 服务管理器
type ServiceManager struct {
	config           *config.RabbitMQConfig
	configPath       string // 配置文件路径（用于统计信息）
	rabbitmqService  *RabbitMQService
	resultReporter   *ResultReporter
	loadMonitor      *rabbitmq.LoadMonitor
	httpServerManger *HTTPServerManager
	shutdownCoord    *ShutdownCoordinator
	logger           *logrus.Logger

	// 生命周期管理
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// 状态管理
	started bool
	mutex   sync.RWMutex
}

// NewServiceManager 创建服务管理器
func NewServiceManager(rabbitmqConfig *config.RabbitMQConfig, logger *logrus.Logger) (*ServiceManager, error) {
	// 验证RabbitMQ配置
	if rabbitmqConfig == nil {
		return nil, fmt.Errorf("RabbitMQ配置为空")
	}

	logger.Info("初始化服务管理器...")

	// 立即初始化RabbitMQ服务，以便注册处理器
	rabbitmqService := NewRabbitMQService(rabbitmqConfig, logger)

	return &ServiceManager{
		config:          rabbitmqConfig,
		configPath:      "", // 不再需要配置文件路径
		rabbitmqService: rabbitmqService,
		logger:          logger,
	}, nil
}

// RegisterProcessor 注册任务处理器
func (sm *ServiceManager) RegisterProcessor(platform string, processor worker.Processor) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if sm.started {
		return fmt.Errorf("服务已启动，无法注册新的处理器")
	}

	if sm.rabbitmqService == nil {
		return fmt.Errorf("RabbitMQ服务未初始化")
	}

	return sm.rabbitmqService.RegisterProcessor(platform, processor)
}

// Start 启动所有服务
func (sm *ServiceManager) Start(ctx context.Context) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if sm.started {
		return fmt.Errorf("服务管理器已启动")
	}

	sm.logger.Info("启动服务管理器...")

	sm.ctx, sm.cancel = context.WithCancel(ctx)

	// 1. 初始化所有服务
	if err := sm.initializeServices(); err != nil {
		return fmt.Errorf("初始化服务失败: %w", err)
	}

	// 2. 设置队列配置（如果配置文件中有定义）
	if len(sm.config.Consumer.Queues) > 0 {
		sm.rabbitmqService.SetQueueConfigs(sm.config.Consumer.Queues)
	}

	// 3. 启动结果上报器
	if err := sm.resultReporter.Start(sm.ctx); err != nil {
		return fmt.Errorf("启动结果上报器失败: %w", err)
	}

	// 4. 启动负载监控
	if err := sm.loadMonitor.Start(sm.ctx); err != nil {
		return fmt.Errorf("启动负载监控失败: %w", err)
	}

	// 5. 启动RabbitMQ服务
	if err := sm.rabbitmqService.Start(sm.ctx); err != nil {
		return fmt.Errorf("启动RabbitMQ服务失败: %w", err)
	}

	// 6. 启动HTTP服务器
	if err := sm.httpServerManger.Start(sm.ctx); err != nil {
		return fmt.Errorf("启动HTTP服务器失败: %w", err)
	}

	// 7. 启动信号监听
	sm.wg.Add(1)
	go sm.shutdownCoord.HandleSignals(sm.ctx, &sm.wg, sm.cancel)

	sm.started = true
	sm.logger.Info("服务管理器启动完成")
	return nil
}

// initializeServices 初始化所有服务
func (sm *ServiceManager) initializeServices() error {
	// 创建结果上报器配置
	reporterConfig := ReporterConfig{
		ReportURL:   sm.config.ResultReporter.ReportURL,
		NodeID:      sm.config.ResultReporter.NodeID,
		Timeout:     sm.config.ResultReporter.Timeout,
		BufferSize:  sm.config.ResultReporter.BufferSize,
		RetryConfig: sm.config.ResultReporter.Retry,
	}
	sm.resultReporter = NewResultReporter(reporterConfig, sm.logger)

	// 创建负载监控器配置
	monitorConfig := rabbitmq.MonitorConfig{
		UpdateInterval: sm.config.LoadMonitor.UpdateInterval,
		EnableCPU:      sm.config.LoadMonitor.EnableCPU,
		EnableMemory:   sm.config.LoadMonitor.EnableMemory,
		EnableTasks:    sm.config.LoadMonitor.EnableTasks,
	}
	sm.loadMonitor = rabbitmq.NewLoadMonitor(monitorConfig, sm.logger)

	// RabbitMQ服务已在构造函数中创建，这里不需要重复创建

	// 创建 HTTP 服务管理器
	sm.httpServerManger = NewHTTPServerManager(sm.config, sm.loadMonitor, sm.rabbitmqService, sm.logger)

	// 创建关闭协调器
	sm.shutdownCoord = NewShutdownCoordinator(
		sm.config,
		sm.rabbitmqService,
		sm.httpServerManger,
		sm.resultReporter,
		sm.loadMonitor,
		sm.logger,
	)

	sm.logger.Info("所有服务初始化完成")
	return nil
}

// Stop 停止服务管理器
func (sm *ServiceManager) Stop(ctx context.Context) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if !sm.started {
		sm.logger.Info("服务管理器未启动，无需停止")
		return nil
	}

	if sm.shutdownCoord != nil {
		sm.shutdownCoord.GracefulShutdown(context.Background())
	}

	// 等待所有goroutine完成
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

// IsStarted 检查是否已启动
func (sm *ServiceManager) IsStarted() bool {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return sm.started
}

// GetConfig 获取当前配置
func (sm *ServiceManager) GetConfig() *config.RabbitMQConfig {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return sm.config
}

// GetStats 获取所有统计信息
func (sm *ServiceManager) GetStats() map[string]interface{} {
	stats := make(map[string]interface{})

	if sm.loadMonitor != nil {
		stats["load"] = sm.loadMonitor.GetStats()
	}

	if sm.rabbitmqService != nil {
		stats["rabbitmq"] = sm.rabbitmqService.GetStats()
	}

	if sm.resultReporter != nil {
		stats["result_reporter"] = sm.resultReporter.GetStats()
	}

	return stats
}

// Wait 等待服务管理器完成（阻塞直到收到信号并完成关闭）
func (sm *ServiceManager) Wait() {
	sm.wg.Wait()
}

// GetClient 获取RabbitMQ客户端
func (sm *ServiceManager) GetClient() *rabbitmq.Client {
	if sm.rabbitmqService == nil {
		return nil
	}
	return sm.rabbitmqService.GetClient()
}

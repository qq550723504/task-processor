// Package messaging 提供服务管理功能
package messaging

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/infra/worker"

	"github.com/sirupsen/logrus"
)

// ServiceManager 服务管理器
type ServiceManager struct {
	config          *config.RabbitMQConfig
	configPath      string // 配置文件路径（用于统计信息）
	rabbitmqService *RabbitMQService
	resultReporter  *ResultReporter
	loadMonitor     *rabbitmq.LoadMonitor
	healthServer    *http.Server
	metricsServer   *http.Server
	logger          *logrus.Logger

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

	// 2. 启动结果上报器
	if err := sm.resultReporter.Start(sm.ctx); err != nil {
		return fmt.Errorf("启动结果上报器失败: %w", err)
	}

	// 3. 启动负载监控
	if err := sm.loadMonitor.Start(sm.ctx); err != nil {
		return fmt.Errorf("启动负载监控失败: %w", err)
	}

	// 4. 启动RabbitMQ服务
	if err := sm.rabbitmqService.Start(sm.ctx); err != nil {
		return fmt.Errorf("启动RabbitMQ服务失败: %w", err)
	}

	// 5. 启动HTTP服务器
	if err := sm.startHTTPServers(); err != nil {
		return fmt.Errorf("启动HTTP服务器失败: %w", err)
	}

	// 6. 启动信号监听
	sm.wg.Add(1)
	go sm.handleSignals()

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

	sm.logger.Info("所有服务初始化完成")
	return nil
}

// startHTTPServers 启动HTTP服务器
func (sm *ServiceManager) startHTTPServers() error {
	// 启动健康检查服务器
	sm.wg.Add(1)
	go sm.startHealthServer()

	// 启动指标服务器
	sm.wg.Add(1)
	go sm.startMetricsServer()

	return nil
}

// startHealthServer 启动健康检查服务器
func (sm *ServiceManager) startHealthServer() {
	defer sm.wg.Done()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", sm.handleHealth)
	mux.HandleFunc("/ready", sm.handleReady)

	sm.healthServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", sm.config.Node.HealthCheckPort),
		Handler: mux,
	}

	sm.logger.Infof("健康检查服务器启动在端口: %d", sm.config.Node.HealthCheckPort)

	if err := sm.healthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		sm.logger.Errorf("健康检查服务器错误: %v", err)
	}
}

// startMetricsServer 启动指标服务器
func (sm *ServiceManager) startMetricsServer() {
	defer sm.wg.Done()

	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", sm.handleMetrics)
	mux.HandleFunc("/stats", sm.handleStats)

	sm.metricsServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", sm.config.Node.MetricsPort),
		Handler: mux,
	}

	sm.logger.Infof("指标服务器启动在端口: %d", sm.config.Node.MetricsPort)

	if err := sm.metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		sm.logger.Errorf("指标服务器错误: %v", err)
	}
}

// handleHealth 处理健康检查请求
func (sm *ServiceManager) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := sm.loadMonitor.GetHealthStatus()

	w.Header().Set("Content-Type", "application/json")

	status := health["status"].(string)
	if status == "healthy" {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	// 简单的JSON响应
	fmt.Fprintf(w, `{"status":"%s","timestamp":"%s"}`,
		status, time.Now().Format(time.RFC3339))
}

// handleReady 处理就绪检查请求
func (sm *ServiceManager) handleReady(w http.ResponseWriter, r *http.Request) {
	ready := sm.rabbitmqService.IsConnected()

	w.Header().Set("Content-Type", "application/json")

	if ready {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"ready":true,"timestamp":"%s"}`, time.Now().Format(time.RFC3339))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, `{"ready":false,"timestamp":"%s"}`, time.Now().Format(time.RFC3339))
	}
}

// handleMetrics 处理指标请求
func (sm *ServiceManager) handleMetrics(w http.ResponseWriter, r *http.Request) {
	stats := sm.loadMonitor.GetStats()

	// 从指标收集器获取系统指标
	metricsCollector := sm.loadMonitor.GetMetricsCollector()
	systemMetrics := metricsCollector.GetMetrics()

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)

	// 任务处理指标
	fmt.Fprintf(w, "# HELP tasks_processed_total Total number of tasks processed\n")
	fmt.Fprintf(w, "# TYPE tasks_processed_total counter\n")
	fmt.Fprintf(w, "tasks_processed_total %d\n", stats.TasksProcessed)

	fmt.Fprintf(w, "# HELP tasks_succeeded_total Total number of tasks succeeded\n")
	fmt.Fprintf(w, "# TYPE tasks_succeeded_total counter\n")
	fmt.Fprintf(w, "tasks_succeeded_total %d\n", stats.TasksSucceeded)

	fmt.Fprintf(w, "# HELP tasks_failed_total Total number of tasks failed\n")
	fmt.Fprintf(w, "# TYPE tasks_failed_total counter\n")
	fmt.Fprintf(w, "tasks_failed_total %d\n", stats.TasksFailed)

	// 系统指标
	if goroutineMetric, ok := systemMetrics["system_goroutines_count"]; ok {
		fmt.Fprintf(w, "# HELP goroutine_count Current number of goroutines\n")
		fmt.Fprintf(w, "# TYPE goroutine_count gauge\n")
		fmt.Fprintf(w, "goroutine_count %.0f\n", goroutineMetric.Value)
	}

	if cpuMetric, ok := systemMetrics["system_cpu_cores"]; ok {
		fmt.Fprintf(w, "# HELP cpu_cores Number of CPU cores\n")
		fmt.Fprintf(w, "# TYPE cpu_cores gauge\n")
		fmt.Fprintf(w, "cpu_cores %.0f\n", cpuMetric.Value)
	}

	if heapMetric, ok := systemMetrics["system_memory_heap_bytes"]; ok {
		fmt.Fprintf(w, "# HELP memory_heap_bytes Heap memory usage in bytes\n")
		fmt.Fprintf(w, "# TYPE memory_heap_bytes gauge\n")
		fmt.Fprintf(w, "memory_heap_bytes %.0f\n", heapMetric.Value)
	}

	if sysMetric, ok := systemMetrics["system_memory_sys_bytes"]; ok {
		fmt.Fprintf(w, "# HELP memory_sys_bytes System memory usage in bytes\n")
		fmt.Fprintf(w, "# TYPE memory_sys_bytes gauge\n")
		fmt.Fprintf(w, "memory_sys_bytes %.0f\n", sysMetric.Value)
	}

	// RabbitMQ特定指标
	if avgTimeMetric, ok := systemMetrics["rabbitmq_avg_processing_time_seconds"]; ok {
		fmt.Fprintf(w, "# HELP rabbitmq_avg_processing_time_seconds Average task processing time\n")
		fmt.Fprintf(w, "# TYPE rabbitmq_avg_processing_time_seconds gauge\n")
		fmt.Fprintf(w, "rabbitmq_avg_processing_time_seconds %.3f\n", avgTimeMetric.Value)
	}
}

// handleStats 处理统计请求
func (sm *ServiceManager) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := make(map[string]interface{})

	// 负载统计
	stats["load"] = sm.loadMonitor.GetStats()

	// RabbitMQ统计
	stats["rabbitmq"] = sm.rabbitmqService.GetStats()

	// 结果上报器统计
	stats["result_reporter"] = sm.resultReporter.GetStats()

	// 节点信息
	stats["node"] = map[string]interface{}{
		"node_id":    sm.config.Node.NodeID,
		"started_at": time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// 简单的JSON序列化
	fmt.Fprintf(w, `{"timestamp":"%s","stats":%v}`,
		time.Now().Format(time.RFC3339), stats)
}

// handleSignals 处理系统信号
func (sm *ServiceManager) handleSignals() {
	defer sm.wg.Done()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		sm.logger.Infof("收到信号: %v，开始优雅关闭...", sig)
		sm.gracefulShutdown()
	case <-sm.ctx.Done():
		sm.logger.Info("上下文已取消，停止信号监听")
	}
}

// gracefulShutdown 优雅关闭
func (sm *ServiceManager) gracefulShutdown() {
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), sm.config.Node.ShutdownTimeout)
	defer shutdownCancel()

	sm.logger.Info("开始优雅关闭所有服务...")

	// 停止接收新任务
	if sm.rabbitmqService != nil {
		if err := sm.rabbitmqService.Stop(shutdownCtx); err != nil {
			sm.logger.Errorf("停止RabbitMQ服务失败: %v", err)
		}
	}

	// 停止HTTP服务器
	if sm.healthServer != nil {
		if err := sm.healthServer.Shutdown(shutdownCtx); err != nil {
			sm.logger.Errorf("停止健康检查服务器失败: %v", err)
		}
	}

	if sm.metricsServer != nil {
		if err := sm.metricsServer.Shutdown(shutdownCtx); err != nil {
			sm.logger.Errorf("停止指标服务器失败: %v", err)
		}
	}

	// 停止其他服务
	if sm.resultReporter != nil {
		if err := sm.resultReporter.Stop(shutdownCtx); err != nil {
			sm.logger.Errorf("停止结果上报器失败: %v", err)
		}
	}

	if sm.loadMonitor != nil {
		if err := sm.loadMonitor.Stop(shutdownCtx); err != nil {
			sm.logger.Errorf("停止负载监控失败: %v", err)
		}
	}

	// 取消主上下文
	if sm.cancel != nil {
		sm.cancel()
	}

	sm.logger.Info("优雅关闭完成")
}

// Stop 停止服务管理器
func (sm *ServiceManager) Stop(ctx context.Context) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if !sm.started {
		sm.logger.Info("服务管理器未启动，无需停止")
		return nil
	}

	sm.gracefulShutdown()

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

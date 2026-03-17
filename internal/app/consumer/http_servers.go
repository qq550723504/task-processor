package consumer

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/rabbitmq"

	"github.com/sirupsen/logrus"
)

// HTTPServerManager 管理健康检查和指标 HTTP 服务
type HTTPServerManager struct {
	config          *config.RabbitMQConfig
	loadMonitor     *rabbitmq.LoadMonitor
	rabbitmqService *RabbitMQService
	logger          *logrus.Logger

	healthServer  *http.Server
	metricsServer *http.Server

	wg sync.WaitGroup
}

// NewHTTPServerManager 创建 HTTPServerManager
func NewHTTPServerManager(
	cfg *config.RabbitMQConfig,
	loadMonitor *rabbitmq.LoadMonitor,
	rabbitmqService *RabbitMQService,
	logger *logrus.Logger,
) *HTTPServerManager {
	return &HTTPServerManager{
		config:          cfg,
		loadMonitor:     loadMonitor,
		rabbitmqService: rabbitmqService,
		logger:          logger,
	}
}

// Start 启动健康检查和指标服务器
func (h *HTTPServerManager) Start(ctx context.Context) error {
	// 启动健康检查服务器
	h.wg.Add(1)
	go h.startHealthServer()

	// 启动指标服务器
	h.wg.Add(1)
	go h.startMetricsServer()

	return nil
}

// Stop 优雅停止所有 HTTP 服务器
func (h *HTTPServerManager) Stop(ctx context.Context) error {
	// 停止健康检查服务器
	if h.healthServer != nil {
		if err := h.healthServer.Shutdown(ctx); err != nil {
			h.logger.Errorf("停止健康检查服务器失败: %v", err)
		}
	}

	// 停止指标服务器
	if h.metricsServer != nil {
		if err := h.metricsServer.Shutdown(ctx); err != nil {
			h.logger.Errorf("停止指标服务器失败: %v", err)
		}
	}

	// 等待所有 goroutine 完成
	h.wg.Wait()
	return nil
}

// startHealthServer 启动健康检查服务器
func (h *HTTPServerManager) startHealthServer() {
	defer h.wg.Done()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", h.handleHealth)
	mux.HandleFunc("/ready", h.handleReady)

	h.healthServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", h.config.Node.HealthCheckPort),
		Handler: mux,
	}
	runServer(h.healthServer, "健康检查服务器", h.config.Node.HealthCheckPort, h.logger)
}

// startMetricsServer 启动指标服务器
func (h *HTTPServerManager) startMetricsServer() {
	defer h.wg.Done()

	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", h.handleMetrics)
	mux.HandleFunc("/stats", h.handleStats)

	h.metricsServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", h.config.Node.MetricsPort),
		Handler: mux,
	}
	runServer(h.metricsServer, "指标服务器", h.config.Node.MetricsPort, h.logger)
}

// runServer 启动 HTTP 服务器并阻塞，统一处理日志和错误
func runServer(srv *http.Server, name string, port int, logger *logrus.Logger) {
	logger.Infof("%s启动在端口: %d", name, port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Errorf("%s错误: %v", name, err)
	}
}

// handleHealth 处理健康检查请求
func (h *HTTPServerManager) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := h.loadMonitor.GetHealthStatus()

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
func (h *HTTPServerManager) handleReady(w http.ResponseWriter, r *http.Request) {
	ready := h.rabbitmqService.IsConnected()

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
func (h *HTTPServerManager) handleMetrics(w http.ResponseWriter, r *http.Request) {
	stats := h.loadMonitor.GetStats()

	// 从指标收集器获取系统指标
	metricsCollector := h.loadMonitor.GetMetricsCollector()
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
func (h *HTTPServerManager) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := make(map[string]any)

	// 负载统计
	stats["load"] = h.loadMonitor.GetStats()

	// RabbitMQ统计
	stats["rabbitmq"] = h.rabbitmqService.GetStats()

	// 节点信息
	stats["node"] = map[string]any{
		"node_id":    h.config.Node.NodeID,
		"started_at": time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// 简单的JSON序列化
	fmt.Fprintf(w, `{"timestamp":"%s","stats":%v}`,
		time.Now().Format(time.RFC3339), stats)
}

package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	coremetrics "task-processor/internal/core/metrics"
	"time"

	"task-processor/internal/core/config"
	appmetrics "task-processor/internal/infra/metrics"
	"task-processor/internal/infra/rabbitmq"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

// HTTPServerManager 管理健康检查和指标 HTTP 服务
type HTTPServerManager struct {
	config          *config.RabbitMQConfig
	loadMonitor     *rabbitmq.LoadMonitor
	rabbitmqService *RabbitMQService
	statsProvider   func() map[string]any
	logger          *logrus.Logger
	consumerMetrics *appmetrics.ConsumerRegistry

	healthServer  *http.Server
	metricsServer *http.Server
	startedAt     time.Time

	wg sync.WaitGroup
}

// NewHTTPServerManager 创建 HTTPServerManager
func NewHTTPServerManager(
	cfg *config.RabbitMQConfig,
	loadMonitor *rabbitmq.LoadMonitor,
	rabbitmqService *RabbitMQService,
	statsProvider func() map[string]any,
	logger *logrus.Logger,
) *HTTPServerManager {
	return &HTTPServerManager{
		config:          cfg,
		loadMonitor:     loadMonitor,
		rabbitmqService: rabbitmqService,
		statsProvider:   statsProvider,
		logger:          logger,
		consumerMetrics: appmetrics.NewConsumerRegistry(),
		startedAt:       time.Now(),
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
	mux.Handle("/metrics", h.metricsHandler())
	mux.HandleFunc("/stats", h.handleStats)

	h.metricsServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", h.config.Node.MetricsPort),
		Handler: mux,
	}
	runServer(h.metricsServer, "指标服务器", h.config.Node.MetricsPort, h.logger)
}

func (h *HTTPServerManager) metricsHandler() http.Handler {
	handler := promhttp.HandlerFor(h.consumerMetrics.Registry(), promhttp.HandlerOpts{})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.refreshMetricsSnapshot()
		handler.ServeHTTP(w, r)
	})
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
	connected := h.rabbitmqService.IsConnected()
	consumerReady := h.rabbitmqService.HasHealthyRequiredConsumers()
	ready := connected && consumerReady

	w.Header().Set("Content-Type", "application/json")

	status := health["status"].(string)
	httpStatus := http.StatusOK
	if !ready {
		status = "degraded"
	} else if status != "healthy" {
		httpStatus = http.StatusServiceUnavailable
	}

	response := map[string]any{
		"status":                    status,
		"ready":                     ready,
		"connected":                 connected,
		"consumer_ready":            consumerReady,
		"timestamp":                 time.Now().Format(time.RFC3339),
		"health":                    health,
		"unhealthy_required_queues": h.rabbitmqService.GetUnhealthyRequiredQueues(),
		"node": map[string]any{
			"node_id":      h.config.Node.NodeID,
			"role":         h.config.Node.NormalizedRole(),
			"health_port":  h.config.Node.HealthCheckPort,
			"metrics_port": h.config.Node.MetricsPort,
			"started_at":   h.startedAt.Format(time.RFC3339),
		},
	}
	if h.statsProvider != nil {
		response["service"] = h.statsProvider()
	}

	w.WriteHeader(httpStatus)
	_ = json.NewEncoder(w).Encode(response)
}

// handleReady 处理就绪检查请求
func (h *HTTPServerManager) handleReady(w http.ResponseWriter, r *http.Request) {
	connected := h.rabbitmqService.IsConnected()
	consumerReady := h.rabbitmqService.HasHealthyRequiredConsumers()
	ready := connected && consumerReady

	w.Header().Set("Content-Type", "application/json")

	response := map[string]any{
		"ready":                     ready,
		"connected":                 connected,
		"consumer_ready":            consumerReady,
		"unhealthy_required_queues": h.rabbitmqService.GetUnhealthyRequiredQueues(),
		"timestamp":                 time.Now().Format(time.RFC3339),
		"node": map[string]any{
			"node_id": h.config.Node.NodeID,
			"role":    h.config.Node.NormalizedRole(),
		},
	}
	if h.statsProvider != nil {
		response["service"] = h.statsProvider()
	}

	if ready {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	_ = json.NewEncoder(w).Encode(response)
}

func (h *HTTPServerManager) refreshMetricsSnapshot() {
	if h.consumerMetrics == nil {
		return
	}

	var stats rabbitmq.LoadStats
	systemMetrics := make(map[string]float64)
	if h.loadMonitor != nil {
		stats = h.loadMonitor.GetStats()
		if collector := h.loadMonitor.GetMetricsCollector(); collector != nil {
			for name, metric := range collector.GetMetrics() {
				systemMetrics[name] = metric.Value
			}
		}
	}

	h.consumerMetrics.UpdateConsumerSnapshot(appmetrics.ConsumerSnapshot{
		Load:   stats,
		Task:   coremetrics.GlobalTaskMetrics().GetSnapshot(),
		Shein:  coremetrics.GlobalSheinMetrics().GetSnapshot(),
		System: systemMetrics,
	})
}

// handleStats 处理统计请求
func (h *HTTPServerManager) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := make(map[string]any)
	limit := parseStatsLimit(r, 10)
	view := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("view")))
	reason := normalizeStatsReason(r.URL.Query().Get("reason"))
	format := normalizeStatsFormat(r.URL.Query().Get("format"))

	// 负载统计
	stats["load"] = h.loadMonitor.GetStats()

	// RabbitMQ统计
	stats["rabbitmq"] = h.rabbitmqService.GetStats()

	// 节点信息
	stats["node"] = map[string]any{
		"node_id":    h.config.Node.NodeID,
		"role":       h.config.Node.NormalizedRole(),
		"started_at": h.startedAt.Format(time.RFC3339),
	}
	if h.statsProvider != nil {
		stats["service"] = h.statsProvider()
	}
	applyStatsView(stats, view, reason, limit)
	responseStats := stats
	if format == "compact" {
		responseStats = buildCompactStats(stats)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"timestamp": time.Now().Format(time.RFC3339),
		"query": map[string]any{
			"view":   normalizeStatsView(view),
			"reason": reason,
			"limit":  limit,
			"format": format,
		},
		"stats": responseStats,
	})
}

func parseStatsLimit(r *http.Request, defaultLimit int) int {
	if r == nil {
		return defaultLimit
	}
	raw := strings.TrimSpace(r.URL.Query().Get("limit"))
	if raw == "" {
		return defaultLimit
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 {
		return defaultLimit
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func normalizeStatsView(view string) string {
	switch strings.ToLower(strings.TrimSpace(view)) {
	case "success":
		return "success"
	case "problem":
		return "problem"
	default:
		return "all"
	}
}

func normalizeStatsReason(reason string) string {
	switch strings.ToLower(strings.TrimSpace(reason)) {
	case "auth_expired":
		return "auth_expired"
	case "cookie_load_failed":
		return "cookie_load_failed"
	case "daily_limit_reached":
		return "daily_limit_reached"
	case "shelf_quota_exhausted":
		return "shelf_quota_exhausted"
	case "draft_saved_validation_failed":
		return "draft_saved_validation_failed"
	case "sku_duplicated":
		return "sku_duplicated"
	case "filter_rule_rejected":
		return "filter_rule_rejected"
	case "retryable_failure":
		return "retryable_failure"
	case "non_retryable_failure":
		return "non_retryable_failure"
	default:
		return ""
	}
}

func normalizeStatsFormat(format string) string {
	if strings.EqualFold(strings.TrimSpace(format), "compact") {
		return "compact"
	}
	return "full"
}

func applyStatsView(stats map[string]any, view, reason string, limit int) {
	serviceStats, ok := stats["service"].(map[string]any)
	if !ok {
		return
	}

	sheinStats, ok := serviceStats["shein_metrics"].(map[string]any)
	if !ok {
		return
	}

	var selected any
	switch normalizeStatsView(view) {
	case "success":
		if stores, ok := sheinStats["top_success_stores"]; ok {
			selected = stores
		}
	case "problem":
		if stores, ok := sheinStats["top_problem_stores"]; ok {
			selected = stores
		}
	default:
		if stores, ok := sheinStats["top_stores"]; ok {
			selected = stores
		}
	}

	if reason != "" {
		selected = filterStatsSliceByReason(selected, reason)
	}
	sheinStats["stores"] = trimStatsSlice(selected, limit)
	sheinStats["top_stores"] = trimStatsSlice(sheinStats["top_stores"], limit)
	sheinStats["top_success_stores"] = trimStatsSlice(sheinStats["top_success_stores"], limit)
	sheinStats["top_problem_stores"] = trimStatsSlice(sheinStats["top_problem_stores"], limit)
}

func trimStatsSlice(value any, limit int) any {
	if limit <= 0 {
		return value
	}

	switch items := value.(type) {
	case []coremetrics.SheinStoreStatsSnapshot:
		if len(items) <= limit {
			return items
		}
		return items[:limit]
	case []any:
		if len(items) <= limit {
			return items
		}
		return items[:limit]
	default:
		return value
	}
}

func filterStatsSliceByReason(value any, reason string) any {
	items, ok := value.([]coremetrics.SheinStoreStatsSnapshot)
	if !ok || reason == "" {
		return value
	}

	filtered := make([]coremetrics.SheinStoreStatsSnapshot, 0, len(items))
	for _, item := range items {
		if matchStatsReason(item, reason) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func matchStatsReason(item coremetrics.SheinStoreStatsSnapshot, reason string) bool {
	switch reason {
	case "auth_expired":
		return item.AuthExpiredCount > 0
	case "cookie_load_failed":
		return item.CookieLoadFailedCount > 0
	case "daily_limit_reached":
		return item.DailyLimitReachedCount > 0
	case "shelf_quota_exhausted":
		return item.ShelfQuotaExhaustedCount > 0
	case "draft_saved_validation_failed":
		return item.DraftSavedValidationCount > 0
	case "sku_duplicated":
		return item.SkuDuplicatedCount > 0
	case "filter_rule_rejected":
		return item.FilterRuleRejectedCount > 0
	case "retryable_failure":
		return item.RetryableFailureCount > 0
	case "non_retryable_failure":
		return item.NonRetryableFailureCount > 0
	default:
		return true
	}
}

func buildCompactStats(stats map[string]any) map[string]any {
	compact := make(map[string]any)

	if node, ok := stats["node"]; ok {
		compact["node"] = node
	}

	if load, ok := stats["load"].(map[string]any); ok {
		compact["load"] = map[string]any{
			"tasks_processed": load["TasksProcessed"],
			"tasks_succeeded": load["TasksSucceeded"],
			"tasks_failed":    load["TasksFailed"],
		}
	} else if load != nil {
		compact["load"] = load
	}

	serviceStats, ok := stats["service"].(map[string]any)
	if !ok {
		return compact
	}

	summary := make(map[string]any)
	if taskMetrics, ok := serviceStats["task_metrics"].(map[string]any); ok {
		summary["task_metrics"] = map[string]any{
			"processing_count":        taskMetrics["processing_count"],
			"completed_count":         taskMetrics["completed_count"],
			"failed_count":            taskMetrics["failed_count"],
			"requeued_count":          taskMetrics["requeued_count"],
			"average_wait_seconds":    taskMetrics["average_wait_seconds"],
			"average_process_seconds": taskMetrics["average_process_seconds"],
		}
	}

	if sheinMetrics, ok := serviceStats["shein_metrics"].(map[string]any); ok {
		summary["shein_metrics"] = map[string]any{
			"published_count":             sheinMetrics["published_count"],
			"paused_count":                sheinMetrics["paused_count"],
			"draft_count":                 sheinMetrics["draft_count"],
			"terminated_count":            sheinMetrics["terminated_count"],
			"auth_expired_count":          sheinMetrics["auth_expired_count"],
			"cookie_load_failed_count":    sheinMetrics["cookie_load_failed_count"],
			"daily_limit_reached_count":   sheinMetrics["daily_limit_reached_count"],
			"shelf_quota_exhausted_count": sheinMetrics["shelf_quota_exhausted_count"],
			"sku_duplicated_count":        sheinMetrics["sku_duplicated_count"],
			"stores":                      sheinMetrics["stores"],
		}
	}

	compact["summary"] = summary
	return compact
}

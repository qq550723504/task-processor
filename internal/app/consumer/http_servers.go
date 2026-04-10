package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"task-processor/internal/core/metrics"
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
	statsProvider   func() map[string]any
	logger          *logrus.Logger

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

// handleMetrics 处理指标请求
func (h *HTTPServerManager) handleMetrics(w http.ResponseWriter, r *http.Request) {
	stats := h.loadMonitor.GetStats()
	taskMetrics := metrics.GlobalTaskMetrics().GetSnapshot()
	sheinMetrics := metrics.GlobalSheinMetrics().GetSnapshot()

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

	fmt.Fprintf(w, "# HELP listing_tasks_processing_total Total number of listing tasks entered processing\n")
	fmt.Fprintf(w, "# TYPE listing_tasks_processing_total counter\n")
	fmt.Fprintf(w, "listing_tasks_processing_total %d\n", taskMetrics.ProcessingCount)

	fmt.Fprintf(w, "# HELP listing_tasks_completed_total Total number of listing tasks completed\n")
	fmt.Fprintf(w, "# TYPE listing_tasks_completed_total counter\n")
	fmt.Fprintf(w, "listing_tasks_completed_total %d\n", taskMetrics.CompletedCount)

	fmt.Fprintf(w, "# HELP listing_tasks_failed_total Total number of listing tasks failed\n")
	fmt.Fprintf(w, "# TYPE listing_tasks_failed_total counter\n")
	fmt.Fprintf(w, "listing_tasks_failed_total %d\n", taskMetrics.FailedCount)

	fmt.Fprintf(w, "# HELP listing_tasks_requeued_total Total number of listing tasks requeued\n")
	fmt.Fprintf(w, "# TYPE listing_tasks_requeued_total counter\n")
	fmt.Fprintf(w, "listing_tasks_requeued_total %d\n", taskMetrics.RequeuedCount)

	fmt.Fprintf(w, "# HELP listing_tasks_wait_seconds_avg Average wait time for listing tasks in seconds\n")
	fmt.Fprintf(w, "# TYPE listing_tasks_wait_seconds_avg gauge\n")
	fmt.Fprintf(w, "listing_tasks_wait_seconds_avg %.3f\n", metrics.GlobalTaskMetrics().GetAverageWaitTime().Seconds())

	fmt.Fprintf(w, "# HELP listing_tasks_process_seconds_avg Average processing time for listing tasks in seconds\n")
	fmt.Fprintf(w, "# TYPE listing_tasks_process_seconds_avg gauge\n")
	fmt.Fprintf(w, "listing_tasks_process_seconds_avg %.3f\n", metrics.GlobalTaskMetrics().GetAverageProcessTime().Seconds())

	fmt.Fprintf(w, "# HELP listing_tasks_priority_high_total Total number of high-priority listing tasks\n")
	fmt.Fprintf(w, "# TYPE listing_tasks_priority_high_total counter\n")
	fmt.Fprintf(w, "listing_tasks_priority_high_total %d\n", taskMetrics.HighPriorityCount)

	fmt.Fprintf(w, "# HELP listing_tasks_priority_medium_total Total number of medium-priority listing tasks\n")
	fmt.Fprintf(w, "# TYPE listing_tasks_priority_medium_total counter\n")
	fmt.Fprintf(w, "listing_tasks_priority_medium_total %d\n", taskMetrics.MediumPriorityCount)

	fmt.Fprintf(w, "# HELP listing_tasks_priority_low_total Total number of low-priority listing tasks\n")
	fmt.Fprintf(w, "# TYPE listing_tasks_priority_low_total counter\n")
	fmt.Fprintf(w, "listing_tasks_priority_low_total %d\n", taskMetrics.LowPriorityCount)

	fmt.Fprintf(w, "# HELP shein_tasks_published_total Total number of SHEIN tasks published successfully\n")
	fmt.Fprintf(w, "# TYPE shein_tasks_published_total counter\n")
	fmt.Fprintf(w, "shein_tasks_published_total %d\n", sheinMetrics.PublishedCount)

	fmt.Fprintf(w, "# HELP shein_tasks_paused_total Total number of SHEIN tasks paused\n")
	fmt.Fprintf(w, "# TYPE shein_tasks_paused_total counter\n")
	fmt.Fprintf(w, "shein_tasks_paused_total %d\n", sheinMetrics.PausedCount)

	fmt.Fprintf(w, "# HELP shein_tasks_draft_total Total number of SHEIN tasks moved to draft\n")
	fmt.Fprintf(w, "# TYPE shein_tasks_draft_total counter\n")
	fmt.Fprintf(w, "shein_tasks_draft_total %d\n", sheinMetrics.DraftCount)

	fmt.Fprintf(w, "# HELP shein_tasks_terminated_total Total number of SHEIN tasks terminated\n")
	fmt.Fprintf(w, "# TYPE shein_tasks_terminated_total counter\n")
	fmt.Fprintf(w, "shein_tasks_terminated_total %d\n", sheinMetrics.TerminatedCount)

	fmt.Fprintf(w, "# HELP shein_reason_auth_expired_total Total number of SHEIN auth expired events\n")
	fmt.Fprintf(w, "# TYPE shein_reason_auth_expired_total counter\n")
	fmt.Fprintf(w, "shein_reason_auth_expired_total %d\n", sheinMetrics.AuthExpiredCount)

	fmt.Fprintf(w, "# HELP shein_reason_cookie_load_failed_total Total number of SHEIN cookie load failures\n")
	fmt.Fprintf(w, "# TYPE shein_reason_cookie_load_failed_total counter\n")
	fmt.Fprintf(w, "shein_reason_cookie_load_failed_total %d\n", sheinMetrics.CookieLoadFailedCount)

	fmt.Fprintf(w, "# HELP shein_reason_daily_limit_reached_total Total number of SHEIN daily limit reached events\n")
	fmt.Fprintf(w, "# TYPE shein_reason_daily_limit_reached_total counter\n")
	fmt.Fprintf(w, "shein_reason_daily_limit_reached_total %d\n", sheinMetrics.DailyLimitReachedCount)

	fmt.Fprintf(w, "# HELP shein_reason_shelf_quota_exhausted_total Total number of SHEIN shelf quota exhausted events\n")
	fmt.Fprintf(w, "# TYPE shein_reason_shelf_quota_exhausted_total counter\n")
	fmt.Fprintf(w, "shein_reason_shelf_quota_exhausted_total %d\n", sheinMetrics.ShelfQuotaExhaustedCount)

	fmt.Fprintf(w, "# HELP shein_reason_draft_saved_validation_failed_total Total number of SHEIN draft-saved validation failures\n")
	fmt.Fprintf(w, "# TYPE shein_reason_draft_saved_validation_failed_total counter\n")
	fmt.Fprintf(w, "shein_reason_draft_saved_validation_failed_total %d\n", sheinMetrics.DraftSavedValidationCount)

	fmt.Fprintf(w, "# HELP shein_reason_sku_duplicated_total Total number of SHEIN duplicated SKU events\n")
	fmt.Fprintf(w, "# TYPE shein_reason_sku_duplicated_total counter\n")
	fmt.Fprintf(w, "shein_reason_sku_duplicated_total %d\n", sheinMetrics.SkuDuplicatedCount)

	fmt.Fprintf(w, "# HELP shein_reason_filter_rule_rejected_total Total number of SHEIN filter-rule rejections\n")
	fmt.Fprintf(w, "# TYPE shein_reason_filter_rule_rejected_total counter\n")
	fmt.Fprintf(w, "shein_reason_filter_rule_rejected_total %d\n", sheinMetrics.FilterRuleRejectedCount)

	fmt.Fprintf(w, "# HELP shein_reason_retryable_failure_total Total number of SHEIN retryable failures\n")
	fmt.Fprintf(w, "# TYPE shein_reason_retryable_failure_total counter\n")
	fmt.Fprintf(w, "shein_reason_retryable_failure_total %d\n", sheinMetrics.RetryableFailureCount)

	fmt.Fprintf(w, "# HELP shein_reason_non_retryable_failure_total Total number of SHEIN non-retryable failures\n")
	fmt.Fprintf(w, "# TYPE shein_reason_non_retryable_failure_total counter\n")
	fmt.Fprintf(w, "shein_reason_non_retryable_failure_total %d\n", sheinMetrics.NonRetryableFailureCount)

	writeSheinTopStoreMetrics(w, "shein_top_problem_store_problem_events", "Top SHEIN problem stores by problem events", sheinMetrics.TopProblemStores, func(item metrics.SheinStoreStatsSnapshot) int64 {
		return item.ProblemEvents
	})
	writeSheinTopStoreMetrics(w, "shein_top_problem_store_auth_expired_total", "Top SHEIN problem stores by auth expired events", sheinMetrics.TopAuthExpiredStores, func(item metrics.SheinStoreStatsSnapshot) int64 {
		return item.AuthExpiredCount
	})
	writeSheinTopStoreMetrics(w, "shein_top_problem_store_cookie_load_failed_total", "Top SHEIN problem stores by cookie load failures", sheinMetrics.TopCookieLoadFailedStores, func(item metrics.SheinStoreStatsSnapshot) int64 {
		return item.CookieLoadFailedCount
	})
	writeSheinTopStoreMetrics(w, "shein_top_problem_store_daily_limit_reached_total", "Top SHEIN problem stores by daily limit reached events", sheinMetrics.TopDailyLimitStores, func(item metrics.SheinStoreStatsSnapshot) int64 {
		return item.DailyLimitReachedCount
	})
	writeSheinTopStoreMetrics(w, "shein_top_problem_store_shelf_quota_exhausted_total", "Top SHEIN problem stores by shelf quota exhausted events", sheinMetrics.TopShelfQuotaStores, func(item metrics.SheinStoreStatsSnapshot) int64 {
		return item.ShelfQuotaExhaustedCount
	})
	writeSheinTopStoreMetrics(w, "shein_top_problem_store_draft_saved_validation_failed_total", "Top SHEIN problem stores by draft validation failures", sheinMetrics.TopDraftValidationStores, func(item metrics.SheinStoreStatsSnapshot) int64 {
		return item.DraftSavedValidationCount
	})
	writeSheinTopStoreMetrics(w, "shein_top_problem_store_sku_duplicated_total", "Top SHEIN problem stores by duplicated SKU events", sheinMetrics.TopSkuDuplicatedStores, func(item metrics.SheinStoreStatsSnapshot) int64 {
		return item.SkuDuplicatedCount
	})
	writeSheinTopStoreMetrics(w, "shein_top_problem_store_filter_rule_rejected_total", "Top SHEIN problem stores by filter-rule rejected events", sheinMetrics.TopFilterRejectedStores, func(item metrics.SheinStoreStatsSnapshot) int64 {
		return item.FilterRuleRejectedCount
	})
	writeSheinTopStoreMetrics(w, "shein_top_problem_store_retryable_failure_total", "Top SHEIN problem stores by retryable failures", sheinMetrics.TopRetryableFailureStores, func(item metrics.SheinStoreStatsSnapshot) int64 {
		return item.RetryableFailureCount
	})
	writeSheinTopStoreMetrics(w, "shein_top_problem_store_non_retryable_failure_total", "Top SHEIN problem stores by non-retryable failures", sheinMetrics.TopNonRetryableStores, func(item metrics.SheinStoreStatsSnapshot) int64 {
		return item.NonRetryableFailureCount
	})
	writeSheinTopStoreMetrics(w, "shein_top_success_store_published_total", "Top SHEIN success stores by published tasks", sheinMetrics.TopSuccessStores, func(item metrics.SheinStoreStatsSnapshot) int64 {
		return item.PublishedCount
	})

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

func writeSheinTopStoreMetrics(w http.ResponseWriter, metricName, help string, stores []metrics.SheinStoreStatsSnapshot, valueFn func(metrics.SheinStoreStatsSnapshot) int64) {
	fmt.Fprintf(w, "# HELP %s %s\n", metricName, help)
	fmt.Fprintf(w, "# TYPE %s gauge\n", metricName)
	for idx, store := range stores {
		value := valueFn(store)
		if value <= 0 {
			continue
		}
		fmt.Fprintf(
			w,
			"%s{rank=\"%d\",tenant_id=\"%d\",store_id=\"%d\"} %d\n",
			metricName,
			idx+1,
			store.TenantID,
			store.StoreID,
			value,
		)
	}
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
	case []metrics.SheinStoreStatsSnapshot:
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
	items, ok := value.([]metrics.SheinStoreStatsSnapshot)
	if !ok || reason == "" {
		return value
	}

	filtered := make([]metrics.SheinStoreStatsSnapshot, 0, len(items))
	for _, item := range items {
		if matchStatsReason(item, reason) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func matchStatsReason(item metrics.SheinStoreStatsSnapshot, reason string) bool {
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

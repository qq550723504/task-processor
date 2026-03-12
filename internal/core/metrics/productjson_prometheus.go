// Package utils 提供 Prometheus 指标导出工具
package utils

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// PrometheusMetrics Prometheus 指标集合
type PrometheusMetrics struct {
	// LLM 调用指标
	llmCallsTotal   *prometheus.CounterVec
	llmSuccessTotal *prometheus.CounterVec
	llmFailureTotal *prometheus.CounterVec
	llmDuration     *prometheus.HistogramVec
	llmCostTotal    prometheus.Counter

	// 任务处理指标
	taskProcessedTotal *prometheus.CounterVec
	taskDuration       prometheus.Histogram

	// 质量评分指标
	qualityScore prometheus.Histogram
	imageScore   prometheus.Histogram
	textScore    prometheus.Histogram
	scrapedScore prometheus.Histogram

	// 策略使用指标
	strategyUsageTotal    *prometheus.CounterVec
	strategySelectedTotal *prometheus.CounterVec

	// 验证指标
	validationIssuesTotal *prometheus.CounterVec
	validImagesTotal      prometheus.Counter
	invalidImagesTotal    *prometheus.CounterVec
	totalImagesTotal      prometheus.Counter

	// 队列指标
	queueLength prometheus.Gauge

	// 缓存指标
	cacheHitsTotal   *prometheus.CounterVec
	cacheMissesTotal *prometheus.CounterVec
	cacheSize        prometheus.Gauge
	cacheOperations  *prometheus.CounterVec

	// API 请求指标
	apiRequestsTotal *prometheus.CounterVec
	apiDuration      *prometheus.HistogramVec
	apiStatusCodes   *prometheus.CounterVec
}

var (
	promMetrics     *PrometheusMetrics
	promMetricsOnce sync.Once
)

// NewPrometheusMetrics 创建 Prometheus 指标集合
func NewPrometheusMetrics(namespace string) *PrometheusMetrics {
	if namespace == "" {
		namespace = "product_json_generator"
	}

	return &PrometheusMetrics{
		// LLM 调用指标
		llmCallsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "llm_calls_total",
				Help:      "Total number of LLM API calls",
			},
			[]string{"client"},
		),
		llmSuccessTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "llm_success_total",
				Help:      "Total number of successful LLM API calls",
			},
			[]string{"client"},
		),
		llmFailureTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "llm_failure_total",
				Help:      "Total number of failed LLM API calls",
			},
			[]string{"client"},
		),
		llmDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "llm_duration_seconds",
				Help:      "LLM API call duration in seconds",
				Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 30},
			},
			[]string{"client"},
		),
		llmCostTotal: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "llm_cost_total",
				Help:      "Total cost of LLM API calls in USD",
			},
		),

		// 任务处理指标
		taskProcessedTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "task_processed_total",
				Help:      "Total number of processed tasks",
			},
			[]string{"status"},
		),
		taskDuration: promauto.NewHistogram(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "task_duration_seconds",
				Help:      "Task processing duration in seconds",
				Buckets:   []float64{1, 5, 10, 30, 60, 120, 300},
			},
		),

		// 质量评分指标
		qualityScore: promauto.NewHistogram(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "quality_score",
				Help:      "Overall quality score distribution (0-100)",
				Buckets:   []float64{0, 20, 40, 50, 60, 80, 100},
			},
		),
		imageScore: promauto.NewHistogram(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "image_score",
				Help:      "Image quality score distribution (0-100)",
				Buckets:   []float64{0, 20, 40, 60, 80, 100},
			},
		),
		textScore: promauto.NewHistogram(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "text_score",
				Help:      "Text quality score distribution (0-100)",
				Buckets:   []float64{0, 20, 40, 60, 80, 100},
			},
		),
		scrapedScore: promauto.NewHistogram(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "scraped_score",
				Help:      "Scraped data quality score distribution (0-100)",
				Buckets:   []float64{0, 20, 40, 60, 80, 100},
			},
		),

		// 策略使用指标
		strategyUsageTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "strategy_usage_total",
				Help:      "Total number of strategy selections",
			},
			[]string{"strategy"},
		),
		strategySelectedTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "strategy_selected_total",
				Help:      "Total number of times each strategy was selected",
			},
			[]string{"strategy"},
		),

		// 验证指标
		validationIssuesTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "validation_issues_total",
				Help:      "Total number of validation issues by severity",
			},
			[]string{"severity", "field"},
		),
		validImagesTotal: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "valid_images_total",
				Help:      "Total number of valid images",
			},
		),
		invalidImagesTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "invalid_images_total",
				Help:      "Total number of invalid images by reason",
			},
			[]string{"reason"},
		),
		totalImagesTotal: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "total_images_total",
				Help:      "Total number of images validated",
			},
		),

		// 队列指标
		queueLength: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "queue_length",
				Help:      "Current queue length",
			},
		),

		// 缓存指标
		cacheHitsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "cache_hits_total",
				Help:      "Total number of cache hits",
			},
			[]string{"cache_type"},
		),
		cacheMissesTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "cache_misses_total",
				Help:      "Total number of cache misses",
			},
			[]string{"cache_type"},
		),
		cacheSize: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "cache_size",
				Help:      "Current number of items in cache",
			},
		),
		cacheOperations: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "cache_operations_total",
				Help:      "Total number of cache operations",
			},
			[]string{"operation", "cache_type"},
		),

		// API 请求指标
		apiRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "api_requests_total",
				Help:      "Total number of API requests",
			},
			[]string{"method", "path", "status"},
		),
		apiDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "api_duration_seconds",
				Help:      "API request duration in seconds",
				Buckets:   []float64{0.001, 0.01, 0.1, 0.5, 1, 2, 5},
			},
			[]string{"method", "path"},
		),
		apiStatusCodes: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "api_status_codes_total",
				Help:      "Total number of API responses by status code",
			},
			[]string{"code"},
		),
	}
}

// GetPrometheusMetrics 获取全局 Prometheus 指标实例
func GetPrometheusMetrics() *PrometheusMetrics {
	promMetricsOnce.Do(func() {
		promMetrics = NewPrometheusMetrics("product_json_generator")
	})
	return promMetrics
}

// PrometheusMetricsCollector Prometheus 指标收集器（实现 MetricsCollector 接口）
type PrometheusMetricsCollector struct {
	metrics *PrometheusMetrics
}

// NewPrometheusMetricsCollector 创建 Prometheus 指标收集器
func NewPrometheusMetricsCollector(metrics *PrometheusMetrics) MetricsCollector {
	if metrics == nil {
		metrics = GetPrometheusMetrics()
	}
	return &PrometheusMetricsCollector{
		metrics: metrics,
	}
}

// RecordLLMCall 记录 LLM 调用
func (p *PrometheusMetricsCollector) RecordLLMCall(client string, success bool, duration time.Duration, cost float64) {
	p.metrics.llmCallsTotal.WithLabelValues(client).Inc()

	if success {
		p.metrics.llmSuccessTotal.WithLabelValues(client).Inc()
	} else {
		p.metrics.llmFailureTotal.WithLabelValues(client).Inc()
	}

	p.metrics.llmDuration.WithLabelValues(client).Observe(duration.Seconds())
	p.metrics.llmCostTotal.Add(cost)
}

// RecordTaskProcessing 记录任务处理
func (p *PrometheusMetricsCollector) RecordTaskProcessing(status string, duration time.Duration) {
	p.metrics.taskProcessedTotal.WithLabelValues(status).Inc()
	p.metrics.taskDuration.Observe(duration.Seconds())
}

// RecordQualityScore 记录质量评分
func (p *PrometheusMetricsCollector) RecordQualityScore(score float64) {
	p.metrics.qualityScore.Observe(score)
}

// RecordImageScore 记录图片质量评分
func (p *PrometheusMetricsCollector) RecordImageScore(score float64) {
	p.metrics.imageScore.Observe(score)
}

// RecordTextScore 记录文本质量评分
func (p *PrometheusMetricsCollector) RecordTextScore(score float64) {
	p.metrics.textScore.Observe(score)
}

// RecordScrapedScore 记录抓取数据质量评分
func (p *PrometheusMetricsCollector) RecordScrapedScore(score float64) {
	p.metrics.scrapedScore.Observe(score)
}

// RecordStrategy 记录策略选择
func (p *PrometheusMetricsCollector) RecordStrategy(strategy string) {
	p.metrics.strategyUsageTotal.WithLabelValues(strategy).Inc()
	p.metrics.strategySelectedTotal.WithLabelValues(strategy).Inc()
}

// RecordValidationIssue 记录验证问题
func (p *PrometheusMetricsCollector) RecordValidationIssue(severity, field string) {
	p.metrics.validationIssuesTotal.WithLabelValues(severity, field).Inc()
}

// RecordImageValidation 记录图片验证结果
func (p *PrometheusMetricsCollector) RecordImageValidation(validCount, invalidCount int, invalidReasons map[string]int) {
	p.metrics.totalImagesTotal.Add(float64(validCount + invalidCount))
	p.metrics.validImagesTotal.Add(float64(validCount))

	for reason, count := range invalidReasons {
		p.metrics.invalidImagesTotal.WithLabelValues(reason).Add(float64(count))
	}
}

// RecordQueueLength 记录队列长度
func (p *PrometheusMetricsCollector) RecordQueueLength(length int) {
	p.metrics.queueLength.Set(float64(length))
}

// RecordAPIRequest 记录 API 请求
func (p *PrometheusMetricsCollector) RecordAPIRequest(method, path string, statusCode int, duration time.Duration) {
	status := statusCodeToString(statusCode)

	p.metrics.apiRequestsTotal.WithLabelValues(method, path, status).Inc()
	p.metrics.apiDuration.WithLabelValues(method, path).Observe(duration.Seconds())
	p.metrics.apiStatusCodes.WithLabelValues(status).Inc()
}

// RecordCacheHit 记录缓存命中
func (p *PrometheusMetricsCollector) RecordCacheHit(cacheType string) {
	p.metrics.cacheHitsTotal.WithLabelValues(cacheType).Inc()
}

// RecordCacheMiss 记录缓存未命中
func (p *PrometheusMetricsCollector) RecordCacheMiss(cacheType string) {
	p.metrics.cacheMissesTotal.WithLabelValues(cacheType).Inc()
}

// RecordCacheOperation 记录缓存操作
func (p *PrometheusMetricsCollector) RecordCacheOperation(operation, cacheType string) {
	p.metrics.cacheOperations.WithLabelValues(operation, cacheType).Inc()
}

// UpdateCacheSize 更新缓存大小
func (p *PrometheusMetricsCollector) UpdateCacheSize(size int) {
	p.metrics.cacheSize.Set(float64(size))
}

// statusCodeToString 将状态码转换为字符串
func statusCodeToString(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "2xx"
	case code >= 300 && code < 400:
		return "3xx"
	case code >= 400 && code < 500:
		return "4xx"
	case code >= 500:
		return "5xx"
	default:
		return "unknown"
	}
}

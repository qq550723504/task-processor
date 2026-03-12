// Package utils 提供 Prometheus 指标工具
package utils

import (
	"sync"
	"time"
)

// MetricsCollector 指标收集器接口
type MetricsCollector interface {
	// RecordLLMCall 记录 LLM 调用
	RecordLLMCall(client string, success bool, duration time.Duration, cost float64)
	// RecordTaskProcessing 记录任务处理
	RecordTaskProcessing(status string, duration time.Duration)
	// RecordQualityScore 记录质量评分
	RecordQualityScore(score float64)
	// RecordImageScore 记录图片质量评分
	RecordImageScore(score float64)
	// RecordTextScore 记录文本质量评分
	RecordTextScore(score float64)
	// RecordScrapedScore 记录抓取数据质量评分
	RecordScrapedScore(score float64)
	// RecordStrategy 记录策略选择
	RecordStrategy(strategy string)
	// RecordValidationIssue 记录验证问题
	RecordValidationIssue(severity, field string)
	// RecordImageValidation 记录图片验证结果
	RecordImageValidation(validCount, invalidCount int, invalidReasons map[string]int)
	// RecordQueueLength 记录队列长度
	RecordQueueLength(length int)
	// RecordAPIRequest 记录 API 请求
	RecordAPIRequest(method, path string, statusCode int, duration time.Duration)
	// RecordCacheHit 记录缓存命中
	RecordCacheHit(cacheType string)
	// RecordCacheMiss 记录缓存未命中
	RecordCacheMiss(cacheType string)
	// RecordCacheOperation 记录缓存操作
	RecordCacheOperation(operation, cacheType string)
	// UpdateCacheSize 更新缓存大小
	UpdateCacheSize(size int)
}

// metricsCollector 指标收集器实现
type metricsCollector struct {
	mu sync.RWMutex

	// LLM 调用指标
	llmCalls     map[string]uint64
	llmSuccesses map[string]uint64
	llmFailures  map[string]uint64
	llmDurations map[string][]time.Duration
	llmTotalCost float64

	// 任务处理指标
	taskProcessed map[string]uint64
	taskDurations []time.Duration

	// 质量评分指标
	qualityScores []float64
	imageScores   []float64
	textScores    []float64
	scrapedScores []float64

	// 策略选择指标
	strategyUsage map[string]uint64

	// 验证指标
	validationIssues map[string]map[string]uint64 // severity -> field -> count
	validImages      uint64
	invalidImages    map[string]uint64 // reason -> count
	totalImages      uint64

	// 队列指标
	queueLength int

	// API 请求指标
	apiRequests    map[string]uint64
	apiDurations   map[string][]time.Duration
	apiStatusCodes map[int]uint64

	// 缓存指标
	cacheHits       map[string]uint64            // cacheType -> count
	cacheMisses     map[string]uint64            // cacheType -> count
	cacheOperations map[string]map[string]uint64 // operation -> cacheType -> count
	cacheSize       int
}

// NewMetricsCollector 创建新的指标收集器
func NewMetricsCollector() MetricsCollector {
	return &metricsCollector{
		llmCalls:         make(map[string]uint64),
		llmSuccesses:     make(map[string]uint64),
		llmFailures:      make(map[string]uint64),
		llmDurations:     make(map[string][]time.Duration),
		taskProcessed:    make(map[string]uint64),
		taskDurations:    make([]time.Duration, 0),
		qualityScores:    make([]float64, 0),
		imageScores:      make([]float64, 0),
		textScores:       make([]float64, 0),
		scrapedScores:    make([]float64, 0),
		strategyUsage:    make(map[string]uint64),
		validationIssues: make(map[string]map[string]uint64),
		invalidImages:    make(map[string]uint64),
		apiRequests:      make(map[string]uint64),
		apiDurations:     make(map[string][]time.Duration),
		apiStatusCodes:   make(map[int]uint64),
		cacheHits:        make(map[string]uint64),
		cacheMisses:      make(map[string]uint64),
		cacheOperations:  make(map[string]map[string]uint64),
	}
}

// RecordLLMCall 记录 LLM 调用
func (m *metricsCollector) RecordLLMCall(client string, success bool, duration time.Duration, cost float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.llmCalls[client]++
	if success {
		m.llmSuccesses[client]++
	} else {
		m.llmFailures[client]++
	}

	m.llmDurations[client] = append(m.llmDurations[client], duration)
	m.llmTotalCost += cost
}

// RecordTaskProcessing 记录任务处理
func (m *metricsCollector) RecordTaskProcessing(status string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.taskProcessed[status]++
	m.taskDurations = append(m.taskDurations, duration)
}

// RecordQualityScore 记录质量评分
func (m *metricsCollector) RecordQualityScore(score float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.qualityScores = append(m.qualityScores, score)
}

// RecordImageScore 记录图片质量评分
func (m *metricsCollector) RecordImageScore(score float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.imageScores = append(m.imageScores, score)
}

// RecordTextScore 记录文本质量评分
func (m *metricsCollector) RecordTextScore(score float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.textScores = append(m.textScores, score)
}

// RecordScrapedScore 记录抓取数据质量评分
func (m *metricsCollector) RecordScrapedScore(score float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.scrapedScores = append(m.scrapedScores, score)
}

// RecordStrategy 记录策略选择
func (m *metricsCollector) RecordStrategy(strategy string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.strategyUsage[strategy]++
}

// RecordValidationIssue 记录验证问题
func (m *metricsCollector) RecordValidationIssue(severity, field string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.validationIssues[severity] == nil {
		m.validationIssues[severity] = make(map[string]uint64)
	}
	m.validationIssues[severity][field]++
}

// RecordImageValidation 记录图片验证结果
func (m *metricsCollector) RecordImageValidation(validCount, invalidCount int, invalidReasons map[string]int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalImages += uint64(validCount + invalidCount)
	m.validImages += uint64(validCount)

	for reason, count := range invalidReasons {
		m.invalidImages[reason] += uint64(count)
	}
}

// RecordQueueLength 记录队列长度
func (m *metricsCollector) RecordQueueLength(length int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.queueLength = length
}

// RecordAPIRequest 记录 API 请求
func (m *metricsCollector) RecordAPIRequest(method, path string, statusCode int, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := method + " " + path
	m.apiRequests[key]++
	m.apiDurations[key] = append(m.apiDurations[key], duration)
	m.apiStatusCodes[statusCode]++
}

// GetLLMMetrics 获取 LLM 指标
func (m *metricsCollector) GetLLMMetrics() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics := make(map[string]interface{})
	metrics["calls"] = m.llmCalls
	metrics["successes"] = m.llmSuccesses
	metrics["failures"] = m.llmFailures
	metrics["total_cost"] = m.llmTotalCost

	// 计算平均延迟
	avgDurations := make(map[string]float64)
	for client, durations := range m.llmDurations {
		if len(durations) > 0 {
			var total time.Duration
			for _, d := range durations {
				total += d
			}
			avgDurations[client] = float64(total) / float64(len(durations)) / float64(time.Millisecond)
		}
	}
	metrics["avg_duration_ms"] = avgDurations

	return metrics
}

// GetTaskMetrics 获取任务指标
func (m *metricsCollector) GetTaskMetrics() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics := make(map[string]interface{})
	metrics["processed"] = m.taskProcessed

	// 计算平均处理时长
	if len(m.taskDurations) > 0 {
		var total time.Duration
		for _, d := range m.taskDurations {
			total += d
		}
		metrics["avg_duration_ms"] = float64(total) / float64(len(m.taskDurations)) / float64(time.Millisecond)
	}

	return metrics
}

// GetQualityMetrics 获取质量指标
func (m *metricsCollector) GetQualityMetrics() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics := make(map[string]interface{})

	// 计算平均质量评分
	if len(m.qualityScores) > 0 {
		var total float64
		for _, score := range m.qualityScores {
			total += score
		}
		metrics["avg_score"] = total / float64(len(m.qualityScores))
		metrics["count"] = len(m.qualityScores)
	}

	return metrics
}

// GetStrategyMetrics 获取策略指标
func (m *metricsCollector) GetStrategyMetrics() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics := make(map[string]interface{})
	metrics["usage"] = m.strategyUsage

	return metrics
}

// GetQueueMetrics 获取队列指标
func (m *metricsCollector) GetQueueMetrics() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics := make(map[string]interface{})
	metrics["length"] = m.queueLength

	return metrics
}

// GetAPIMetrics 获取 API 指标
func (m *metricsCollector) GetAPIMetrics() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics := make(map[string]interface{})
	metrics["requests"] = m.apiRequests
	metrics["status_codes"] = m.apiStatusCodes

	// 计算平均延迟
	avgDurations := make(map[string]float64)
	for endpoint, durations := range m.apiDurations {
		if len(durations) > 0 {
			var total time.Duration
			for _, d := range durations {
				total += d
			}
			avgDurations[endpoint] = float64(total) / float64(len(durations)) / float64(time.Millisecond)
		}
	}
	metrics["avg_duration_ms"] = avgDurations

	return metrics
}

// 全局指标收集器实例
var globalMetrics MetricsCollector
var metricsOnce sync.Once

// RecordCacheHit 记录缓存命中
func (m *metricsCollector) RecordCacheHit(cacheType string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cacheHits[cacheType]++
}

// RecordCacheMiss 记录缓存未命中
func (m *metricsCollector) RecordCacheMiss(cacheType string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cacheMisses[cacheType]++
}

// RecordCacheOperation 记录缓存操作
func (m *metricsCollector) RecordCacheOperation(operation, cacheType string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cacheOperations[operation] == nil {
		m.cacheOperations[operation] = make(map[string]uint64)
	}
	m.cacheOperations[operation][cacheType]++
}

// UpdateCacheSize 更新缓存大小
func (m *metricsCollector) UpdateCacheSize(size int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cacheSize = size
}

// GetCacheMetrics 获取缓存指标
func (m *metricsCollector) GetCacheMetrics() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics := make(map[string]interface{})
	metrics["hits"] = m.cacheHits
	metrics["misses"] = m.cacheMisses
	metrics["operations"] = m.cacheOperations
	metrics["size"] = m.cacheSize

	// 计算缓存命中率
	hitRates := make(map[string]float64)
	for cacheType, hits := range m.cacheHits {
		misses := m.cacheMisses[cacheType]
		total := hits + misses
		if total > 0 {
			hitRates[cacheType] = float64(hits) / float64(total) * 100
		}
	}
	metrics["hit_rates"] = hitRates

	return metrics
}

// GetGlobalMetrics 获取全局指标收集器
func GetGlobalMetrics() MetricsCollector {
	metricsOnce.Do(func() {
		globalMetrics = NewMetricsCollector()
	})
	return globalMetrics
}

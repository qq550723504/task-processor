// Package rabbitmq 提供负载监控服务
package rabbitmq

import (
	"context"
	"fmt"
	"sync"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/monitoring"
	"task-processor/internal/pkg/recovery"
	"time"

	"github.com/sirupsen/logrus"
)

// LoadMonitor 负载监控器
// 基于通用监控基础设施,专注于RabbitMQ任务处理的监控
type LoadMonitor struct {
	logger *logrus.Logger

	// 使用通用指标收集器
	metricsCollector *monitoring.MetricsCollector

	// 任务统计数据
	stats      LoadStats
	statsMutex sync.RWMutex

	// 滑动窗口统计
	processingTimeWindow *SlidingWindowStats
	queueWindows         map[string]*SlidingWindowStats
	queueWindowsMutex    sync.RWMutex

	// 生命周期管理
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// 配置
	config config.LoadMonitorConfig
}

// LoadStats 负载统计信息
type LoadStats struct {
	// 任务统计
	TasksProcessed int64 `json:"tasks_processed"`
	TasksSucceeded int64 `json:"tasks_succeeded"`
	TasksFailed    int64 `json:"tasks_failed"`
	TasksRetried   int64 `json:"tasks_retried"`

	// 性能指标（使用滑动窗口统计）
	ProcessingTimeStats WindowStats `json:"processing_time_stats"`

	// 队列统计
	QueueStats map[string]QueueLoadStats `json:"queue_stats"`

	// 时间戳
	LastUpdated time.Time `json:"last_updated"`
}

// QueueLoadStats 队列负载统计
type QueueLoadStats struct {
	MessagesProcessed int64       `json:"messages_processed"`
	MessagesSucceeded int64       `json:"messages_succeeded"`
	MessagesFailed    int64       `json:"messages_failed"`
	ProcessingStats   WindowStats `json:"processing_stats"`
	LastProcessed     time.Time   `json:"last_processed"`
}

// MonitorConfig 监控配置（内部使用，用于兼容）
type MonitorConfig = config.LoadMonitorConfig

// NewLoadMonitor 创建负载监控器
func NewLoadMonitor(cfg config.LoadMonitorConfig, logger *logrus.Logger) *LoadMonitor {
	// 设置默认值
	if cfg.UpdateInterval == 0 {
		cfg.UpdateInterval = 30 * time.Second
	}

	// 创建通用指标收集器
	metricsCollector := monitoring.NewMetricsCollector(logger, cfg.UpdateInterval)

	return &LoadMonitor{
		logger:               logger,
		metricsCollector:     metricsCollector,
		config:               cfg,
		processingTimeWindow: NewSlidingWindowStats(1000), // 记录最近1000个处理时间
		queueWindows:         make(map[string]*SlidingWindowStats),
		stats: LoadStats{
			QueueStats: make(map[string]QueueLoadStats),
		},
	}
}

// Start 启动负载监控
func (lm *LoadMonitor) Start(ctx context.Context) error {
	lm.ctx, lm.cancel = context.WithCancel(ctx)

	// 启动通用指标收集器
	if err := lm.metricsCollector.Start(ctx); err != nil {
		return fmt.Errorf("启动指标收集器失败: %w", err)
	}

	// 启动监控goroutine
	lm.wg.Add(1)
	go lm.monitorLoop()

	lm.logger.Info("负载监控器启动完成")
	return nil
}

// Stop 停止负载监控
func (lm *LoadMonitor) Stop(ctx context.Context) error {
	lm.logger.Info("停止负载监控器...")

	// 停止指标收集器
	if err := lm.metricsCollector.Stop(ctx); err != nil {
		lm.logger.Warnf("停止指标收集器失败: %v", err)
	}

	// 取消上下文
	if lm.cancel != nil {
		lm.cancel()
	}

	// 等待goroutine完成
	done := make(chan struct{})
	go func() {
		defer close(done)
		lm.wg.Wait()
	}()

	select {
	case <-done:
		lm.logger.Info("负载监控器停止完成")
	case <-ctx.Done():
		lm.logger.Warn("等待负载监控器停止超时")
		return fmt.Errorf("停止负载监控器超时")
	}

	return nil
}

// monitorLoop 监控循环
func (lm *LoadMonitor) monitorLoop() {
	defer lm.wg.Done()
	defer recovery.RecoverWithStack("负载监控器", lm.logger.WithField("component", "LoadMonitor"))

	ticker := time.NewTicker(lm.config.UpdateInterval)
	defer ticker.Stop()

	// 立即执行一次
	lm.updateStats()

	for {
		select {
		case <-lm.ctx.Done():
			lm.logger.Info("负载监控循环停止")
			return
		case <-ticker.C:
			lm.updateStats()
		}
	}
}

// updateStats 更新统计信息
func (lm *LoadMonitor) updateStats() {
	lm.statsMutex.Lock()
	defer lm.statsMutex.Unlock()

	// 获取处理时间统计
	lm.stats.ProcessingTimeStats = lm.processingTimeWindow.GetStats()

	// 更新RabbitMQ特定的指标到通用指标收集器
	lm.metricsCollector.SetCounter("rabbitmq_tasks_processed_total", float64(lm.stats.TasksProcessed), nil, "处理的任务总数")
	lm.metricsCollector.SetCounter("rabbitmq_tasks_succeeded_total", float64(lm.stats.TasksSucceeded), nil, "成功的任务总数")
	lm.metricsCollector.SetCounter("rabbitmq_tasks_failed_total", float64(lm.stats.TasksFailed), nil, "失败的任务总数")
	lm.metricsCollector.SetCounter("rabbitmq_tasks_retried_total", float64(lm.stats.TasksRetried), nil, "重试的任务总数")

	lm.metricsCollector.SetGauge("rabbitmq_avg_processing_time_seconds", lm.stats.ProcessingTimeStats.Average.Seconds(), nil, "平均处理时间")
	lm.metricsCollector.SetGauge("rabbitmq_max_processing_time_seconds", lm.stats.ProcessingTimeStats.Max.Seconds(), nil, "最大处理时间")
	lm.metricsCollector.SetGauge("rabbitmq_min_processing_time_seconds", lm.stats.ProcessingTimeStats.Min.Seconds(), nil, "最小处理时间")
	lm.metricsCollector.SetGauge("rabbitmq_p95_processing_time_seconds", lm.stats.ProcessingTimeStats.P95.Seconds(), nil, "P95处理时间")
	lm.metricsCollector.SetGauge("rabbitmq_p99_processing_time_seconds", lm.stats.ProcessingTimeStats.P99.Seconds(), nil, "P99处理时间")

	// 更新队列统计
	for queueName, queueStats := range lm.stats.QueueStats {
		labels := map[string]string{"queue": queueName}
		lm.metricsCollector.SetCounter("rabbitmq_queue_messages_processed_total", float64(queueStats.MessagesProcessed), labels, "队列处理消息总数")
		lm.metricsCollector.SetCounter("rabbitmq_queue_messages_succeeded_total", float64(queueStats.MessagesSucceeded), labels, "队列成功消息总数")
		lm.metricsCollector.SetCounter("rabbitmq_queue_messages_failed_total", float64(queueStats.MessagesFailed), labels, "队列失败消息总数")
		lm.metricsCollector.SetGauge("rabbitmq_queue_avg_processing_time_seconds", queueStats.ProcessingStats.Average.Seconds(), labels, "队列平均处理时间")
		lm.metricsCollector.SetGauge("rabbitmq_queue_p95_processing_time_seconds", queueStats.ProcessingStats.P95.Seconds(), labels, "队列P95处理时间")
	}

	// 更新时间戳
	lm.stats.LastUpdated = time.Now()

	lm.logger.Debugf("负载统计更新: 已处理=%d, 成功=%d, 失败=%d, 平均耗时=%v",
		lm.stats.TasksProcessed, lm.stats.TasksSucceeded, lm.stats.TasksFailed, lm.stats.ProcessingTimeStats.Average)
}

// RecordTaskProcessed 记录任务处理
func (lm *LoadMonitor) RecordTaskProcessed(queueName string, success bool, processingTime time.Duration) {
	lm.statsMutex.Lock()
	defer lm.statsMutex.Unlock()

	// 更新总体统计
	lm.stats.TasksProcessed++
	if success {
		lm.stats.TasksSucceeded++
	} else {
		lm.stats.TasksFailed++
	}

	// 添加到滑动窗口
	lm.processingTimeWindow.Add(processingTime)

	// 更新队列统计
	queueStats, exists := lm.stats.QueueStats[queueName]
	if !exists {
		queueStats = QueueLoadStats{}
	}

	queueStats.MessagesProcessed++
	if success {
		queueStats.MessagesSucceeded++
	} else {
		queueStats.MessagesFailed++
	}
	queueStats.LastProcessed = time.Now()

	// 获取或创建队列的滑动窗口
	lm.queueWindowsMutex.Lock()
	queueWindow, exists := lm.queueWindows[queueName]
	if !exists {
		queueWindow = NewSlidingWindowStats(1000)
		lm.queueWindows[queueName] = queueWindow
	}
	lm.queueWindowsMutex.Unlock()

	// 添加到队列窗口
	queueWindow.Add(processingTime)

	// 获取队列统计
	queueStats.ProcessingStats = queueWindow.GetStats()

	lm.stats.QueueStats[queueName] = queueStats
}

// RecordTaskRetried 记录任务重试
func (lm *LoadMonitor) RecordTaskRetried(queueName string) {
	lm.statsMutex.Lock()
	defer lm.statsMutex.Unlock()

	lm.stats.TasksRetried++
}

// GetStats 获取负载统计信息
func (lm *LoadMonitor) GetStats() LoadStats {
	lm.statsMutex.RLock()
	defer lm.statsMutex.RUnlock()

	// 深拷贝统计信息
	stats := lm.stats
	stats.QueueStats = make(map[string]QueueLoadStats)
	for k, v := range lm.stats.QueueStats {
		stats.QueueStats[k] = v
	}

	return stats
}

// GetHealthStatus 获取健康状态
func (lm *LoadMonitor) GetHealthStatus() map[string]any {
	stats := lm.GetStats()

	// 从通用指标收集器获取系统指标
	metrics := lm.metricsCollector.GetMetrics()

	health := make(map[string]any)
	health["status"] = "healthy"
	health["tasks_processed"] = stats.TasksProcessed
	health["success_rate"] = lm.calculateSuccessRate(stats)
	health["last_updated"] = stats.LastUpdated

	// 添加系统指标
	if cpuMetric, ok := metrics["system_cpu_cores"]; ok {
		health["cpu_cores"] = cpuMetric.Value
	}
	if goroutineMetric, ok := metrics["system_goroutines_count"]; ok {
		health["goroutine_count"] = goroutineMetric.Value

		// 判断健康状态
		if goroutineMetric.Value > 1000 {
			health["status"] = "warning"
			health["warning"] = "Goroutine数量过多"
		}
	}

	return health
}

// calculateSuccessRate 计算成功率
func (lm *LoadMonitor) calculateSuccessRate(stats LoadStats) float64 {
	if stats.TasksProcessed == 0 {
		return 0.0
	}

	return (float64(stats.TasksSucceeded) / float64(stats.TasksProcessed)) * 100.0
}

// ResetStats 重置统计信息
func (lm *LoadMonitor) ResetStats() {
	lm.statsMutex.Lock()
	defer lm.statsMutex.Unlock()

	lm.stats = LoadStats{
		QueueStats:  make(map[string]QueueLoadStats),
		LastUpdated: time.Now(),
	}

	// 重置滑动窗口
	lm.processingTimeWindow.Reset()

	lm.queueWindowsMutex.Lock()
	for _, window := range lm.queueWindows {
		window.Reset()
	}
	lm.queueWindowsMutex.Unlock()

	lm.logger.Info("负载统计信息已重置")
}

// GetMetricsCollector 获取指标收集器
// 允许外部访问底层的指标收集器
func (lm *LoadMonitor) GetMetricsCollector() *monitoring.MetricsCollector {
	return lm.metricsCollector
}

// Package rabbitmq 提供负载监控服务
package rabbitmq

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// LoadMonitor 负载监控器
type LoadMonitor struct {
	logger *logrus.Logger

	// 监控数据
	stats      LoadStats
	statsMutex sync.RWMutex

	// 生命周期管理
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// 配置
	config MonitorConfig
}

// LoadStats 负载统计信息
type LoadStats struct {
	// 系统资源
	CPUUsage       float64 `json:"cpu_usage"`
	MemoryUsage    float64 `json:"memory_usage"`
	GoroutineCount int     `json:"goroutine_count"`

	// 任务统计
	TasksProcessed int64 `json:"tasks_processed"`
	TasksSucceeded int64 `json:"tasks_succeeded"`
	TasksFailed    int64 `json:"tasks_failed"`
	TasksRetried   int64 `json:"tasks_retried"`

	// 性能指标
	AvgProcessingTime time.Duration `json:"avg_processing_time"`
	MaxProcessingTime time.Duration `json:"max_processing_time"`
	MinProcessingTime time.Duration `json:"min_processing_time"`

	// 队列统计
	QueueStats map[string]QueueLoadStats `json:"queue_stats"`

	// 时间戳
	LastUpdated time.Time `json:"last_updated"`
}

// QueueLoadStats 队列负载统计
type QueueLoadStats struct {
	MessagesProcessed int64         `json:"messages_processed"`
	MessagesSucceeded int64         `json:"messages_succeeded"`
	MessagesFailed    int64         `json:"messages_failed"`
	AvgProcessingTime time.Duration `json:"avg_processing_time"`
	LastProcessed     time.Time     `json:"last_processed"`
}

// MonitorConfig 监控配置
type MonitorConfig struct {
	UpdateInterval time.Duration `yaml:"update_interval"`
	EnableCPU      bool          `yaml:"enable_cpu"`
	EnableMemory   bool          `yaml:"enable_memory"`
	EnableTasks    bool          `yaml:"enable_tasks"`
}

// NewLoadMonitor 创建负载监控器
func NewLoadMonitor(config MonitorConfig, logger *logrus.Logger) *LoadMonitor {
	// 设置默认值
	if config.UpdateInterval == 0 {
		config.UpdateInterval = 30 * time.Second
	}

	return &LoadMonitor{
		logger: logger,
		config: config,
		stats: LoadStats{
			QueueStats:        make(map[string]QueueLoadStats),
			MinProcessingTime: time.Hour, // 初始化为一个大值
		},
	}
}

// Start 启动负载监控
func (lm *LoadMonitor) Start(ctx context.Context) error {
	lm.ctx, lm.cancel = context.WithCancel(ctx)

	// 启动监控goroutine
	lm.wg.Add(1)
	go lm.monitorLoop()

	lm.logger.Info("负载监控器启动完成")
	return nil
}

// Stop 停止负载监控
func (lm *LoadMonitor) Stop(ctx context.Context) error {
	lm.logger.Info("停止负载监控器...")

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
	defer func() {
		if r := recover(); r != nil {
			lm.logger.Errorf("负载监控器发生panic: %v", r)
		}
	}()

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

	// 更新系统资源统计
	if lm.config.EnableCPU {
		lm.updateCPUStats()
	}

	if lm.config.EnableMemory {
		lm.updateMemoryStats()
	}

	// 更新Goroutine数量
	lm.stats.GoroutineCount = runtime.NumGoroutine()

	// 更新时间戳
	lm.stats.LastUpdated = time.Now()

	lm.logger.Debugf("负载统计更新: CPU=%.2f%%, Memory=%.2f%%, Goroutines=%d",
		lm.stats.CPUUsage, lm.stats.MemoryUsage, lm.stats.GoroutineCount)
}

// updateCPUStats 更新CPU统计
func (lm *LoadMonitor) updateCPUStats() {
	// 简单的CPU使用率估算（基于Goroutine数量）
	// 在实际生产环境中，可以使用更精确的CPU监控库
	goroutineCount := runtime.NumGoroutine()

	// 基于Goroutine数量估算CPU使用率
	// 这是一个简化的实现，实际应该使用系统调用获取真实CPU使用率
	if goroutineCount < 10 {
		lm.stats.CPUUsage = float64(goroutineCount) * 2.0
	} else if goroutineCount < 50 {
		lm.stats.CPUUsage = 20.0 + float64(goroutineCount-10)*1.5
	} else {
		lm.stats.CPUUsage = 80.0 + float64(goroutineCount-50)*0.2
		if lm.stats.CPUUsage > 100.0 {
			lm.stats.CPUUsage = 100.0
		}
	}
}

// updateMemoryStats 更新内存统计
func (lm *LoadMonitor) updateMemoryStats() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// 计算内存使用率（简化版本）
	// 实际应该获取系统总内存来计算准确的使用率
	allocMB := float64(m.Alloc) / 1024 / 1024
	sysMB := float64(m.Sys) / 1024 / 1024

	// 假设系统有4GB内存（实际应该动态获取）
	totalMemoryMB := 4096.0
	lm.stats.MemoryUsage = (sysMB / totalMemoryMB) * 100.0

	if lm.stats.MemoryUsage > 100.0 {
		lm.stats.MemoryUsage = 100.0
	}

	lm.logger.Debugf("内存统计: Alloc=%.2fMB, Sys=%.2fMB, Usage=%.2f%%",
		allocMB, sysMB, lm.stats.MemoryUsage)
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

	// 更新处理时间统计
	lm.updateProcessingTimeStats(processingTime)

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

	// 更新队列平均处理时间
	if queueStats.MessagesProcessed == 1 {
		queueStats.AvgProcessingTime = processingTime
	} else {
		// 简单的移动平均
		queueStats.AvgProcessingTime = (queueStats.AvgProcessingTime + processingTime) / 2
	}

	queueStats.LastProcessed = time.Now()
	lm.stats.QueueStats[queueName] = queueStats
}

// RecordTaskRetried 记录任务重试
func (lm *LoadMonitor) RecordTaskRetried(queueName string) {
	lm.statsMutex.Lock()
	defer lm.statsMutex.Unlock()

	lm.stats.TasksRetried++
}

// updateProcessingTimeStats 更新处理时间统计
func (lm *LoadMonitor) updateProcessingTimeStats(processingTime time.Duration) {
	// 更新最大处理时间
	if processingTime > lm.stats.MaxProcessingTime {
		lm.stats.MaxProcessingTime = processingTime
	}

	// 更新最小处理时间
	if processingTime < lm.stats.MinProcessingTime {
		lm.stats.MinProcessingTime = processingTime
	}

	// 更新平均处理时间（简单移动平均）
	if lm.stats.TasksProcessed == 1 {
		lm.stats.AvgProcessingTime = processingTime
	} else {
		lm.stats.AvgProcessingTime = (lm.stats.AvgProcessingTime + processingTime) / 2
	}
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
func (lm *LoadMonitor) GetHealthStatus() map[string]interface{} {
	stats := lm.GetStats()

	health := make(map[string]interface{})
	health["status"] = "healthy"
	health["cpu_usage"] = stats.CPUUsage
	health["memory_usage"] = stats.MemoryUsage
	health["goroutine_count"] = stats.GoroutineCount
	health["tasks_processed"] = stats.TasksProcessed
	health["success_rate"] = lm.calculateSuccessRate(stats)
	health["last_updated"] = stats.LastUpdated

	// 判断健康状态
	if stats.CPUUsage > 90.0 {
		health["status"] = "warning"
		health["warning"] = "CPU使用率过高"
	}

	if stats.MemoryUsage > 90.0 {
		health["status"] = "warning"
		health["warning"] = "内存使用率过高"
	}

	if stats.GoroutineCount > 1000 {
		health["status"] = "warning"
		health["warning"] = "Goroutine数量过多"
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
		QueueStats:        make(map[string]QueueLoadStats),
		MinProcessingTime: time.Hour,
		LastUpdated:       time.Now(),
	}

	lm.logger.Info("负载统计信息已重置")
}

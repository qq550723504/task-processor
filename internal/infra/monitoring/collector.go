// Package monitoring 提供指标收集功能
package monitoring

import (
	"context"
	"os"
	"runtime"
	"sync"
	"task-processor/internal/core/errors"
	"task-processor/internal/core/lifecycle"
	"time"

	"github.com/sirupsen/logrus"
)

// MetricsCollector 指标收集器
type MetricsCollector struct {
	*lifecycle.BaseComponent
	logger   *logrus.Logger
	metrics  map[string]*Metric
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	interval time.Duration
}

// NewMetricsCollector 创建指标收集器
func NewMetricsCollector(logger *logrus.Logger, interval time.Duration) *MetricsCollector {
	if interval <= 0 {
		interval = 30 * time.Second
	}

	return &MetricsCollector{
		BaseComponent: lifecycle.NewBaseComponent("MetricsCollector"),
		logger:        logger,
		metrics:       make(map[string]*Metric),
		interval:      interval,
	}
}

// Start 启动指标收集器
func (m *MetricsCollector) Start(ctx context.Context) error {
	if m.IsRunning() {
		return errors.New(errors.ErrCodeSystem, "MetricsCollector已在运行")
	}

	m.logger.Info("启动指标收集器...")

	m.ctx, m.cancel = context.WithCancel(ctx)

	// 启动指标收集循环
	go m.collectLoop()

	m.SetRunning(true)
	m.logger.Info("指标收集器启动完成")
	return nil
}

// Stop 停止指标收集器
func (m *MetricsCollector) Stop(ctx context.Context) error {
	if !m.IsRunning() {
		return nil
	}

	m.logger.Info("停止指标收集器...")

	if m.cancel != nil {
		m.cancel()
	}

	m.SetRunning(false)
	m.logger.Info("指标收集器停止完成")
	return nil
}

// collectLoop 指标收集循环
func (m *MetricsCollector) collectLoop() {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			m.logger.Info("指标收集循环停止")
			return
		case <-ticker.C:
			m.collectMetrics()
		}
	}
}

// collectMetrics 收集指标
func (m *MetricsCollector) collectMetrics() {
	m.logger.Debug("开始收集指标...")

	// 收集系统指标
	m.collectSystemMetrics()

	// 收集应用指标
	m.collectApplicationMetrics()

	// 输出指标
	m.logMetrics()
}

// collectSystemMetrics 收集系统指标
func (m *MetricsCollector) collectSystemMetrics() {
	// 收集内存使用情况
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// 堆内存使用量 (字节)
	m.SetGauge("system_memory_heap_bytes", float64(memStats.HeapInuse),
		map[string]string{"type": "inuse"}, "堆内存使用量")

	// 堆内存分配量 (字节)
	m.SetGauge("system_memory_heap_bytes", float64(memStats.HeapAlloc),
		map[string]string{"type": "alloc"}, "堆内存分配量")

	// 系统内存使用量 (字节)
	m.SetGauge("system_memory_sys_bytes", float64(memStats.Sys),
		nil, "系统内存使用量")

	// GC次数
	m.SetGauge("system_gc_runs_total", float64(memStats.NumGC),
		nil, "GC运行次数")

	// 上次GC暂停时间 (纳秒)
	if memStats.NumGC > 0 {
		lastPause := memStats.PauseNs[(memStats.NumGC+255)%256]
		m.SetGauge("system_gc_pause_ns", float64(lastPause),
			map[string]string{"type": "last"}, "上次GC暂停时间")
	}

	// Goroutine数量
	m.SetGauge("system_goroutines_count", float64(runtime.NumGoroutine()),
		nil, "Goroutine数量")

	// CPU核心数
	m.SetGauge("system_cpu_cores", float64(runtime.NumCPU()),
		nil, "CPU核心数")

	// 收集进程信息
	m.collectProcessMetrics()
}

// collectApplicationMetrics 收集应用指标
func (m *MetricsCollector) collectApplicationMetrics() {
	// 这里可以收集应用特定的指标
	// 比如任务处理数量、错误率等
}

// GetMetrics 获取所有指标
func (m *MetricsCollector) GetMetrics() map[string]*Metric {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*Metric)
	for k, v := range m.metrics {
		result[k] = v
	}
	return result
}

// logMetrics 输出指标到日志
func (m *MetricsCollector) logMetrics() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.metrics) == 0 {
		return
	}

	fields := make(logrus.Fields)
	for key, metric := range m.metrics {
		fields[key] = metric.Value
	}

	m.logger.WithFields(fields).Debug("当前指标状态")
}

// collectProcessMetrics 收集进程相关指标
func (m *MetricsCollector) collectProcessMetrics() {
	// 进程ID
	m.SetGauge("system_process_id", float64(os.Getpid()),
		nil, "进程ID")

	// 进程启动时间 (Unix时间戳)
	startTime := GetProcessStartTime()
	m.SetGauge("system_process_start_time", float64(startTime),
		nil, "进程启动时间")

	// 进程运行时间 (秒)
	uptime := GetProcessUptime()
	m.SetGauge("system_process_uptime_seconds", float64(uptime),
		nil, "进程运行时间")
}

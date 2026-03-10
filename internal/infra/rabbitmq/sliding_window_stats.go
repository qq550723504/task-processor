// Package rabbitmq 提供滑动窗口统计功能
package rabbitmq

import (
	"sync"
	"time"
)

// SlidingWindowStats 滑动窗口统计
// 使用固定大小的环形缓冲区记录最近N个数据点
type SlidingWindowStats struct {
	window     []time.Duration
	windowSize int
	index      int
	full       bool
	mutex      sync.RWMutex
}

// NewSlidingWindowStats 创建滑动窗口统计
func NewSlidingWindowStats(size int) *SlidingWindowStats {
	if size <= 0 {
		size = 100 // 默认窗口大小
	}

	return &SlidingWindowStats{
		window:     make([]time.Duration, size),
		windowSize: size,
	}
}

// Add 添加一个数据点
func (sws *SlidingWindowStats) Add(duration time.Duration) {
	sws.mutex.Lock()
	defer sws.mutex.Unlock()

	sws.window[sws.index] = duration
	sws.index = (sws.index + 1) % sws.windowSize

	if !sws.full && sws.index == 0 {
		sws.full = true
	}
}

// Average 计算平均值
func (sws *SlidingWindowStats) Average() time.Duration {
	sws.mutex.RLock()
	defer sws.mutex.RUnlock()

	count := sws.getCount()
	if count == 0 {
		return 0
	}

	var sum time.Duration
	for i := 0; i < count; i++ {
		sum += sws.window[i]
	}

	return sum / time.Duration(count)
}

// Max 获取最大值
func (sws *SlidingWindowStats) Max() time.Duration {
	sws.mutex.RLock()
	defer sws.mutex.RUnlock()

	count := sws.getCount()
	if count == 0 {
		return 0
	}

	max := sws.window[0]
	for i := 1; i < count; i++ {
		if sws.window[i] > max {
			max = sws.window[i]
		}
	}

	return max
}

// Min 获取最小值
func (sws *SlidingWindowStats) Min() time.Duration {
	sws.mutex.RLock()
	defer sws.mutex.RUnlock()

	count := sws.getCount()
	if count == 0 {
		return 0
	}

	min := sws.window[0]
	for i := 1; i < count; i++ {
		if sws.window[i] < min && sws.window[i] > 0 {
			min = sws.window[i]
		}
	}

	return min
}

// Percentile 计算百分位数
// p 应该在 0-100 之间，例如 p=95 表示 P95
func (sws *SlidingWindowStats) Percentile(p float64) time.Duration {
	sws.mutex.RLock()
	defer sws.mutex.RUnlock()

	count := sws.getCount()
	if count == 0 {
		return 0
	}

	// 复制数据并排序
	data := make([]time.Duration, count)
	copy(data, sws.window[:count])

	// 简单的冒泡排序（对于小数据集足够）
	for i := 0; i < count-1; i++ {
		for j := 0; j < count-i-1; j++ {
			if data[j] > data[j+1] {
				data[j], data[j+1] = data[j+1], data[j]
			}
		}
	}

	// 计算百分位数索引
	index := int(float64(count-1) * p / 100.0)
	if index < 0 {
		index = 0
	}
	if index >= count {
		index = count - 1
	}

	return data[index]
}

// Count 获取当前数据点数量
func (sws *SlidingWindowStats) Count() int {
	sws.mutex.RLock()
	defer sws.mutex.RUnlock()

	return sws.getCount()
}

// Reset 重置统计
func (sws *SlidingWindowStats) Reset() {
	sws.mutex.Lock()
	defer sws.mutex.Unlock()

	sws.window = make([]time.Duration, sws.windowSize)
	sws.index = 0
	sws.full = false
}

// getCount 获取当前数据点数量（内部方法，不加锁）
func (sws *SlidingWindowStats) getCount() int {
	if sws.full {
		return sws.windowSize
	}
	return sws.index
}

// GetStats 获取所有统计信息
func (sws *SlidingWindowStats) GetStats() WindowStats {
	sws.mutex.RLock()
	defer sws.mutex.RUnlock()

	count := sws.getCount()
	if count == 0 {
		return WindowStats{}
	}

	// 计算所有统计值
	var sum time.Duration
	max := sws.window[0]
	min := sws.window[0]

	for i := 0; i < count; i++ {
		duration := sws.window[i]
		sum += duration

		if duration > max {
			max = duration
		}
		if duration < min && duration > 0 {
			min = duration
		}
	}

	avg := sum / time.Duration(count)

	// 计算百分位数
	data := make([]time.Duration, count)
	copy(data, sws.window[:count])

	// 排序
	for i := 0; i < count-1; i++ {
		for j := 0; j < count-i-1; j++ {
			if data[j] > data[j+1] {
				data[j], data[j+1] = data[j+1], data[j]
			}
		}
	}

	p50 := data[int(float64(count-1)*0.50)]
	p95 := data[int(float64(count-1)*0.95)]
	p99 := data[int(float64(count-1)*0.99)]

	return WindowStats{
		Count:   count,
		Average: avg,
		Max:     max,
		Min:     min,
		P50:     p50,
		P95:     p95,
		P99:     p99,
	}
}

// WindowStats 窗口统计信息
type WindowStats struct {
	Count   int           `json:"count"`
	Average time.Duration `json:"average"`
	Max     time.Duration `json:"max"`
	Min     time.Duration `json:"min"`
	P50     time.Duration `json:"p50"` // 中位数
	P95     time.Duration `json:"p95"`
	P99     time.Duration `json:"p99"`
}

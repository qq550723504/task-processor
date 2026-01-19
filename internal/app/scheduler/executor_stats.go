// Package scheduler 提供任务执行器统计功能
package scheduler

import (
	"sync/atomic"
	"time"
)

// ExecutorStats 任务执行器统计信息
type ExecutorStats struct {
	totalExecutions   int64     // 总执行次数
	successExecutions int64     // 成功执行次数
	failedExecutions  int64     // 失败执行次数
	skipExecutions    int64     // 跳过执行次数
	lastExecutionTime time.Time // 最后执行时间
	lastSuccessTime   time.Time // 最后成功时间
	lastFailureTime   time.Time // 最后失败时间
	totalDuration     int64     // 总执行时长(纳秒)
	maxDuration       int64     // 最大执行时长(纳秒)
	minDuration       int64     // 最小执行时长(纳秒)
}

// NewExecutorStats 创建新的执行器统计
func NewExecutorStats() *ExecutorStats {
	return &ExecutorStats{
		minDuration: int64(^uint64(0) >> 1), // 设置为最大值
	}
}

// RecordExecution 记录执行统计
func (s *ExecutorStats) RecordExecution(duration time.Duration, success bool) {
	atomic.AddInt64(&s.totalExecutions, 1)

	durationNanos := duration.Nanoseconds()
	atomic.AddInt64(&s.totalDuration, durationNanos)

	// 更新最大执行时长
	for {
		current := atomic.LoadInt64(&s.maxDuration)
		if durationNanos <= current {
			break
		}
		if atomic.CompareAndSwapInt64(&s.maxDuration, current, durationNanos) {
			break
		}
	}

	// 更新最小执行时长
	for {
		current := atomic.LoadInt64(&s.minDuration)
		if durationNanos >= current {
			break
		}
		if atomic.CompareAndSwapInt64(&s.minDuration, current, durationNanos) {
			break
		}
	}

	s.lastExecutionTime = time.Now()

	if success {
		atomic.AddInt64(&s.successExecutions, 1)
		s.lastSuccessTime = time.Now()
	} else {
		atomic.AddInt64(&s.failedExecutions, 1)
		s.lastFailureTime = time.Now()
	}
}

// RecordSkip 记录跳过执行
func (s *ExecutorStats) RecordSkip() {
	atomic.AddInt64(&s.skipExecutions, 1)
}

// GetStats 获取统计信息
func (s *ExecutorStats) GetStats() ExecutorStatsSnapshot {
	totalExec := atomic.LoadInt64(&s.totalExecutions)
	totalDur := atomic.LoadInt64(&s.totalDuration)

	var avgDuration time.Duration
	if totalExec > 0 {
		avgDuration = time.Duration(totalDur / totalExec)
	}

	minDur := atomic.LoadInt64(&s.minDuration)
	if minDur == int64(^uint64(0)>>1) {
		minDur = 0 // 如果没有执行过，最小时长为0
	}

	return ExecutorStatsSnapshot{
		TotalExecutions:   totalExec,
		SuccessExecutions: atomic.LoadInt64(&s.successExecutions),
		FailedExecutions:  atomic.LoadInt64(&s.failedExecutions),
		SkipExecutions:    atomic.LoadInt64(&s.skipExecutions),
		LastExecutionTime: s.lastExecutionTime,
		LastSuccessTime:   s.lastSuccessTime,
		LastFailureTime:   s.lastFailureTime,
		AverageDuration:   avgDuration,
		MaxDuration:       time.Duration(atomic.LoadInt64(&s.maxDuration)),
		MinDuration:       time.Duration(minDur),
	}
}

// Reset 重置统计信息
func (s *ExecutorStats) Reset() {
	atomic.StoreInt64(&s.totalExecutions, 0)
	atomic.StoreInt64(&s.successExecutions, 0)
	atomic.StoreInt64(&s.failedExecutions, 0)
	atomic.StoreInt64(&s.skipExecutions, 0)
	atomic.StoreInt64(&s.totalDuration, 0)
	atomic.StoreInt64(&s.maxDuration, 0)
	atomic.StoreInt64(&s.minDuration, int64(^uint64(0)>>1))
	s.lastExecutionTime = time.Time{}
	s.lastSuccessTime = time.Time{}
	s.lastFailureTime = time.Time{}
}

// ExecutorStatsSnapshot 执行器统计快照
type ExecutorStatsSnapshot struct {
	TotalExecutions   int64         `json:"total_executions"`
	SuccessExecutions int64         `json:"success_executions"`
	FailedExecutions  int64         `json:"failed_executions"`
	SkipExecutions    int64         `json:"skip_executions"`
	LastExecutionTime time.Time     `json:"last_execution_time"`
	LastSuccessTime   time.Time     `json:"last_success_time"`
	LastFailureTime   time.Time     `json:"last_failure_time"`
	AverageDuration   time.Duration `json:"average_duration"`
	MaxDuration       time.Duration `json:"max_duration"`
	MinDuration       time.Duration `json:"min_duration"`
}

// GetSuccessRate 获取成功率
func (s *ExecutorStatsSnapshot) GetSuccessRate() float64 {
	if s.TotalExecutions == 0 {
		return 0
	}
	return float64(s.SuccessExecutions) / float64(s.TotalExecutions) * 100
}

// GetFailureRate 获取失败率
func (s *ExecutorStatsSnapshot) GetFailureRate() float64 {
	if s.TotalExecutions == 0 {
		return 0
	}
	return float64(s.FailedExecutions) / float64(s.TotalExecutions) * 100
}

// GetSkipRate 获取跳过率
func (s *ExecutorStatsSnapshot) GetSkipRate() float64 {
	totalAttempts := s.TotalExecutions + s.SkipExecutions
	if totalAttempts == 0 {
		return 0
	}
	return float64(s.SkipExecutions) / float64(totalAttempts) * 100
}

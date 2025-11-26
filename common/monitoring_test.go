package common

import (
	"testing"
	"time"
)

func TestTaskMetrics(t *testing.T) {
	// 创建新的指标实例用于测试
	metrics := &TaskMetrics{
		LastUpdateTime: time.Now(),
	}

	// 测试任务流转统计
	metrics.IncrementPending()
	metrics.IncrementProcessing()
	metrics.IncrementCompleted()
	metrics.IncrementFailed()
	metrics.IncrementRequeued()

	snapshot := metrics.GetSnapshot()
	if snapshot.PendingCount != 1 {
		t.Errorf("Expected PendingCount=1, got %d", snapshot.PendingCount)
	}
	if snapshot.ProcessingCount != 1 {
		t.Errorf("Expected ProcessingCount=1, got %d", snapshot.ProcessingCount)
	}
	if snapshot.CompletedCount != 1 {
		t.Errorf("Expected CompletedCount=1, got %d", snapshot.CompletedCount)
	}
	if snapshot.FailedCount != 1 {
		t.Errorf("Expected FailedCount=1, got %d", snapshot.FailedCount)
	}
	if snapshot.RequeuedCount != 1 {
		t.Errorf("Expected RequeuedCount=1, got %d", snapshot.RequeuedCount)
	}

	// 测试优先级分布统计
	metrics.RecordPriority(90) // High
	metrics.RecordPriority(60) // Medium
	metrics.RecordPriority(30) // Low
	metrics.RecordPriority(80) // High
	metrics.RecordPriority(50) // Medium

	snapshot = metrics.GetSnapshot()
	if snapshot.HighPriorityCount != 2 {
		t.Errorf("Expected HighPriorityCount=2, got %d", snapshot.HighPriorityCount)
	}
	if snapshot.MediumPriorityCount != 2 {
		t.Errorf("Expected MediumPriorityCount=2, got %d", snapshot.MediumPriorityCount)
	}
	if snapshot.LowPriorityCount != 1 {
		t.Errorf("Expected LowPriorityCount=1, got %d", snapshot.LowPriorityCount)
	}

	// 测试时间统计
	metrics.RecordWaitTime(100 * time.Millisecond)
	metrics.RecordWaitTime(200 * time.Millisecond)
	metrics.RecordProcessTime(500 * time.Millisecond)

	avgWaitTime := metrics.GetAverageWaitTime()
	if avgWaitTime != 150*time.Millisecond {
		t.Errorf("Expected average wait time=150ms, got %v", avgWaitTime)
	}

	// 测试异常统计
	metrics.IncrementHeartbeatTimeout()
	metrics.IncrementTaskLoss()
	metrics.IncrementRequeueFailure()
	metrics.IncrementMarkFailedError()

	snapshot = metrics.GetSnapshot()
	if snapshot.HeartbeatTimeoutCount != 1 {
		t.Errorf("Expected HeartbeatTimeoutCount=1, got %d", snapshot.HeartbeatTimeoutCount)
	}
	if snapshot.TaskLossCount != 1 {
		t.Errorf("Expected TaskLossCount=1, got %d", snapshot.TaskLossCount)
	}
	if snapshot.RequeueFailureCount != 1 {
		t.Errorf("Expected RequeueFailureCount=1, got %d", snapshot.RequeueFailureCount)
	}
	if snapshot.MarkFailedErrorCount != 1 {
		t.Errorf("Expected MarkFailedErrorCount=1, got %d", snapshot.MarkFailedErrorCount)
	}

	// 测试重置功能
	metrics.Reset()
	snapshot = metrics.GetSnapshot()
	if snapshot.PendingCount != 0 || snapshot.ProcessingCount != 0 {
		t.Error("Reset did not clear all metrics")
	}
}

func TestGlobalMetrics(t *testing.T) {
	// 测试全局指标实例
	metrics := GetGlobalMetrics()
	if metrics == nil {
		t.Error("Global metrics should not be nil")
	}

	// 测试日志输出（不会失败，只是确保不会panic）
	metrics.LogMetricsSummary()
}

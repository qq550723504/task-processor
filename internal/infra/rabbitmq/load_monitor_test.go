package rabbitmq

import (
	"context"
	"task-processor/internal/core/config"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestNewLoadMonitor(t *testing.T) {
	logger := logrus.New()
	cfg := config.LoadMonitorConfig{
		UpdateInterval: 10 * time.Second,
	}

	lm := NewLoadMonitor(cfg, logger)

	if lm == nil {
		t.Fatal("NewLoadMonitor 应该返回非 nil 实例")
	}

	if lm.logger != logger {
		t.Error("logger 未正确设置")
	}

	if lm.config.UpdateInterval != 10*time.Second {
		t.Errorf("UpdateInterval = %v, want %v", lm.config.UpdateInterval, 10*time.Second)
	}

	if lm.processingTimeWindow == nil {
		t.Error("processingTimeWindow 应该被初始化")
	}

	if lm.queueWindows == nil {
		t.Error("queueWindows 应该被初始化")
	}
}

func TestNewLoadMonitor_DefaultInterval(t *testing.T) {
	logger := logrus.New()
	cfg := config.LoadMonitorConfig{
		UpdateInterval: 0, // 未设置
	}

	lm := NewLoadMonitor(cfg, logger)

	// 应该使用默认值 30 秒
	if lm.config.UpdateInterval != 30*time.Second {
		t.Errorf("默认 UpdateInterval = %v, want %v", lm.config.UpdateInterval, 30*time.Second)
	}
}

func TestLoadMonitor_RecordTaskProcessed_Success(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // 减少日志输出

	cfg := config.LoadMonitorConfig{
		UpdateInterval: 1 * time.Second,
	}

	lm := NewLoadMonitor(cfg, logger)

	// 记录成功的任务
	lm.RecordTaskProcessed("test-queue", true, 100*time.Millisecond)

	stats := lm.GetStats()

	if stats.TasksProcessed != 1 {
		t.Errorf("TasksProcessed = %v, want %v", stats.TasksProcessed, 1)
	}

	if stats.TasksSucceeded != 1 {
		t.Errorf("TasksSucceeded = %v, want %v", stats.TasksSucceeded, 1)
	}

	if stats.TasksFailed != 0 {
		t.Errorf("TasksFailed = %v, want %v", stats.TasksFailed, 0)
	}

	// 验证队列统计
	queueStats, exists := stats.QueueStats["test-queue"]
	if !exists {
		t.Fatal("test-queue 统计信息不存在")
	}

	if queueStats.MessagesProcessed != 1 {
		t.Errorf("MessagesProcessed = %v, want %v", queueStats.MessagesProcessed, 1)
	}

	if queueStats.MessagesSucceeded != 1 {
		t.Errorf("MessagesSucceeded = %v, want %v", queueStats.MessagesSucceeded, 1)
	}
}

func TestLoadMonitor_RecordTaskProcessed_Failure(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.LoadMonitorConfig{
		UpdateInterval: 1 * time.Second,
	}

	lm := NewLoadMonitor(cfg, logger)

	// 记录失败的任务
	lm.RecordTaskProcessed("test-queue", false, 50*time.Millisecond)

	stats := lm.GetStats()

	if stats.TasksProcessed != 1 {
		t.Errorf("TasksProcessed = %v, want %v", stats.TasksProcessed, 1)
	}

	if stats.TasksSucceeded != 0 {
		t.Errorf("TasksSucceeded = %v, want %v", stats.TasksSucceeded, 0)
	}

	if stats.TasksFailed != 1 {
		t.Errorf("TasksFailed = %v, want %v", stats.TasksFailed, 1)
	}

	// 验证队列统计
	queueStats, exists := stats.QueueStats["test-queue"]
	if !exists {
		t.Fatal("test-queue 统计信息不存在")
	}

	if queueStats.MessagesFailed != 1 {
		t.Errorf("MessagesFailed = %v, want %v", queueStats.MessagesFailed, 1)
	}
}

func TestLoadMonitor_RecordTaskProcessed_MultipleQueues(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.LoadMonitorConfig{
		UpdateInterval: 1 * time.Second,
	}

	lm := NewLoadMonitor(cfg, logger)

	// 记录多个队列的任务
	lm.RecordTaskProcessed("queue-1", true, 100*time.Millisecond)
	lm.RecordTaskProcessed("queue-2", true, 200*time.Millisecond)
	lm.RecordTaskProcessed("queue-1", false, 150*time.Millisecond)

	stats := lm.GetStats()

	if stats.TasksProcessed != 3 {
		t.Errorf("TasksProcessed = %v, want %v", stats.TasksProcessed, 3)
	}

	if len(stats.QueueStats) != 2 {
		t.Errorf("队列数量 = %v, want %v", len(stats.QueueStats), 2)
	}

	// 验证 queue-1
	queue1Stats := stats.QueueStats["queue-1"]
	if queue1Stats.MessagesProcessed != 2 {
		t.Errorf("queue-1 MessagesProcessed = %v, want %v", queue1Stats.MessagesProcessed, 2)
	}

	// 验证 queue-2
	queue2Stats := stats.QueueStats["queue-2"]
	if queue2Stats.MessagesProcessed != 1 {
		t.Errorf("queue-2 MessagesProcessed = %v, want %v", queue2Stats.MessagesProcessed, 1)
	}
}

func TestLoadMonitor_RecordTaskRetried(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.LoadMonitorConfig{
		UpdateInterval: 1 * time.Second,
	}

	lm := NewLoadMonitor(cfg, logger)

	// 记录重试
	lm.RecordTaskRetried("test-queue")
	lm.RecordTaskRetried("test-queue")

	stats := lm.GetStats()

	if stats.TasksRetried != 2 {
		t.Errorf("TasksRetried = %v, want %v", stats.TasksRetried, 2)
	}
}

func TestLoadMonitor_GetStats_DeepCopy(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.LoadMonitorConfig{
		UpdateInterval: 1 * time.Second,
	}

	lm := NewLoadMonitor(cfg, logger)

	lm.RecordTaskProcessed("test-queue", true, 100*time.Millisecond)

	// 获取统计信息
	stats1 := lm.GetStats()
	stats2 := lm.GetStats()

	// 验证是深拷贝
	if &stats1.QueueStats == &stats2.QueueStats {
		t.Error("GetStats 应该返回深拷贝，而不是同一个引用")
	}

	// 修改 stats1 不应该影响 stats2
	stats1.QueueStats["new-queue"] = QueueLoadStats{}

	if _, exists := stats2.QueueStats["new-queue"]; exists {
		t.Error("修改 stats1 不应该影响 stats2")
	}
}

func TestLoadMonitor_ResetStats(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.LoadMonitorConfig{
		UpdateInterval: 1 * time.Second,
	}

	lm := NewLoadMonitor(cfg, logger)

	// 记录一些数据
	lm.RecordTaskProcessed("test-queue", true, 100*time.Millisecond)
	lm.RecordTaskProcessed("test-queue", false, 200*time.Millisecond)
	lm.RecordTaskRetried("test-queue")

	// 重置
	lm.ResetStats()

	stats := lm.GetStats()

	// 验证所有计数器都被重置
	if stats.TasksProcessed != 0 {
		t.Errorf("TasksProcessed = %v, want %v", stats.TasksProcessed, 0)
	}

	if stats.TasksSucceeded != 0 {
		t.Errorf("TasksSucceeded = %v, want %v", stats.TasksSucceeded, 0)
	}

	if stats.TasksFailed != 0 {
		t.Errorf("TasksFailed = %v, want %v", stats.TasksFailed, 0)
	}

	if stats.TasksRetried != 0 {
		t.Errorf("TasksRetried = %v, want %v", stats.TasksRetried, 0)
	}

	if len(stats.QueueStats) != 0 {
		t.Errorf("QueueStats 应该为空, got %v", len(stats.QueueStats))
	}
}

func TestLoadMonitor_CalculateSuccessRate(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.LoadMonitorConfig{
		UpdateInterval: 1 * time.Second,
	}

	lm := NewLoadMonitor(cfg, logger)

	tests := []struct {
		name      string
		succeeded int64
		failed    int64
		wantRate  float64
	}{
		{"全部成功", 10, 0, 100.0},
		{"全部失败", 0, 10, 0.0},
		{"一半成功", 5, 5, 50.0},
		{"80%成功", 8, 2, 80.0},
		{"无任务", 0, 0, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := LoadStats{
				TasksProcessed: tt.succeeded + tt.failed,
				TasksSucceeded: tt.succeeded,
				TasksFailed:    tt.failed,
			}

			rate := lm.calculateSuccessRate(stats)

			if rate != tt.wantRate {
				t.Errorf("成功率 = %v, want %v", rate, tt.wantRate)
			}
		})
	}
}

func TestLoadMonitor_GetHealthStatus(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.LoadMonitorConfig{
		UpdateInterval: 1 * time.Second,
	}

	lm := NewLoadMonitor(cfg, logger)

	// 记录一些成功的任务
	for i := 0; i < 10; i++ {
		lm.RecordTaskProcessed("test-queue", true, 100*time.Millisecond)
	}

	health := lm.GetHealthStatus()

	// 验证健康状态
	if health["status"] != "healthy" {
		t.Errorf("status = %v, want %v", health["status"], "healthy")
	}

	if health["tasks_processed"] != int64(10) {
		t.Errorf("tasks_processed = %v, want %v", health["tasks_processed"], 10)
	}

	if health["success_rate"] != 100.0 {
		t.Errorf("success_rate = %v, want %v", health["success_rate"], 100.0)
	}
}

func TestLoadMonitor_StartStop(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.LoadMonitorConfig{
		UpdateInterval: 100 * time.Millisecond,
	}

	lm := NewLoadMonitor(cfg, logger)

	ctx := context.Background()

	// 启动
	err := lm.Start(ctx)
	if err != nil {
		t.Fatalf("启动失败: %v", err)
	}

	// 等待一小段时间
	time.Sleep(200 * time.Millisecond)

	// 停止
	stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = lm.Stop(stopCtx)
	if err != nil {
		t.Fatalf("停止失败: %v", err)
	}
}

func TestLoadMonitor_GetMetricsCollector(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.LoadMonitorConfig{
		UpdateInterval: 1 * time.Second,
	}

	lm := NewLoadMonitor(cfg, logger)

	collector := lm.GetMetricsCollector()

	if collector == nil {
		t.Error("GetMetricsCollector 应该返回非 nil 实例")
	}
}

func TestLoadMonitor_ProcessingTimeStats(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.LoadMonitorConfig{
		UpdateInterval: 1 * time.Second,
	}

	lm := NewLoadMonitor(cfg, logger)

	// 记录不同处理时间的任务
	lm.RecordTaskProcessed("test-queue", true, 100*time.Millisecond)
	lm.RecordTaskProcessed("test-queue", true, 200*time.Millisecond)
	lm.RecordTaskProcessed("test-queue", true, 300*time.Millisecond)

	stats := lm.GetStats()

	// 验证队列统计中的处理时间
	queueStats, exists := stats.QueueStats["test-queue"]
	if !exists {
		t.Fatal("test-queue 统计信息不存在")
	}

	if queueStats.ProcessingStats.Count != 3 {
		t.Errorf("Count = %v, want %v", queueStats.ProcessingStats.Count, 3)
	}

	// 平均值应该是 200ms
	expectedAvg := 200 * time.Millisecond
	if queueStats.ProcessingStats.Average != expectedAvg {
		t.Errorf("Average = %v, want %v", queueStats.ProcessingStats.Average, expectedAvg)
	}

	// 最大值应该是 300ms
	if queueStats.ProcessingStats.Max != 300*time.Millisecond {
		t.Errorf("Max = %v, want %v", queueStats.ProcessingStats.Max, 300*time.Millisecond)
	}

	// 最小值应该是 100ms
	if queueStats.ProcessingStats.Min != 100*time.Millisecond {
		t.Errorf("Min = %v, want %v", queueStats.ProcessingStats.Min, 100*time.Millisecond)
	}
}

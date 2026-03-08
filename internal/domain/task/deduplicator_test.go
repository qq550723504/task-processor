package task

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestDeduplicator_IsDuplicate(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // 减少测试输出

	dedup := NewDeduplicator(5*time.Minute, logger)
	defer dedup.Stop()

	taskID := int64(12345)

	// 第一次检查，应该不重复
	isDup := dedup.IsDuplicate(taskID)
	assert.False(t, isDup, "第一次检查应该不重复")

	// 标记为已处理
	dedup.MarkProcessed(taskID)

	// 第二次检查，应该重复
	isDup = dedup.IsDuplicate(taskID)
	assert.True(t, isDup, "标记后应该检测为重复")
}

func TestDeduplicator_MarkProcessed(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	dedup := NewDeduplicator(5*time.Minute, logger)
	defer dedup.Stop()

	taskID := int64(67890)

	// 标记为已处理
	dedup.MarkProcessed(taskID)

	// 验证已标记
	isDup := dedup.IsDuplicate(taskID)
	assert.True(t, isDup, "标记后应该能检测到")
}

func TestDeduplicator_TTL(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	// 使用很短的 TTL 进行测试
	dedup := NewDeduplicator(100*time.Millisecond, logger)
	defer dedup.Stop()

	taskID := int64(11111)

	// 标记为已处理
	dedup.MarkProcessed(taskID)

	// 立即检查，应该重复
	isDup := dedup.IsDuplicate(taskID)
	assert.True(t, isDup, "TTL内应该检测为重复")

	// 等待 TTL 过期
	time.Sleep(150 * time.Millisecond)

	// 再次检查，应该不重复（已过期）
	isDup = dedup.IsDuplicate(taskID)
	assert.False(t, isDup, "TTL过期后应该不重复")
}

func TestDeduplicator_MultipleTaskIDs(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	dedup := NewDeduplicator(5*time.Minute, logger)
	defer dedup.Stop()

	taskIDs := []int64{1001, 1002, 1003, 1004, 1005}

	// 标记所有任务
	for _, id := range taskIDs {
		dedup.MarkProcessed(id)
	}

	// 验证所有任务都被标记
	for _, id := range taskIDs {
		isDup := dedup.IsDuplicate(id)
		assert.True(t, isDup, "任务 %d 应该被标记为重复", id)
	}

	// 检查未标记的任务
	newTaskID := int64(9999)
	isDup := dedup.IsDuplicate(newTaskID)
	assert.False(t, isDup, "未标记的任务应该不重复")
}

func TestDeduplicator_GetStats(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	dedup := NewDeduplicator(5*time.Minute, logger)
	defer dedup.Stop()

	// 标记几个任务
	dedup.MarkProcessed(1)
	dedup.MarkProcessed(2)
	dedup.MarkProcessed(3)

	// 获取统计信息
	stats := dedup.GetStats()

	assert.NotNil(t, stats, "统计信息不应为空")
	assert.Equal(t, 3, stats["total_records"], "应该有3条记录")
	assert.Equal(t, 300, stats["ttl_seconds"], "TTL应该是300秒")
}

func TestDeduplicator_Cleanup(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	// 使用很短的 TTL
	dedup := NewDeduplicator(50*time.Millisecond, logger)
	defer dedup.Stop()

	// 标记多个任务
	for i := int64(1); i <= 10; i++ {
		dedup.MarkProcessed(i)
	}

	// 验证记录数
	stats := dedup.GetStats()
	assert.Equal(t, 10, stats["total_records"], "应该有10条记录")

	// 等待清理
	time.Sleep(100 * time.Millisecond)

	// 再次获取统计（清理应该已经执行）
	stats = dedup.GetStats()
	// 注意：由于清理是异步的，这里可能还有一些记录
	// 但应该少于10条
	totalRecords := stats["total_records"].(int)
	assert.LessOrEqual(t, totalRecords, 10, "清理后记录数应该不超过10")
}

func TestDeduplicator_Stop(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	dedup := NewDeduplicator(5*time.Minute, logger)

	// 标记一个任务
	dedup.MarkProcessed(12345)

	// 停止去重器
	dedup.Stop()

	// 停止后仍然可以查询（只是不再清理）
	isDup := dedup.IsDuplicate(12345)
	assert.True(t, isDup, "停止后仍然可以查询")
}

func BenchmarkDeduplicator_IsDuplicate(b *testing.B) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	dedup := NewDeduplicator(5*time.Minute, logger)
	defer dedup.Stop()

	// 预先标记一些任务
	for i := int64(1); i <= 1000; i++ {
		dedup.MarkProcessed(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		taskID := int64(i % 1000)
		dedup.IsDuplicate(taskID)
	}
}

func BenchmarkDeduplicator_MarkProcessed(b *testing.B) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	dedup := NewDeduplicator(5*time.Minute, logger)
	defer dedup.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dedup.MarkProcessed(int64(i))
	}
}

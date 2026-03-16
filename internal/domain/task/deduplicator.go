// Package task 提供任务领域相关的业务规则
package task

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Deduplicator 任务去重器（业务规则）
// 负责防止重复任务的提交和处理
type Deduplicator struct {
	records sync.Map // taskID -> processedTime
	ttl     time.Duration
	logger  *logrus.Logger
	stopCh  chan struct{}
}

// NewDeduplicator 创建任务去重器
func NewDeduplicator(ttl time.Duration, logger *logrus.Logger) *Deduplicator {
	if ttl == 0 {
		ttl = 10 * time.Minute // 默认10分钟
	}

	td := &Deduplicator{
		ttl:    ttl,
		logger: logger,
		stopCh: make(chan struct{}),
	}

	// 启动清理goroutine
	go td.cleanupLoop()

	return td
}

// IsDuplicate 检查任务是否重复
func (td *Deduplicator) IsDuplicate(taskID int64) bool {
	if _, exists := td.records.Load(taskID); exists {
		td.logger.Debugf("检测到重复任务: taskID=%d", taskID)
		return true
	}
	return false
}

// MarkProcessed 标记任务已处理
func (td *Deduplicator) MarkProcessed(taskID int64) {
	td.records.Store(taskID, time.Now())
	td.logger.Debugf("标记任务已处理: taskID=%d", taskID)
}

// cleanupLoop 定期清理过期记录
func (td *Deduplicator) cleanupLoop() {
	ticker := time.NewTicker(td.ttl / 2) // 每半个TTL清理一次
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			td.cleanup()
		case <-td.stopCh:
			td.logger.Info("任务去重器清理goroutine已停止")
			return
		}
	}
}

// cleanup 清理过期记录
func (td *Deduplicator) cleanup() {
	now := time.Now()
	expiredCount := 0

	td.records.Range(func(key, value any) bool {
		processedTime := value.(time.Time)
		if now.Sub(processedTime) > td.ttl {
			td.records.Delete(key)
			expiredCount++
		}
		return true
	})

	if expiredCount > 0 {
		td.logger.Debugf("清理了 %d 条过期去重记录", expiredCount)
	}
}

// Stop 停止去重器
func (td *Deduplicator) Stop() {
	close(td.stopCh)
}

// GetStats 获取统计信息
func (td *Deduplicator) GetStats() map[string]any {
	count := 0
	td.records.Range(func(_, _ any) bool {
		count++
		return true
	})

	return map[string]any{
		"total_records": count,
		"ttl_seconds":   int(td.ttl.Seconds()),
	}
}

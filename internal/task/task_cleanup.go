// Package task 提供任务清理功能
package task

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
)

// cleanupExpiredTasks 定期清理过期的任务记录
func (f *TaskFetcher) cleanupExpiredTasks(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("清理过期任务goroutine panic: %v", r)
		}
	}()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logrus.Info("清理过期任务协程停止")
			return
		case <-ticker.C:
			f.performCleanup()
		}
	}
}

// performCleanup 执行清理操作
func (f *TaskFetcher) performCleanup() {
	f.tasksMutex.Lock()
	defer f.tasksMutex.Unlock()

	now := time.Now()
	expiredCount := 0

	// 清理超过30分钟的任务记录（任务处理超时应该在15分钟内）
	for taskID, submitTime := range f.processingTasks {
		if now.Sub(submitTime) > 30*time.Minute {
			delete(f.processingTasks, taskID)
			expiredCount++
		}
	}

	totalTasks := len(f.processingTasks)

	if expiredCount > 0 {
		logrus.Infof("🧹 清理过期任务记录: 清理=%d, 剩余=%d", expiredCount, totalTasks)
	}
}

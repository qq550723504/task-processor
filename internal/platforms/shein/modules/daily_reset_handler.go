package modules

import (
	"context"
	"task-processor/internal/common/memory"
	"task-processor/internal/goroutine"
	"time"

	"github.com/sirupsen/logrus"
)

// DailyResetHandler 每日重置处理器
type DailyResetHandler struct {
	memoryManager *memory.MemoryManager
}

// NewDailyResetHandler 创建新的每日重置处理器
func NewDailyResetHandler(memoryManager *memory.MemoryManager) *DailyResetHandler {
	return &DailyResetHandler{
		memoryManager: memoryManager,
	}
}

// Start 启动每日重置任务
func (h *DailyResetHandler) Start(ctx context.Context) {
	// 使用统一的goroutine管理器
	goroutineManager := goroutine.NewGoroutineManager(ctx, logrus.WithField("component", "daily_reset_handler"))

	goroutineManager.Start("daily_reset_scheduler", func(ctx context.Context) error {
		// 等待到下一个UTC时间的00:00:00
		now := time.Now().UTC()
		nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
		duration := nextMidnight.Sub(now)

		logrus.Infof("每日重置任务将在 %v 后启动", duration)

		// 等待到下一个午夜
		timer := time.NewTimer(duration)
		select {
		case <-timer.C:
			// 执行每日重置
			h.performDailyReset(ctx)
		case <-ctx.Done():
			// 上下文被取消，退出
			timer.Stop()
			return ctx.Err()
		}

		// 之后每24小时执行一次
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// 执行每日重置
				h.performDailyReset(ctx)
			case <-ctx.Done():
				// 上下文被取消，退出
				return ctx.Err()
			}
		}
	})
}

// performDailyReset 执行每日重置操作
func (h *DailyResetHandler) performDailyReset(ctx context.Context) {
	logrus.Info("开始执行每日重置任务")

	// 注意：DailyCountManager会自动处理日期切换
	// 这里主要是清理过期的暂停状态
	h.cleanupExpiredPauseStates()

	logrus.Info("每日重置任务执行完成")
}

// cleanupExpiredPauseStates 清理过期的暂停状态
func (h *DailyResetHandler) cleanupExpiredPauseStates() {
	if h.memoryManager == nil || h.memoryManager.ShopPauseManager == nil {
		logrus.Warn("内存管理器未初始化，跳过清理过期暂停状态")
		return
	}

	// ShopPauseManager会自动处理过期状态
	// 这里只是触发一次清理检查
	logrus.Debug("触发暂停状态清理检查")

	// 可以添加统计信息
	stats := h.memoryManager.GetStats()
	logrus.Infof("当前暂停店铺数: %d", stats["paused_shops"])
}

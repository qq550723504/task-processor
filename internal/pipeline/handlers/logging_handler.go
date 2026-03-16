// Package handlers 提供公共处理器实现
package handlers

import (
	"task-processor/internal/pipeline"
	"task-processor/internal/pkg/recovery"
	"time"
)

// LoggingHandler 通用日志处理器
type LoggingHandler struct {
	*pipeline.BaseHandler
	logLevel string
}

// NewLoggingHandler 创建日志处理器
func NewLoggingHandler(logLevel string) pipeline.Handler {
	return &LoggingHandler{
		BaseHandler: pipeline.NewBaseHandler("通用日志处理器"),
		logLevel:    logLevel,
	}
}

// Handle 执行日志处理
func (h *LoggingHandler) Handle(ctx pipeline.TaskContext) error {
	h.LogStart()
	defer recovery.Recover("日志处理器", h.GetLogger())

	// 验证上下文
	if err := h.ValidateContext(ctx); err != nil {
		h.LogError(err)
		return err
	}

	task := ctx.GetTask()

	// 记录任务详细信息
	h.GetLogger().WithFields(map[string]any{
		"task_id":    task.ID,
		"product_id": task.ProductID,
		"store_id":   task.StoreID,
		"platform":   task.Platform,
		"status":     task.Status,
		"created_at": task.CreateTime,
	}).Infof("任务详细信息记录")

	// 记录上下文数据（如果需要）
	if h.logLevel == "debug" {
		// 尝试获取Amazon上下文
		if amazonCtx, ok := ctx.(pipeline.AmazonContext); ok {
			if amazonProduct := amazonCtx.GetAmazonProduct(); amazonProduct != nil {
				h.GetLogger().Debugf("Amazon产品信息: ASIN=%s, Title=%s",
					amazonProduct.Asin, amazonProduct.Title)
			}
		}
	}

	// 设置日志记录标记
	h.SetResult(ctx, "logged", true)
	h.SetResult(ctx, "log_time", time.Now())

	h.LogSuccess()
	return nil
}

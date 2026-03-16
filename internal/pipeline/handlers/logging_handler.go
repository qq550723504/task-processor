// Package handlers 提供公共处理器实现
package handlers

import (
	"fmt"
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
	h.Logger().Infof("开始执行处理器: %s", h.Name())
	defer recovery.Recover(h.Name(), h.Logger())

	task := ctx.GetTask()
	if task == nil {
		return fmt.Errorf("任务信息为空")
	}

	h.Logger().WithFields(map[string]any{
		"task_id":    task.ID,
		"product_id": task.ProductID,
		"store_id":   task.StoreID,
		"platform":   task.Platform,
		"status":     task.Status,
		"created_at": task.CreateTime,
	}).Info("任务详细信息记录")

	if h.logLevel == "debug" {
		if amazonCtx, ok := ctx.(pipeline.AmazonContext); ok {
			if p := amazonCtx.GetAmazonProduct(); p != nil {
				h.Logger().Debugf("Amazon产品信息: ASIN=%s, Title=%s", p.Asin, p.Title)
			}
		}
	}

	ctx.SetData("logged", true)
	ctx.SetData("log_time", time.Now())

	h.Logger().Infof("处理器执行成功: %s", h.Name())
	return nil
}

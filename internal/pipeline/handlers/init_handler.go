// Package handlers 提供公共处理器实现
package handlers

import (
	"task-processor/internal/pipeline"
)

// InitHandler 通用初始化处理器
type InitHandler struct {
	*pipeline.BaseHandler
}

// NewInitHandler 创建初始化处理器
func NewInitHandler() pipeline.Handler {
	return &InitHandler{
		BaseHandler: pipeline.NewBaseHandler("通用初始化处理器"),
	}
}

// Handle 执行初始化处理
func (h *InitHandler) Handle(ctx pipeline.TaskContext) error {
	h.LogStart()
	defer func() {
		if err := recover(); err != nil {
			h.GetLogger().Errorf("初始化处理器发生panic: %v", err)
		}
	}()

	// 验证上下文
	if err := h.ValidateContext(ctx); err != nil {
		h.LogError(err)
		return err
	}

	task := ctx.GetTask()
	h.GetLogger().Infof("初始化任务: ID=%d, ProductID=%s, Platform=%s",
		task.ID, task.ProductID, task.Platform)

	// 设置初始化标记
	h.SetResult(ctx, "initialized", true)
	h.SetResult(ctx, "init_time", task.CreateTime)

	h.LogSuccess()
	return nil
}

// Package handlers 提供公共处理器实现
package handlers

import (
	"fmt"
	"task-processor/internal/pipeline"
	"task-processor/internal/pkg/recovery"
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
	h.Logger().Infof("开始执行处理器: %s", h.Name())
	defer recovery.Recover(h.Name(), h.Logger())

	task := ctx.GetTask()
	if task == nil {
		return fmt.Errorf("任务信息为空")
	}

	h.Logger().Infof("初始化任务: ID=%d, ProductID=%s, Platform=%s",
		task.ID, task.ProductID, task.Platform)

	ctx.SetData("initialized", true)
	ctx.SetData("init_time", task.CreateTime)

	h.Logger().Infof("处理器执行成功: %s", h.Name())
	return nil
}

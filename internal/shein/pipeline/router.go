package pipeline

import (
	"task-processor/internal/core/logger"
	"fmt"
	"strings"
	"task-processor/internal/shein"

)

// PlatformRouterHandler 平台路由处理器
// 根据任务的平台类型决定后续处理流程
type PlatformRouterHandler struct{}

// NewPlatformRouterHandler 创建平台路由处理器
func NewPlatformRouterHandler() *PlatformRouterHandler {
	return &PlatformRouterHandler{}
}

// Name 返回处理器名称
func (h *PlatformRouterHandler) Name() string {
	return "平台路由处理器"
}

// Handle 处理任务
func (h *PlatformRouterHandler) Handle(ctx *shein.TaskContext) error {
	task := ctx.Task
	platform := strings.ToLower(task.Platform)

	logger.GetGlobalLogger("shein/pipeline").Infof("[PlatformRouter] 检测到平台: %s, ProductID: %s", platform, task.ProductID)

	// 根据平台类型设置标记
	switch platform {
	case "amazon":
		// Amazon平台任务
		logger.GetGlobalLogger("shein/pipeline").Infof("[PlatformRouter] 任务将使用Amazon处理流程")
		// 在上下文中设置标记，后续handler可以根据此标记决定是否执行
		if ctx.Extra == nil {
			ctx.Extra = make(map[string]any)
		}
		ctx.Extra["platform"] = "amazon"
		ctx.Extra["skipSheinPipeline"] = true
		return nil

	case "shein", "":
		// Shein平台任务（默认）
		logger.GetGlobalLogger("shein/pipeline").Infof("[PlatformRouter] 任务将使用Shein处理流程")
		if ctx.Extra == nil {
			ctx.Extra = make(map[string]any)
		}
		ctx.Extra["platform"] = "shein"
		return nil

	default:
		return fmt.Errorf("不支持的平台: %s", platform)
	}
}

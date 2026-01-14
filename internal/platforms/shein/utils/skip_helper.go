package utils

// TaskContext 任务上下文
type TaskContext struct {
	Extra map[string]interface{}
}

// ShouldSkipForAmazon 检查是否应该跳过Shein处理流程（用于Amazon任务）
func ShouldSkipForAmazon(ctx *TaskContext) bool {
	if ctx == nil || ctx.Extra == nil {
		return false
	}

	// 检查是否为Amazon平台
	if platform, ok := ctx.Extra["platform"].(string); ok && platform == "amazon" {
		return true
	}

	// 检查是否已经被Amazon处理器处理过
	if processed, ok := ctx.Extra["amazonProcessed"].(bool); ok && processed {
		return true
	}

	return false
}

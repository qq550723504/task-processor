package modules

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

// ReListingHandler 重新上架任务处理器
type ReListingHandler struct {
}

// NewReListingHandler 创建新的重新上架任务处理器
func NewReListingHandler() *ReListingHandler {
	return &ReListingHandler{}
}

// Name 返回处理器名称
func (h *ReListingHandler) Name() string {
	return "重新上架任务处理器"
}

// Handle 执行重新上架任务处理
func (h *ReListingHandler) Handle(ctx *TaskContext) error {
	// 这个处理器主要用于处理状态为7的重新上架任务
	// 在实际应用中，可能需要从特定队列中获取这些任务并处理

	// 检查任务是否为重新上架任务（状态7）
	if h.isReListingTask(ctx.Task) {
		logrus.Infof("处理重新上架任务: TaskID=%d, ProductID=%s", ctx.Task.ID, ctx.Task.ProductID)

		// 可以在这里添加重新上架的具体逻辑
		// 例如：检查产品当前状态、重新验证变体信息等

		// 示例：记录重新上架时间
		logrus.Infof("任务 %d 已重新上架，原始上架时间: %d", ctx.Task.ID, ctx.Task.CreateTime)
	}

	return nil
}

// isReListingTask 检查是否为重新上架任务
func (h *ReListingHandler) isReListingTask(task *Task) bool {
	// 检查任务中是否包含状态7的标识
	// 注意：这需要任务数据结构支持状态字段
	taskData, _ := json.Marshal(task)
	taskMap := make(map[string]interface{})
	if err := json.Unmarshal(taskData, &taskMap); err == nil {
		if status, ok := taskMap["status"]; ok {
			if statusFloat, ok := status.(float64); ok {
				return int(statusFloat) == 7
			}
			if statusInt, ok := status.(int); ok {
				return statusInt == 7
			}
			if statusStr, ok := status.(string); ok {
				return statusStr == "7"
			}
		}
	}
	return false
}

// markTaskAsReListing 将任务标记为待重新上架
func (h *ReListingHandler) markTaskAsReListing(ctx *TaskContext, reason string) error {
	if ctx.MemoryManager == nil || ctx.Task == nil {
		return fmt.Errorf("内存管理器或任务信息不可用")
	}

	// 创建重新上架任务数据
	reListingTask := map[string]interface{}{
		"taskId":       ctx.Task.ID,
		"tenantId":     ctx.Task.TenantID,
		"productId":    ctx.Task.ProductID,
		"platform":     ctx.Task.Platform,
		"region":       ctx.Task.Region,
		"storeId":      ctx.Task.StoreID,
		"categoryId":   ctx.Task.CategoryID,
		"createTime":   ctx.Task.CreateTime,
		"retryCount":   ctx.Task.RetryCount,
		"priority":     ctx.Task.Priority,
		"creator":      ctx.Task.Creator,
		"status":       7,                 // 状态7表示待重新上架
		"relistTime":   time.Now().Unix(), // 记录重新上架时间
		"relistReason": reason,
	}

	// 序列化任务数据
	taskJSON, err := json.Marshal(reListingTask)
	if err != nil {
		return fmt.Errorf("序列化重新上架任务数据失败: %w", err)
	}

	// 添加到内存重新上架队列
	ctx.MemoryManager.ReListingQueue.PushTask(
		ctx.Task.TenantID,
		ctx.Task.StoreID,
		string(taskJSON),
	)

	// 不再需要标记任务为完成状态
	// 任务状态由API管理
	logrus.Infof("原始任务已添加到重新上架队列: TaskID=%d", ctx.Task.ID)

	logrus.Infof("任务 %d 已标记为待重新上架，原因: %s", ctx.Task.ID, reason)
	return nil
}

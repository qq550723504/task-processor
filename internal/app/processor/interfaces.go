// Package processor 提供处理器相关接口定义
package processor

import (
	"context"
	"task-processor/internal/model"
)

// VariantTaskSubmitter 变体任务提交器接口
// 用于提交产品变体任务（Amazon特定业务逻辑）
type VariantTaskSubmitter interface {
	SubmitVariantTasks(ctx context.Context, parentTask *model.Task, variations []model.Variation, parentAsin string) (successCount, failCount int)
}

package processor

import (
	"context"
	"task-processor/internal/model"
)

// VariantTaskSubmitter 变体任务提交器接口
type VariantTaskSubmitter interface {
	SubmitVariantTasks(ctx context.Context, parentTask *model.Task, variations []model.Variation, parentAsin string) (successCount, failCount int)
}

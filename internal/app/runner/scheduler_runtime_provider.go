package runner

import (
	"context"

	"task-processor/internal/app/scheduler"
	"task-processor/internal/listingruntime"
)

type SchedulerRuntimeProvider interface {
	GetRuntimeStoreService() listingruntime.StoreService
	ListRuntimeAutoPricingStoreIDs(ctx context.Context, platform string) ([]int64, error)
	ListRuntimeScheduledTaskConfigs(ctx context.Context, platform string, taskType scheduler.TaskType) ([]listingruntime.ScheduledTaskConfig, error)
}

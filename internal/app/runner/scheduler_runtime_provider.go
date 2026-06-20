package runner

import (
	"context"

	"task-processor/internal/listingruntime"
)

type SchedulerRuntimeProvider interface {
	GetRuntimeStoreService() listingruntime.StoreService
	ListRuntimeAutoPricingStoreIDs(ctx context.Context, platform string) ([]int64, error)
}

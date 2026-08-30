package local

import (
	"context"
	"fmt"

	"task-processor/internal/app/scheduler"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingruntime"
)

func (r *LocalRuntime) ListRuntimeScheduledTaskConfigs(ctx context.Context, platform string, taskType scheduler.TaskType) ([]listingruntime.ScheduledTaskConfig, error) {
	if r == nil || (r.resources == nil && r.provider == nil) {
		return nil, fmt.Errorf("local listing runtime is not initialized")
	}
	repo := r.scheduledTaskConfigRepository()
	if repo == nil {
		return nil, fmt.Errorf("scheduled task config repository is not configured")
	}
	items, err := repo.ListEnabledScheduledTaskConfigs(ctx, platform, string(taskType))
	if err != nil {
		return nil, err
	}
	result := make([]listingruntime.ScheduledTaskConfig, 0, len(items))
	for _, item := range items {
		result = append(result, listingruntime.ScheduledTaskConfig{
			TenantID:        item.TenantID,
			StoreID:         item.StoreID,
			Platform:        item.Platform,
			TaskType:        item.TaskType,
			Enabled:         item.Enabled,
			IntervalSeconds: item.IntervalSeconds,
		})
	}
	return result, nil
}

func (r *LocalRuntime) ListRuntimeScheduledTaskConfigStates(ctx context.Context, platform string, taskType scheduler.TaskType) ([]listingruntime.ScheduledTaskConfig, error) {
	if r == nil || (r.resources == nil && r.provider == nil) {
		return nil, fmt.Errorf("local listing runtime is not initialized")
	}
	repo := r.scheduledTaskConfigRepository()
	if repo == nil {
		return nil, fmt.Errorf("scheduled task config repository is not configured")
	}

	pageNo := 1
	result := make([]listingruntime.ScheduledTaskConfig, 0)
	for {
		page, err := repo.ListScheduledTaskConfigs(ctx, listingadmin.ScheduledTaskConfigQuery{
			Page:     pageNo,
			PageSize: 500,
			Platform: platform,
			TaskType: string(taskType),
		})
		if err != nil {
			return nil, err
		}
		if page == nil || len(page.Items) == 0 {
			break
		}
		for _, item := range page.Items {
			result = append(result, listingruntime.ScheduledTaskConfig{
				TenantID:        item.TenantID,
				StoreID:         item.StoreID,
				Platform:        item.Platform,
				TaskType:        item.TaskType,
				Enabled:         item.Enabled,
				IntervalSeconds: item.IntervalSeconds,
			})
		}
		if page.Total > 0 && int64(page.Page*page.PageSize) >= page.Total {
			break
		}
		if len(page.Items) < page.PageSize {
			break
		}
		pageNo++
	}
	return result, nil
}

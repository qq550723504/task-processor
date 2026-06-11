package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

func backfillRetryableBlockedTasks(ctx context.Context, repo Repository, createdAfter time.Time) (int, error) {
	if repo == nil {
		return 0, fmt.Errorf("task recovery repository is not configured")
	}

	tasks, _, err := repo.ListTasks(ctx, &TaskListQuery{
		Status:   string(TaskStatusFailed),
		Page:     1,
		PageSize: taskRecoveryBackfillPageSize,
	})
	if err != nil {
		return 0, err
	}

	backfilledAt := time.Now().UTC()
	count := 0
	for i := range tasks {
		task := tasks[i]
		if !createdAfter.IsZero() && task.CreatedAt.Before(createdAfter) {
			continue
		}
		if strings.TrimSpace(task.Error) == "" {
			continue
		}
		block, ok := classifyRetryableTaskFailure(errors.New(task.Error))
		if !ok {
			continue
		}
		block.BlockedAt = task.UpdatedAt.UTC()
		if block.BlockedAt.IsZero() {
			block.BlockedAt = backfilledAt
		}
		block.RetryAttempts = 0
		block.LastRetryAt = nil
		block.AutoRetryPaused = false
		block.MaxAutoRetryAttempts = taskRecoveryBackfillMaxAutoRetryAttempt
		nextRetryAt := backfilledAt.Add(boundedRecoveryRetryDelay(1))
		block.NextRetryAt = cloneTimePointer(nextRetryAt)
		if err := repo.MarkBlockedRetryable(ctx, task.ID, block, task.Error); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

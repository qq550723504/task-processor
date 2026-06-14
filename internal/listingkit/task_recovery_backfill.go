package listingkit

import (
	"context"
	"fmt"
	"time"

	listingsubmission "task-processor/internal/listing/submission"
)

func backfillRetryableBlockedTasks(ctx context.Context, repo Repository, createdAfter time.Time) (int, error) {
	if repo == nil {
		return 0, fmt.Errorf("task recovery repository is not configured")
	}
	service := listingsubmission.NewRetryableBackfillService(listingsubmission.RetryableBackfillServiceConfig[Task]{
		ListFailedTasks: func(ctx context.Context) ([]Task, error) {
			tasks, _, err := repo.ListTasks(ctx, &TaskListQuery{
				Status:   string(TaskStatusFailed),
				Page:     1,
				PageSize: taskRecoveryBackfillPageSize,
			})
			return tasks, err
		},
		Record: func(task Task) listingsubmission.RetryableBackfillRecord {
			return listingsubmission.RetryableBackfillRecord{
				ID:        task.ID,
				Error:     task.Error,
				CreatedAt: task.CreatedAt,
				UpdatedAt: task.UpdatedAt,
			}
		},
		MarkBlockedRetryable: func(ctx context.Context, task Task, block *listingsubmission.RetryableBlockState, errorMsg string) error {
			return repo.MarkBlockedRetryable(ctx, task.ID, adaptSubmissionRetryableBlock(block), errorMsg)
		},
		Now:                  func() time.Time { return time.Now().UTC() },
		MaxAutoRetryAttempts: taskRecoveryBackfillMaxAutoRetryAttempt,
		DefaultRecoveryScope: retryableRecoveryScopeTask,
	})
	return service.Backfill(ctx, listingsubmission.RetryableBackfillRequest{
		CreatedAfter: createdAfter,
	})
}

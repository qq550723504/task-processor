package listingkit

import (
	"context"

	submissiondomain "task-processor/internal/listing/submission"
)

func markTaskBlockedRetryableState(ctx context.Context, repo Repository, taskID string, block *submissiondomain.RetryableBlockState, errorMsg string) error {
	return repo.MarkBlockedRetryable(ctx, taskID, adaptSubmissionRetryableBlock(block), errorMsg)
}

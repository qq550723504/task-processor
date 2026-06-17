package listingkit

import (
	"context"
	"errors"
	"fmt"
	"time"

	submissiondomain "task-processor/internal/listing/submission"
)

func (s *taskRecoveryService) restoreRecoveryDurability(ctx context.Context, taskID string, previousBlock *RetryableBlock, errorMsg string, submitErr error, persistErr error) error {
	joined := errors.Join(fmt.Errorf("submit recovered task %s: %w", taskID, submitErr), persistErr)

	restoredAt := time.Time{}
	if previousBlock == nil || previousBlock.BlockedAt.IsZero() {
		restoredAt = s.currentTime()
	}
	restoreBlock, ok := submissiondomain.BuildRecoveryDurabilityRestoreBlock(
		adaptRetryableBlockState(previousBlock),
		submitErr,
		restoredAt,
		submissiondomain.RetryableRecoveryScopeTask,
	)
	if !ok {
		return joined
	}
	if rollbackErr := markTaskBlockedRetryableState(ctx, s.repo, taskID, restoreBlock, errorMsg); rollbackErr != nil {
		return errors.Join(joined, fmt.Errorf("restore blocked retryable state: %w", rollbackErr))
	}
	return joined
}

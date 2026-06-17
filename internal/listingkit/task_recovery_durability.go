package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	submissiondomain "task-processor/internal/listing/submission"
)

func (s *taskRecoveryService) restoreRecoveryDurability(ctx context.Context, taskID string, previousBlock *RetryableBlock, errorMsg string, submitErr error, persistErr error) error {
	joined := errors.Join(fmt.Errorf("submit recovered task %s: %w", taskID, submitErr), persistErr)

	restoreBlock := cloneRetryableBlock(previousBlock)
	if restoreBlock == nil {
		if classified, ok := classifyRetryableTaskFailure(submitErr); ok {
			restoreBlock = cloneRetryableBlock(classified)
		}
	}
	if restoreBlock == nil {
		return joined
	}
	if strings.TrimSpace(restoreBlock.RecoveryScope) == "" {
		restoreBlock.RecoveryScope = submissiondomain.RetryableRecoveryScopeTask
	}
	if restoreBlock.BlockedAt.IsZero() {
		restoreBlock.BlockedAt = s.currentTime()
	}
	if rollbackErr := s.repo.MarkBlockedRetryable(ctx, taskID, restoreBlock, errorMsg); rollbackErr != nil {
		return errors.Join(joined, fmt.Errorf("restore blocked retryable state: %w", rollbackErr))
	}
	return joined
}

func (s *taskRecoveryService) buildReblockedTask(previous *RetryableBlock, classified *RetryableBlock, recoveredAt time.Time) *RetryableBlock {
	return adaptSubmissionRetryableBlock(submissiondomain.BuildReblockedRetryableBlock(
		adaptRetryableBlockState(previous),
		adaptRetryableBlockState(classified),
		recoveredAt,
		submissiondomain.RetryableRecoveryScopeTask,
	))
}

func cloneTimePointer(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copied := value
	return &copied
}

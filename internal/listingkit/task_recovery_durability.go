package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	listingsubmission "task-processor/internal/listing/submission"
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
		restoreBlock.RecoveryScope = retryableRecoveryScopeTask
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
	block := cloneRetryableBlock(previous)
	if block == nil {
		block = cloneRetryableBlock(classified)
	}
	if block == nil {
		block = &RetryableBlock{}
	}
	if classified != nil {
		if strings.TrimSpace(classified.ReasonCode) != "" {
			block.ReasonCode = strings.TrimSpace(classified.ReasonCode)
		}
		if strings.TrimSpace(classified.ReasonMessage) != "" {
			block.ReasonMessage = strings.TrimSpace(classified.ReasonMessage)
		}
		if strings.TrimSpace(classified.RecoveryScope) != "" {
			block.RecoveryScope = strings.TrimSpace(classified.RecoveryScope)
		}
		if block.ReasonCode == "" && classified.AutoResumeEnabled {
			block.AutoResumeEnabled = true
		}
	}
	if block.BlockedAt.IsZero() {
		block.BlockedAt = recoveredAt
	}
	block.RetryAttempts++
	block.LastRetryAt = cloneTimePointer(recoveredAt)
	if strings.TrimSpace(block.RecoveryScope) == "" {
		block.RecoveryScope = retryableRecoveryScopeTask
	}
	if block.AutoRetryPaused {
		block.NextRetryAt = nil
		return block
	}
	if block.MaxAutoRetryAttempts > 0 && block.RetryAttempts >= block.MaxAutoRetryAttempts {
		block.AutoRetryPaused = true
		block.NextRetryAt = nil
		return block
	}
	if block.AutoResumeEnabled {
		nextRetryAt := recoveredAt.Add(boundedRecoveryRetryDelay(block.RetryAttempts))
		block.NextRetryAt = cloneTimePointer(nextRetryAt)
	} else {
		block.NextRetryAt = nil
	}
	return block
}

func boundedRecoveryRetryDelay(attempt int) time.Duration {
	return listingsubmission.BoundedEnqueueRetryDelay(attempt)
}

func cloneTimePointer(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copied := value
	return &copied
}

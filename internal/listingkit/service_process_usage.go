package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (s *service) generationUsageSettlement() GenerationUsageSettlement {
	if s == nil {
		return nil
	}
	return s.taskDeps.generationUsage
}

func (s *service) reserveGenerationUsage(ctx context.Context, task *Task) (GenerationUsageReservation, bool, error) {
	settlement := s.generationUsageSettlement()
	if settlement == nil || task == nil {
		return GenerationUsageReservation{}, false, nil
	}
	// A new reservation belongs to the period in which generation is actually
	// claimed. The ledger resolves an existing idempotency key first and keeps
	// its persisted period/occurrence for replays, so delayed tasks cannot be
	// charged to their creation month while retries remain idempotent.
	occurredAt := time.Now().UTC()
	reservation, err := settlement.ReserveGeneration(ctx, task.TenantID, task.ID, occurredAt)
	if err != nil {
		return GenerationUsageReservation{}, true, err
	}
	return reservation, true, nil
}

func (s *service) releaseGenerationUsage(ctx context.Context, task *Task, reason string) error {
	settlement := s.generationUsageSettlement()
	if settlement == nil || task == nil {
		return nil
	}
	return settlement.ReleaseGeneration(ctx, task.TenantID, task.ID, strings.TrimSpace(reason))
}

func (s *service) commitGenerationUsage(ctx context.Context, task *Task) error {
	settlement := s.generationUsageSettlement()
	if settlement == nil || task == nil {
		return nil
	}
	return settlement.CommitGeneration(ctx, task.TenantID, task.ID)
}

func (s *service) handleGenerationTerminalPersistenceFailure(ctx context.Context, task *Task, persistErr error) error {
	if task == nil {
		return persistErr
	}
	var errs []error
	if persistErr != nil {
		errs = append(errs, persistErr)
	}
	if releaseErr := s.releaseGenerationUsage(ctx, task, "terminal_persistence_failed"); releaseErr != nil {
		errs = append(errs, releaseErr)
		blockErr := s.markGenerationUsageReleasePending(ctx, task, persistErr, releaseErr)
		if blockErr != nil {
			errs = append(errs, blockErr)
		}
		return errors.Join(errs...)
	}
	if markErr := s.repo.MarkFailed(ctx, task.ID, "listing kit generation result persistence failed"); markErr != nil {
		errs = append(errs, markErr)
	}
	return errors.Join(errs...)
}

const (
	usageCommitPendingReason            = "usage_commit_pending"
	usageReleasePendingReason           = "usage_release_pending"
	usageSettlementRecoveryScope        = "listingkit_usage_settlement"
	usageSettlementMaxAutoRetryAttempts = 8
)

func (s *service) markGenerationUsageReleasePending(ctx context.Context, task *Task, persistErr, releaseErr error) error {
	if task == nil {
		return releaseErr
	}
	now := time.Now().UTC()
	notBefore := now
	block := &RetryableBlock{
		ReasonCode:           usageReleasePendingReason,
		ReasonMessage:        "usage release is pending",
		BlockedAt:            now,
		NextRetryAt:          &notBefore,
		MaxAutoRetryAttempts: usageSettlementMaxAutoRetryAttempts,
		RecoveryScope:        usageSettlementRecoveryScope,
		AutoResumeEnabled:    true,
	}
	errorMsg := block.ReasonMessage
	if persistErr != nil {
		errorMsg = fmt.Sprintf("%s: %v", errorMsg, persistErr)
	}
	if err := markRetryableTaskState(ctx, s.repo, task.ID, block, errorMsg); err != nil {
		return errors.Join(releaseErr, err)
	}
	return nil
}

func generationQuotaFailure(taskID string) error {
	return fmt.Errorf("listingkit generation quota exceeded for task %s", strings.TrimSpace(taskID))
}

func generationUsageCommittedReplayResult(task *Task) (*ListingKitResult, error) {
	if task == nil || task.Result == nil {
		return nil, errors.New("generation usage is committed but task result is missing")
	}
	status := task.Result.Status
	if status != "completed" && status != "needs_review" {
		return nil, fmt.Errorf("generation usage is committed but task result is non-terminal: %s", status)
	}
	return task.Result, nil
}

func (s *service) markGenerationUsageCommitPending(ctx context.Context, task *Task, commitErr error) error {
	if task == nil {
		return commitErr
	}
	blockedAt := time.Now().UTC()
	nextRetryAt := blockedAt
	block := &RetryableBlock{
		ReasonCode:           usageCommitPendingReason,
		ReasonMessage:        "usage settlement is pending",
		BlockedAt:            blockedAt,
		NextRetryAt:          &nextRetryAt,
		MaxAutoRetryAttempts: usageSettlementMaxAutoRetryAttempts,
		RecoveryScope:        usageSettlementRecoveryScope,
		AutoResumeEnabled:    true,
	}
	if persistErr := markRetryableTaskState(ctx, s.repo, task.ID, block, block.ReasonMessage); persistErr != nil {
		return errors.Join(commitErr, persistErr)
	}
	return commitErr
}

func markRetryableTaskState(ctx context.Context, repo Repository, taskID string, block *RetryableBlock, errorMsg string) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		lastErr = repo.MarkBlockedRetryable(ctx, taskID, block, errorMsg)
		if lastErr == nil {
			return nil
		}
		if ctx.Err() != nil {
			return errors.Join(lastErr, ctx.Err())
		}
	}
	return lastErr
}

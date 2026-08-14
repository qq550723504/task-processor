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
	occurredAt := task.CreatedAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
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
	}
	if markErr := s.repo.MarkFailed(ctx, task.ID, "listing kit generation result persistence failed"); markErr != nil {
		errs = append(errs, markErr)
	}
	return errors.Join(errs...)
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
		ReasonCode:           "usage_commit_pending",
		ReasonMessage:        "usage settlement is pending",
		BlockedAt:            blockedAt,
		NextRetryAt:          &nextRetryAt,
		MaxAutoRetryAttempts: 8,
		RecoveryScope:        "listingkit_usage_settlement",
		AutoResumeEnabled:    true,
	}
	if persistErr := s.repo.MarkBlockedRetryable(ctx, task.ID, block, block.ReasonMessage); persistErr != nil {
		return errors.Join(commitErr, persistErr)
	}
	return commitErr
}

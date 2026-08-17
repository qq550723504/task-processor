package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
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
	if task.GenerationUsageReservationState == "" && !s.allowsNewGenerationUsageReservation(task) {
		return GenerationUsageReservation{}, false, nil
	}
	if task.GenerationUsageReservationState != "" && s.taskDeps.generationUsageAdmission != nil && strings.TrimSpace(task.BillingTenantID) == "" {
		return GenerationUsageReservation{}, true, errors.New("generation usage reservation is missing its billing tenant")
	}
	reservationRepo, ok := s.repo.(GenerationUsageReservationRepository)
	if !ok {
		return GenerationUsageReservation{}, true, errors.New("generation usage reservation repository is not configured")
	}
	leaseUntil := generationUsageReservationLeaseUntil()
	if err := reservationRepo.BeginGenerationUsageReservation(ctx, task.ID, leaseUntil); err != nil {
		return GenerationUsageReservation{}, true, err
	}
	task.GenerationUsageReservationState = GenerationUsageReservationStatePending
	task.GenerationUsageReservationLeaseUntil = &leaseUntil
	// A new reservation belongs to the period in which generation is actually
	// claimed. The ledger resolves an existing idempotency key first and keeps
	// its persisted period/occurrence for replays, so delayed tasks cannot be
	// charged to their creation month while retries remain idempotent.
	occurredAt := time.Now().UTC()
	reservation, err := settlement.ReserveGeneration(ctx, generationUsageTenantID(ctx, task), task.ID, occurredAt)
	if err != nil {
		return GenerationUsageReservation{}, true, err
	}
	if err := reservationRepo.MarkGenerationUsageReserved(ctx, task.ID, leaseUntil); err != nil {
		return GenerationUsageReservation{}, true, err
	}
	task.GenerationUsageReservationState = GenerationUsageReservationStateReserved
	task.GenerationUsageReservationLeaseUntil = &leaseUntil
	return reservation, true, nil
}

const (
	generationUsageReservationLeaseDuration   = 10 * time.Minute
	generationUsageReservationRenewalInterval = 3 * time.Minute
)

func generationUsageReservationLeaseUntil() time.Time {
	return time.Now().UTC().Add(generationUsageReservationLeaseDuration)
}

func (s *service) startGenerationUsageReservationLeaseRenewal(ctx context.Context, task *Task) (context.Context, func() error) {
	reservationRepo, ok := s.repo.(GenerationUsageReservationRepository)
	if !ok || task == nil || task.GenerationUsageReservationState == "" {
		return ctx, func() error { return nil }
	}
	return startGenerationUsageReservationLeaseRenewer(ctx, task, reservationRepo, generationUsageReservationRenewalInterval)
}

func startGenerationUsageReservationLeaseRenewer(ctx context.Context, task *Task, reservationRepo GenerationUsageReservationRepository, interval time.Duration) (context.Context, func() error) {
	if task == nil || reservationRepo == nil || interval <= 0 {
		return ctx, func() error { return nil }
	}
	workflowCtx, cancelWorkflow := context.WithCancel(ctx)
	stop := make(chan struct{})
	done := make(chan struct{})
	var stopOnce sync.Once
	var errMu sync.Mutex
	var renewalErr error

	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				persistCtx, persistCancel := settlementPersistenceContext(ctx)
				err := reservationRepo.RenewGenerationUsageReservation(persistCtx, task.ID, generationUsageReservationLeaseUntil())
				persistCancel()
				if err == nil {
					continue
				}
				errMu.Lock()
				renewalErr = fmt.Errorf("renew generation usage reservation lease: %w", err)
				errMu.Unlock()
				cancelWorkflow()
				return
			}
		}
	}()

	return workflowCtx, func() error {
		stopOnce.Do(func() {
			close(stop)
			cancelWorkflow()
			<-done
		})
		errMu.Lock()
		defer errMu.Unlock()
		return renewalErr
	}
}

func (s *service) allowsNewGenerationUsageReservation(task *Task) bool {
	if s == nil || task == nil {
		return false
	}
	admission := s.taskDeps.generationUsageAdmission
	if admission == nil {
		return true
	}
	billingTenantID := strings.TrimSpace(task.BillingTenantID)
	return billingTenantID != "" && admission.AllowsGenerationUsage(billingTenantID)
}

func (s *service) releaseGenerationUsage(ctx context.Context, task *Task, reason string) error {
	settlement := s.generationUsageSettlement()
	if settlement == nil || task == nil {
		return nil
	}
	if err := settlement.ReleaseGeneration(ctx, generationUsageTenantID(ctx, task), task.ID, strings.TrimSpace(reason)); err != nil {
		return err
	}
	return s.clearGenerationUsageReservation(ctx, task)
}

func (s *service) commitGenerationUsage(ctx context.Context, task *Task) error {
	settlement := s.generationUsageSettlement()
	if settlement == nil || task == nil {
		return nil
	}
	if err := settlement.CommitGeneration(ctx, generationUsageTenantID(ctx, task), task.ID); err != nil {
		return err
	}
	return s.clearGenerationUsageReservation(ctx, task)
}

func (s *service) clearGenerationUsageReservation(ctx context.Context, task *Task) error {
	if task == nil {
		return nil
	}
	reservationRepo, ok := s.repo.(GenerationUsageReservationRepository)
	if !ok {
		return errors.New("generation usage reservation repository is not configured")
	}
	if err := reservationRepo.ClearGenerationUsageReservation(ctx, task.ID); err != nil {
		return err
	}
	task.GenerationUsageReservationState = ""
	task.GenerationUsageReservationLeaseUntil = nil
	return nil
}

func generationUsageTenantID(ctx context.Context, task *Task) string {
	if task != nil {
		if billingTenantID := strings.TrimSpace(task.BillingTenantID); billingTenantID != "" {
			return billingTenantID
		}
		if tenantID := strings.TrimSpace(task.TenantID); tenantID != "" {
			return tenantID
		}
	}
	return TenantIDFromContext(ctx)
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
	if markErr := markFailedTaskState(ctx, s.repo, task.ID, "listing kit generation result persistence failed"); markErr != nil {
		errs = append(errs, markErr)
		if blockErr := markTerminalPersistencePending(ctx, s.repo, task.ID, markErr); blockErr != nil {
			errs = append(errs, blockErr)
		}
	}
	return errors.Join(errs...)
}

const (
	usageCommitPendingReason                = "usage_commit_pending"
	usageReleasePendingReason               = "usage_release_pending"
	usageSettlementRecoveryScope            = "listingkit_usage_settlement"
	usageSettlementMaxAutoRetryAttempts     = 8
	terminalPersistencePendingReason        = "terminal_persistence_pending"
	committedReplayPersistencePendingReason = "committed_replay_persistence_pending"
	settlementPersistenceTimeout            = 5 * time.Second
)

func settlementPersistenceContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), settlementPersistenceTimeout)
}

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
	persistCtx, cancel := settlementPersistenceContext(ctx)
	defer cancel()
	if err := markRetryableTaskState(persistCtx, s.repo, task.ID, block, errorMsg); err != nil {
		return errors.Join(releaseErr, err)
	}
	return nil
}

func markFailedTaskState(ctx context.Context, repo Repository, taskID, errorMsg string) error {
	persistCtx, cancel := settlementPersistenceContext(ctx)
	defer cancel()
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		lastErr = repo.MarkFailed(persistCtx, taskID, errorMsg)
		if lastErr == nil {
			return nil
		}
		if persistCtx.Err() != nil {
			break
		}
	}
	return lastErr
}

func markTerminalPersistencePending(ctx context.Context, repo Repository, taskID string, persistErr error) error {
	return markPersistencePending(ctx, repo, taskID, terminalPersistencePendingReason, "listing kit terminal state persistence is pending", persistErr)
}

func markCommittedReplayPersistencePending(ctx context.Context, repo Repository, taskID string, persistErr error) error {
	return markPersistencePending(ctx, repo, taskID, committedReplayPersistencePendingReason, "listing kit committed replay result persistence is pending", persistErr)
}

func markPersistencePending(ctx context.Context, repo Repository, taskID, reasonCode, reasonMessage string, persistErr error) error {
	now := time.Now().UTC()
	nextRetryAt := now
	block := &RetryableBlock{
		ReasonCode:           reasonCode,
		ReasonMessage:        reasonMessage,
		BlockedAt:            now,
		NextRetryAt:          &nextRetryAt,
		MaxAutoRetryAttempts: usageSettlementMaxAutoRetryAttempts,
		RecoveryScope:        usageSettlementRecoveryScope,
		AutoResumeEnabled:    true,
	}
	errorMsg := block.ReasonMessage
	if persistErr != nil {
		errorMsg = fmt.Sprintf("%s: %v", errorMsg, persistErr)
	}
	persistCtx, cancel := settlementPersistenceContext(ctx)
	defer cancel()
	return markRetryableTaskState(persistCtx, repo, taskID, block, errorMsg)
}

func (s *service) persistReservationFailure(ctx context.Context, task *Task, reserveErr error) error {
	return s.persistScheduledRetryableFailure(ctx, task, reserveErr, nil, false)
}

func (s *service) persistProcessRetryableFailure(ctx context.Context, task *Task, result *ListingKitResult, workflowErr error, reservationHeld bool) error {
	if task == nil {
		return workflowErr
	}
	persistCtx, cancel := settlementPersistenceContext(ctx)
	defer cancel()
	var persistErrs []error
	if result != nil {
		if err := s.repo.SaveTaskResult(persistCtx, task.ID, result); err != nil {
			persistErrs = append(persistErrs, fmt.Errorf("save partial result: %w", err))
		}
	}
	if err := s.persistScheduledRetryableFailure(persistCtx, task, workflowErr, errors.Join(persistErrs...), reservationHeld); err != nil {
		persistErrs = append(persistErrs, err)
	}
	return errors.Join(persistErrs...)
}

func (s *service) persistScheduledRetryableFailure(ctx context.Context, task *Task, failureErr, persistErr error, reservationHeld bool) error {
	if task == nil {
		return persistErr
	}
	block, ok := classifyRetryableTaskFailure(failureErr)
	if !ok {
		persistCtx, cancel := settlementPersistenceContext(ctx)
		defer cancel()
		if err := persistClassifiedTaskFailure(persistCtx, s.repo, task.ID, failureErr.Error(), failureErr); err != nil {
			return errors.Join(err, markTerminalPersistencePending(ctx, s.repo, task.ID, err))
		}
		return persistErr
	}
	now := time.Now().UTC()
	block.BlockedAt = now
	block.NextRetryAt = &now
	block.MaxAutoRetryAttempts = usageSettlementMaxAutoRetryAttempts
	block.AutoResumeEnabled = true
	errorMsg := block.ReasonMessage
	if persistErr != nil {
		errorMsg = fmt.Sprintf("%s: task failure persistence failed: %v", errorMsg, persistErr)
	}
	persistCtx, cancel := settlementPersistenceContext(ctx)
	defer cancel()
	if err := markRetryableTaskState(persistCtx, s.repo, task.ID, block, errorMsg); err != nil {
		if reservationHeld {
			recoveryErr := errors.Join(persistErr, err)
			return errors.Join(recoveryErr, s.markGenerationUsageReleasePending(ctx, task, failureErr, recoveryErr))
		}
		return errors.Join(persistErr, err, markTerminalPersistencePending(ctx, s.repo, task.ID, err))
	}
	return persistErr
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
	persistCtx, cancel := settlementPersistenceContext(ctx)
	defer cancel()
	if persistErr := markRetryableTaskState(persistCtx, s.repo, task.ID, block, block.ReasonMessage); persistErr != nil {
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

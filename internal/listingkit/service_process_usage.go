package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	submissiondomain "task-processor/internal/listing/submission"
	"task-processor/internal/listingkit/core"
)

type generationUsagePostReservePersistenceError struct {
	err error
}

// generationUsageReplayReservationError marks a failure after a task-side
// intent already existed. The deterministic ledger event may therefore be
// reserved even though the replay operation returned an error, so normal
// failure classification cannot safely finalize the task.
type generationUsageReplayReservationError struct {
	err error
}

func (e *generationUsageReplayReservationError) Error() string {
	if e == nil || e.err == nil {
		return "generation usage reservation replay failed"
	}
	return e.err.Error()
}

func (e *generationUsageReplayReservationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func (e *generationUsagePostReservePersistenceError) Error() string {
	if e == nil || e.err == nil {
		return "generation usage post-reserve persistence failed"
	}
	return e.err.Error()
}

func (e *generationUsagePostReservePersistenceError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

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
	hasReservationIntent := task.GenerationUsageReservationState != ""
	leaseUntil := generationUsageReservationLeaseUntil()
	if err := reservationRepo.BeginGenerationUsageReservation(ctx, task.ID, leaseUntil); err != nil {
		return GenerationUsageReservation{}, true, err
	}
	task.GenerationUsageReservationState = GenerationUsageReservationStatePending
	task.GenerationUsageReservationLeaseUntil = &leaseUntil
	// A new reservation belongs to the period in which generation is actually
	// claimed. An existing task-side intent deliberately supplies a zero time:
	// its deterministic event already owns the billing period, and the ledger
	// replays that event without accepting a new explicit period.
	occurredAt := time.Now().UTC()
	if hasReservationIntent {
		occurredAt = time.Time{}
	}
	reservation, err := settlement.ReserveGeneration(ctx, generationUsageTenantID(ctx, task), task.ID, occurredAt)
	if err != nil {
		if hasReservationIntent {
			return GenerationUsageReservation{}, true, &generationUsageReplayReservationError{err: err}
		}
		return GenerationUsageReservation{}, true, err
	}
	if err := reservationRepo.MarkGenerationUsageReserved(ctx, task.ID, leaseUntil); err != nil {
		return GenerationUsageReservation{}, true, &generationUsagePostReservePersistenceError{err: err}
	}
	task.GenerationUsageReservationState = GenerationUsageReservationStateReserved
	task.GenerationUsageReservationLeaseUntil = &leaseUntil
	return reservation, true, nil
}

func (s *service) persistGenerationUsageReconciliation(ctx context.Context, task *Task, cause error) error {
	if task == nil {
		return core.ErrTaskNotRecoverable
	}
	persistCtx, cancel := settlementPersistenceContext(ctx)
	defer cancel()
	return markRetryableTaskState(persistCtx, s.repo, task.ID, generationUsageReconciliationBlock(time.Now().UTC()), fmt.Sprintf("generation usage requires reconciliation after reserve: %v", cause))
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

func (s *service) releaseGenerationUsageAndFail(ctx context.Context, task *Task, result *ListingKitResult, reason, terminalError string) error {
	if task == nil {
		return core.ErrTaskNotFound
	}
	settlement := s.generationUsageSettlement()
	if settlement == nil {
		return errors.New("generation usage settlement is not configured")
	}
	recovery, ok := s.repo.(GenerationUsageReleaseRecoveryRepository)
	if !ok {
		return errors.New("generation usage release recovery repository is not configured")
	}
	block := newGenerationUsageReleasePendingBlock(reason, terminalError)
	persistCtx, cancel := settlementPersistenceContext(ctx)
	prepareErr := recovery.PrepareGenerationUsageRelease(persistCtx, task.ID, block, block.ReasonMessage, result)
	cancel()
	if prepareErr != nil {
		return prepareErr
	}
	if err := settlement.ReleaseGeneration(ctx, generationUsageTenantID(ctx, task), task.ID, block.UsageReleaseReason); err != nil {
		return s.reblockGenerationUsageRelease(ctx, task, block, err)
	}
	persistCtx, cancel = settlementPersistenceContext(ctx)
	resolveErr := recovery.ResolveGenerationUsageRelease(persistCtx, task.ID, block.TerminalError)
	cancel()
	if resolveErr != nil {
		return s.reblockGenerationUsageRelease(ctx, task, block, resolveErr)
	}
	task.Status = core.TaskStatusFailed
	task.RetryableBlock = nil
	task.Error = block.TerminalError
	task.GenerationUsageReservationState = ""
	task.GenerationUsageReservationLeaseUntil = nil
	return nil
}

func (s *service) reblockGenerationUsageRelease(ctx context.Context, task *Task, block *RetryableBlock, recoveryErr error) error {
	if task == nil || block == nil {
		return recoveryErr
	}
	classified, _ := submissiondomain.ClassifyRetryableFailure(recoveryErr, usageSettlementRecoveryScope)
	updated := adaptSubmissionRetryableBlock(submissiondomain.BuildReblockedRetryableBlock(
		adaptRetryableBlockState(block),
		classified,
		time.Now().UTC(),
		usageSettlementRecoveryScope,
	))
	updated.ReasonCode = block.ReasonCode
	updated.ReasonMessage = block.ReasonMessage
	updated.UsageReleaseReason = block.UsageReleaseReason
	updated.TerminalError = block.TerminalError
	updated.RecoveryScope = block.RecoveryScope
	updated.AutoResumeEnabled = block.AutoResumeEnabled
	persistCtx, cancel := settlementPersistenceContext(ctx)
	defer cancel()
	if err := markRetryableTaskState(persistCtx, s.repo, task.ID, updated, recoveryErr.Error()); err != nil {
		return errors.Join(recoveryErr, err)
	}
	return recoveryErr
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

func (s *service) handleGenerationTerminalPersistenceFailure(ctx context.Context, task *Task, result *ListingKitResult, persistErr error) error {
	if task == nil {
		return persistErr
	}
	terminalError := "listing kit generation result persistence failed"
	return errors.Join(persistErr, s.releaseGenerationUsageAndFail(ctx, task, result, "terminal_persistence_failed", terminalError))
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

func newGenerationUsageReleasePendingBlock(releaseReason, terminalError string) *RetryableBlock {
	now := time.Now().UTC()
	notBefore := now
	return &RetryableBlock{
		ReasonCode:           usageReleasePendingReason,
		ReasonMessage:        "usage release is pending",
		UsageReleaseReason:   strings.TrimSpace(releaseReason),
		TerminalError:        strings.TrimSpace(terminalError),
		BlockedAt:            now,
		NextRetryAt:          &notBefore,
		MaxAutoRetryAttempts: usageSettlementMaxAutoRetryAttempts,
		RecoveryScope:        usageSettlementRecoveryScope,
		AutoResumeEnabled:    true,
	}
}

func (s *service) markGenerationUsageReleasePending(ctx context.Context, task *Task, releaseReason string, terminalErr, releaseErr error) error {
	if task == nil {
		return releaseErr
	}
	terminalError := ""
	if terminalErr != nil {
		terminalError = terminalErr.Error()
	}
	block := newGenerationUsageReleasePendingBlock(releaseReason, terminalError)
	recovery, ok := s.repo.(GenerationUsageReleaseRecoveryRepository)
	if !ok {
		return errors.Join(releaseErr, errors.New("generation usage release recovery repository is not configured"))
	}
	persistCtx, cancel := settlementPersistenceContext(ctx)
	defer cancel()
	if err := recovery.PrepareGenerationUsageRelease(persistCtx, task.ID, block, block.ReasonMessage, nil); err != nil {
		return errors.Join(releaseErr, err)
	}
	return nil
}

func markFailedTaskState(ctx context.Context, repo Repository, taskID, errorMsg string) error {
	persistCtx, cancel := settlementPersistenceContext(ctx)
	defer cancel()
	var lastErr error
	admissionRepo, clearsAdmission := repo.(GenerationUsageAdmissionRepository)
	for attempt := 0; attempt < 3; attempt++ {
		if clearsAdmission {
			lastErr = admissionRepo.FinalizeGenerationUsageAdmission(persistCtx, taskID, core.TaskStatusFailed, nil, errorMsg)
		} else {
			lastErr = repo.MarkFailed(persistCtx, taskID, errorMsg)
		}
		if lastErr == nil {
			return nil
		}
		if persistCtx.Err() != nil {
			break
		}
	}
	return lastErr
}

func markTerminalPersistencePending(ctx context.Context, repo Repository, taskID, terminalError string, persistErr error) error {
	return markPersistencePending(ctx, repo, taskID, terminalPersistencePendingReason, "listing kit terminal state persistence is pending", terminalError, persistErr)
}

func markCommittedReplayPersistencePending(ctx context.Context, repo Repository, taskID string, persistErr error) error {
	return markPersistencePending(ctx, repo, taskID, committedReplayPersistencePendingReason, "listing kit committed replay result persistence is pending", "", persistErr)
}

func markPersistencePending(ctx context.Context, repo Repository, taskID, reasonCode, reasonMessage, terminalError string, persistErr error) error {
	now := time.Now().UTC()
	nextRetryAt := now
	block := &RetryableBlock{
		ReasonCode:           reasonCode,
		ReasonMessage:        reasonMessage,
		TerminalError:        strings.TrimSpace(terminalError),
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
	if reasonCode == terminalPersistencePendingReason {
		if admissionRepo, ok := repo.(GenerationUsageAdmissionRepository); ok {
			var lastErr error
			for attempt := 0; attempt < 3; attempt++ {
				lastErr = admissionRepo.FinalizeGenerationUsageAdmission(persistCtx, taskID, core.TaskStatusBlockedRetryable, block, errorMsg)
				if lastErr == nil || persistCtx.Err() != nil {
					return lastErr
				}
			}
			return lastErr
		}
	}
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
	classified, ok := classifyRetryableTaskFailure(failureErr)
	if !ok {
		persistCtx, cancel := settlementPersistenceContext(ctx)
		defer cancel()
		if err := persistClassifiedTaskFailure(persistCtx, s.repo, task.ID, failureErr.Error(), failureErr); err != nil {
			return errors.Join(err, markTerminalPersistencePending(ctx, s.repo, task.ID, failureErr.Error(), err))
		}
		return persistErr
	}
	now := time.Now().UTC()
	block := classified
	if task.RetryableBlock == nil {
		block.BlockedAt = now
		block.NextRetryAt = &now
		block.MaxAutoRetryAttempts = usageSettlementMaxAutoRetryAttempts
		block.AutoResumeEnabled = true
	} else {
		block = adaptSubmissionRetryableBlock(submissiondomain.BuildReblockedRetryableBlock(
			adaptRetryableBlockState(task.RetryableBlock),
			adaptRetryableBlockState(classified),
			now,
			submissiondomain.RetryableRecoveryScopeTask,
		))
		if block.MaxAutoRetryAttempts == 0 {
			block.MaxAutoRetryAttempts = usageSettlementMaxAutoRetryAttempts
		}
	}
	// A reservation remains held while retries are safe. At the retry cap we
	// cannot prove that the provider did not finish after returning an error, so
	// retain it under an operator-only reconciliation block instead of leaving a
	// paused automatic retry that permanently consumes quota without an owner.
	if reservationHeld && block.AutoRetryPaused {
		return errors.Join(persistErr, s.persistGenerationUsageReconciliation(ctx, task, failureErr))
	}
	errorMsg := block.ReasonMessage
	if persistErr != nil {
		errorMsg = fmt.Sprintf("%s: task failure persistence failed: %v", errorMsg, persistErr)
	}
	persistCtx, cancel := settlementPersistenceContext(ctx)
	defer cancel()
	if err := markRetryableTaskState(persistCtx, s.repo, task.ID, block, errorMsg); err != nil {
		if reservationHeld {
			recoveryErr := errors.Join(persistErr, err)
			return errors.Join(recoveryErr, s.markGenerationUsageReleasePending(ctx, task, "retryable_persistence_failed", failureErr, recoveryErr))
		}
		return errors.Join(persistErr, err, markTerminalPersistencePending(ctx, s.repo, task.ID, failureErr.Error(), err))
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

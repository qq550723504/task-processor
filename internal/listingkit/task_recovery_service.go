package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	submissiondomain "task-processor/internal/listing/submission"
	"task-processor/internal/listingkit/core"
)

type taskRecoveryServiceConfig struct {
	repo            Repository
	taskSubmitter   func() TaskSubmitter
	generationUsage GenerationUsageSettlement
	now             func() time.Time
}

type taskRecoveryService struct {
	repo            Repository
	taskSubmitter   func() TaskSubmitter
	generationUsage GenerationUsageSettlement
	now             func() time.Time
	recoveryNow     *submissiondomain.RecoveryNowService[Task]
	recoveryBatch   *submissiondomain.RecoveryBatchService[Task]
}

const (
	taskRecoveryBackfillPageSize               = 100
	taskRecoveryBackfillMaxAutoRetryAttempt    = 8
	generationUsageReconciliationPendingReason = "generation_usage_reconciliation_pending"
)

func newTaskRecoveryService(config taskRecoveryServiceConfig) *taskRecoveryService {
	nowFn := config.now
	if nowFn == nil {
		nowFn = func() time.Time { return time.Now().UTC() }
	}
	svc := &taskRecoveryService{
		repo:            config.repo,
		taskSubmitter:   config.taskSubmitter,
		generationUsage: config.generationUsage,
		now:             nowFn,
	}
	wiring := buildTaskRecoveryRunnerWiring(svc)
	svc.recoveryNow = submissiondomain.NewRecoveryNowService(submissiondomain.RecoveryNowServiceConfig[Task]{
		LoadTask:         wiring.loadTask,
		CurrentSubmitter: wiring.currentSubmitter,
		MarkRecovered:    wiring.markRecoveredNow,
		SubmitRecovered:  wiring.submitRecoveredNow,
		ReloadTask:       wiring.loadTask,
		ErrUnavailable:   core.ErrTaskRecoveryUnavailable,
		ErrEmptyTaskID:   core.ErrTaskNotFound,
	})
	svc.recoveryBatch = submissiondomain.NewRecoveryBatchService(submissiondomain.RecoveryBatchServiceConfig[Task]{
		ListCandidates:       wiring.listCandidates,
		CurrentSubmitter:     wiring.currentSubmitter,
		MarkRecovered:        wiring.markRecoveredBatch,
		SubmitRecovered:      wiring.submitRecoveredBatch,
		TaskID:               wiring.taskID,
		IsTaskNotRecoverable: wiring.isTaskNotRecoverable,
		Now:                  svc.currentTime,
		ErrUnavailable:       core.ErrTaskRecoveryUnavailable,
	})
	return svc
}

type taskRecoveryRunnerWiring struct {
	svc *taskRecoveryService
}

func buildTaskRecoveryRunnerWiring(svc *taskRecoveryService) taskRecoveryRunnerWiring {
	return taskRecoveryRunnerWiring{svc: svc}
}

func (w taskRecoveryRunnerWiring) loadTask(ctx context.Context, taskID string) (*Task, error) {
	return w.svc.repo.GetTask(ctx, taskID)
}

func (w taskRecoveryRunnerWiring) listCandidates(ctx context.Context, dueBefore time.Time, limit int) ([]Task, error) {
	// Classify in the repository before applying the provider limit. This keeps
	// both provider and settlement scans bounded without letting either class
	// consume the other's recovery page.
	tasks, err := w.svc.repo.ListRecoverableTasks(ctx, &RecoverableTaskQuery{
		DueBefore:          dueBefore,
		Limit:              limit,
		ExcludeReasonCodes: recoverySettlementReasonCodes(),
	})
	if err != nil {
		return nil, err
	}
	filtered := make([]Task, 0, len(tasks))
	for i := 0; i < len(tasks); i++ {
		if isUsageSettlementPending(&tasks[i]) || isPersistenceOnlyPending(&tasks[i]) {
			continue
		}
		filtered = append(filtered, tasks[i])
	}
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered, nil
}

func (w taskRecoveryRunnerWiring) currentSubmitter() submissiondomain.RecoverySubmitFunc {
	submitter := w.svc.currentSubmitter()
	if submitter == nil {
		return nil
	}
	return submitter.Submit
}

func (w taskRecoveryRunnerWiring) markRecoveredNow(ctx context.Context, taskID string) error {
	return w.svc.repo.RecoverBlockedTaskNow(ctx, taskID, time.Time{})
}

func (w taskRecoveryRunnerWiring) markRecoveredBatch(ctx context.Context, taskID string, recoverAt time.Time) error {
	task, err := w.svc.repo.GetTask(ctx, taskID)
	if err != nil {
		return err
	}
	if isUsageCommitPending(task) {
		return core.ErrTaskNotRecoverable
	}
	if isUsageReleasePending(task) {
		return core.ErrTaskNotRecoverable
	}
	if isPersistenceOnlyPending(task) {
		return core.ErrTaskNotRecoverable
	}
	return w.svc.repo.RecoverBlockedTaskNow(ctx, taskID, recoverAt)
}

func (w taskRecoveryRunnerWiring) submitRecoveredNow(ctx context.Context, submit submissiondomain.RecoverySubmitFunc, taskID string, current *Task) error {
	return w.svc.submitRecoveredTask(ctx, submit, current, w.svc.currentTime())
}

func (w taskRecoveryRunnerWiring) submitRecoveredBatch(ctx context.Context, submit submissiondomain.RecoverySubmitFunc, task Task, recoverAt time.Time) error {
	return w.svc.submitRecoveredTask(ctx, submit, &task, recoverAt)
}

func (w taskRecoveryRunnerWiring) taskID(task Task) string {
	return task.ID
}

func (w taskRecoveryRunnerWiring) isTaskNotRecoverable(err error) bool {
	return errors.Is(err, core.ErrTaskNotRecoverable)
}

func (s *taskRecoveryService) RecoverTaskNow(ctx context.Context, taskID string) (*Task, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("task recovery repository is not configured")
	}
	if s.recoveryNow == nil {
		return nil, fmt.Errorf("task recovery runner is not configured")
	}
	if task, err := s.repo.GetTask(ctx, taskID); err != nil {
		return nil, err
	} else if isGenerationUsageReconciliationPending(task) {
		return nil, core.ErrTaskNotRecoverable
	} else if isUsageCommitPending(task) {
		return s.recoverUsageCommit(ctx, task)
	} else if isUsageReleasePending(task) {
		return s.recoverUsageRelease(ctx, task)
	} else if isCommittedReplayPersistencePending(task) {
		return s.recoverCommittedReplayPersistence(ctx, task)
	} else if isTerminalPersistencePending(task) {
		return s.recoverTerminalPersistence(ctx, task)
	}
	return s.recoveryNow.RecoverNow(ctx, taskID)
}

func isUsageCommitPending(task *Task) bool {
	return task != nil && task.RetryableBlock != nil && task.RetryableBlock.ReasonCode == usageCommitPendingReason
}

func isUsageReleasePending(task *Task) bool {
	return task != nil && task.RetryableBlock != nil && task.RetryableBlock.ReasonCode == usageReleasePendingReason
}

func isUsageSettlementPending(task *Task) bool {
	return isUsageCommitPending(task) || isUsageReleasePending(task)
}

func isGenerationUsageReconciliationPending(task *Task) bool {
	return task != nil && task.RetryableBlock != nil && task.RetryableBlock.ReasonCode == generationUsageReconciliationPendingReason
}

func isTerminalPersistencePending(task *Task) bool {
	return task != nil && task.RetryableBlock != nil && task.RetryableBlock.ReasonCode == terminalPersistencePendingReason
}

func isCommittedReplayPersistencePending(task *Task) bool {
	return task != nil && task.RetryableBlock != nil && task.RetryableBlock.ReasonCode == committedReplayPersistencePendingReason
}

func isPersistenceOnlyPending(task *Task) bool {
	return isTerminalPersistencePending(task) || isCommittedReplayPersistencePending(task)
}

func (s *taskRecoveryService) recoverUsageCommit(ctx context.Context, task *Task) (*Task, error) {
	if s.generationUsage == nil {
		return nil, fmt.Errorf("generation usage settlement is not configured")
	}
	if err := s.generationUsage.CommitGeneration(ctx, generationUsageTenantID(ctx, task), task.ID); err != nil {
		return s.reblockOrAcceptResolvedUsageCommit(ctx, task, err)
	}
	settlementRepo, ok := s.repo.(UsageSettlementRepository)
	if !ok {
		return nil, fmt.Errorf("usage settlement repository is not configured")
	}
	if err := settlementRepo.ResolveUsageSettlement(ctx, task.ID); err != nil {
		if errors.Is(err, core.ErrTaskNotRecoverable) {
			if current, loadErr := s.repo.GetTask(ctx, task.ID); loadErr == nil && isResolvedUsageCommit(current) {
				return current, nil
			}
		}
		return s.reblockOrAcceptResolvedUsageCommit(ctx, task, err)
	}
	return s.repo.GetTask(ctx, task.ID)
}

func (s *taskRecoveryService) recoverUsageRelease(ctx context.Context, task *Task) (*Task, error) {
	if s.generationUsage == nil {
		return nil, fmt.Errorf("generation usage settlement is not configured")
	}
	if err := s.generationUsage.ReleaseGeneration(ctx, generationUsageTenantID(ctx, task), task.ID, usageReleaseRecoveryReason(task)); err != nil {
		return s.reblockOrAcceptResolvedUsageRelease(ctx, task, err)
	}
	recovery, ok := s.repo.(GenerationUsageReleaseRecoveryRepository)
	if !ok {
		return s.reblockOrAcceptResolvedUsageRelease(ctx, task, errors.New("generation usage release recovery repository is not configured"))
	}
	if err := recovery.ResolveGenerationUsageRelease(ctx, task.ID, terminalRecoveryError(task)); err != nil {
		if errors.Is(err, core.ErrTaskNotRecoverable) {
			if current, loadErr := s.repo.GetTask(ctx, task.ID); loadErr == nil && isResolvedUsageRelease(current) {
				return current, nil
			}
		}
		return s.reblockOrAcceptResolvedUsageRelease(ctx, task, err)
	}
	return s.repo.GetTask(ctx, task.ID)
}

func (s *taskRecoveryService) reblockOrAcceptResolvedUsageCommit(ctx context.Context, task *Task, recoveryErr error) (*Task, error) {
	if err := s.reblockUsageSettlement(ctx, task, recoveryErr); err != nil {
		if errors.Is(err, core.ErrTaskNotRecoverable) {
			if current, loadErr := s.repo.GetTask(ctx, task.ID); loadErr == nil && isResolvedUsageCommit(current) {
				return current, nil
			}
		}
		return nil, err
	}
	return nil, recoveryErr
}

func (s *taskRecoveryService) reblockOrAcceptResolvedUsageRelease(ctx context.Context, task *Task, recoveryErr error) (*Task, error) {
	if err := s.reblockUsageSettlement(ctx, task, recoveryErr); err != nil {
		if errors.Is(err, core.ErrTaskNotRecoverable) {
			if current, loadErr := s.repo.GetTask(ctx, task.ID); loadErr == nil && isResolvedUsageRelease(current) {
				return current, nil
			}
		}
		return nil, err
	}
	return nil, recoveryErr
}

func isResolvedUsageCommit(task *Task) bool {
	if task == nil || task.RetryableBlock != nil || task.Result == nil {
		return false
	}
	return (task.Status == core.TaskStatusCompleted || task.Status == core.TaskStatusNeedsReview) && task.Result.Status == string(task.Status)
}

func isResolvedUsageRelease(task *Task) bool {
	return task != nil && task.Status == core.TaskStatusFailed && task.RetryableBlock == nil && task.GenerationUsageReservationState == "" && task.GenerationUsageReservationLeaseUntil == nil
}

func usageReleaseRecoveryReason(task *Task) string {
	if task != nil && task.RetryableBlock != nil {
		if reason := strings.TrimSpace(task.RetryableBlock.UsageReleaseReason); reason != "" {
			return reason
		}
	}
	return "terminal_persistence_failed"
}

func (s *taskRecoveryService) recoverTerminalPersistence(ctx context.Context, task *Task) (*Task, error) {
	if task == nil {
		return nil, core.ErrTaskNotFound
	}
	if err := markFailedTaskState(ctx, s.repo, task.ID, terminalRecoveryError(task)); err != nil {
		return nil, s.reblockTerminalPersistence(ctx, task, err)
	}
	return s.repo.GetTask(ctx, task.ID)
}

func terminalRecoveryError(task *Task) string {
	if task != nil && task.RetryableBlock != nil {
		if terminalError := strings.TrimSpace(task.RetryableBlock.TerminalError); terminalError != "" {
			return terminalError
		}
	}
	if task == nil {
		return ""
	}
	return task.Error
}

func (s *taskRecoveryService) recoverCommittedReplayPersistence(ctx context.Context, task *Task) (*Task, error) {
	if task == nil {
		return nil, core.ErrTaskNotFound
	}
	if err := persistCommittedReplayResult(ctx, s.repo, task); err != nil {
		return nil, s.reblockTask(ctx, task, err, usageSettlementRecoveryScope)
	}
	return s.repo.GetTask(ctx, task.ID)
}

func persistCommittedReplayResult(ctx context.Context, repo Repository, task *Task) error {
	if repo == nil || task == nil || task.Result == nil {
		return core.ErrTaskNotRecoverable
	}
	persistCtx, cancel := settlementPersistenceContext(ctx)
	defer cancel()
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if task.Result.Status == string(core.TaskStatusNeedsReview) {
			lastErr = repo.MarkNeedsReview(persistCtx, task.ID, task.Result, taskNeedsReviewReason(task.Result))
		} else {
			lastErr = repo.MarkCompleted(persistCtx, task.ID, task.Result)
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

func (s *taskRecoveryService) reblockUsageSettlement(ctx context.Context, task *Task, settlementErr error) error {
	return s.reblockTask(ctx, task, settlementErr, usageSettlementRecoveryScope)
}

func (s *taskRecoveryService) reblockTerminalPersistence(ctx context.Context, task *Task, persistErr error) error {
	return s.reblockTask(ctx, task, persistErr, usageSettlementRecoveryScope)
}

func (s *taskRecoveryService) reblockTask(ctx context.Context, task *Task, recoveryErr error, defaultRecoveryScope string) error {
	if task == nil || task.RetryableBlock == nil {
		return recoveryErr
	}
	classified, _ := submissiondomain.ClassifyRetryableFailure(recoveryErr, defaultRecoveryScope)
	updated := adaptSubmissionRetryableBlock(submissiondomain.BuildReblockedRetryableBlock(
		adaptRetryableBlockState(task.RetryableBlock),
		classified,
		s.currentTime(),
		defaultRecoveryScope,
	))
	if updated.AutoRetryPaused && isUsageSettlementPending(task) {
		// A held settlement cannot be retried automatically once its bounded
		// attempts are exhausted. Preserve the intent under an operator-owned
		// reconciliation block instead of leaving a paused block with no sweep
		// owner while quota remains reserved.
		reconciliation := generationUsageReconciliationBlock(s.currentTime(), task.RetryableBlock)
		if applied, markErr := s.markRetryableBlockIfCurrent(ctx, task, reconciliation, generationUsageReconciliationError(task, recoveryErr)); markErr != nil {
			return errors.Join(recoveryErr, markErr)
		} else if !applied {
			return errors.Join(recoveryErr, core.ErrTaskNotRecoverable)
		}
		return recoveryErr
	}
	// Keep the settlement-only route even when the underlying ledger error is
	// classified as a generic upstream timeout/unavailable failure. Otherwise
	// the next sweep could send a terminal generation task back to the provider.
	updated.ReasonCode = task.RetryableBlock.ReasonCode
	updated.ReasonMessage = task.RetryableBlock.ReasonMessage
	updated.UsageReleaseReason = task.RetryableBlock.UsageReleaseReason
	updated.TerminalError = task.RetryableBlock.TerminalError
	updated.RecoveryScope = defaultRecoveryScope
	updated.AutoResumeEnabled = task.RetryableBlock.AutoResumeEnabled
	if applied, markErr := s.markRetryableBlockIfCurrent(ctx, task, updated, recoveryErr.Error()); markErr != nil {
		return errors.Join(recoveryErr, markErr)
	} else if !applied {
		return errors.Join(recoveryErr, core.ErrTaskNotRecoverable)
	}
	return recoveryErr
}

func (s *taskRecoveryService) markRetryableBlockIfCurrent(ctx context.Context, task *Task, next *RetryableBlock, errorMsg string) (bool, error) {
	if task == nil || task.RetryableBlock == nil {
		return false, core.ErrTaskNotRecoverable
	}
	repository, ok := s.repo.(ConditionalRetryableBlockRepository)
	if !ok {
		return false, errors.New("conditional retryable block repository is not configured")
	}
	persistCtx, cancel := settlementPersistenceContext(ctx)
	defer cancel()
	return repository.MarkBlockedRetryableIfCurrent(persistCtx, task.ID, task.RetryableBlock, next, errorMsg)
}

func (s *taskRecoveryService) RunRecoverySweep(ctx context.Context, now time.Time, limit int) (int64, error) {
	return s.BulkRecoverTasks(ctx, &RecoverBlockedTasksQuery{
		DueBefore: now,
		RecoverAt: now,
		Limit:     limit,
	})
}

func (s *taskRecoveryService) recoverExpiredGenerationUsageReservations(ctx context.Context, dueBefore time.Time, limit int) (int64, error) {
	if s == nil || s.repo == nil {
		return 0, nil
	}
	reservations, ok := s.repo.(GenerationUsageReservationRepository)
	if !ok {
		return 0, nil
	}
	items, err := reservations.ListExpiredGenerationUsageReservations(ctx, dueBefore, limit)
	if err != nil {
		return 0, err
	}
	var recovered int64
	var recoveryErr error
	for i := range items {
		task := &items[i]
		if task.Status == core.TaskStatusCompleted || task.Status == core.TaskStatusNeedsReview {
			if err := s.settleExpiredTerminalGenerationUsageReservation(ctx, task); err != nil {
				recoveryErr = errors.Join(recoveryErr, err)
				continue
			}
			recovered++
			continue
		}
		if err := reservations.ResolveExpiredGenerationUsageReservation(ctx, task.ID, task.Status, dueBefore, generationUsageReconciliationBlock(dueBefore, task.RetryableBlock), "generation usage lease expired and requires reconciliation", false); err != nil {
			if errors.Is(err, core.ErrTaskNotRecoverable) {
				continue
			}
			recoveryErr = errors.Join(recoveryErr, err)
			continue
		}
		recovered++
	}
	return recovered, recoveryErr
}

func (s *taskRecoveryService) settleExpiredTerminalGenerationUsageReservation(ctx context.Context, task *Task) error {
	if s == nil || s.generationUsage == nil || task == nil || task.Result == nil {
		return s.markGenerationUsageReconciliation(ctx, task, "terminal generation usage reservation cannot be settled")
	}
	if err := s.generationUsage.CommitGeneration(ctx, generationUsageTenantID(ctx, task), task.ID); err != nil {
		return s.markExpiredGenerationUsageCommitPending(ctx, task, err)
	}
	reservations, ok := s.repo.(GenerationUsageReservationRepository)
	if !ok {
		return s.markExpiredGenerationUsageCommitPending(ctx, task, errors.New("generation usage reservation repository is not configured"))
	}
	if err := reservations.ClearGenerationUsageReservation(ctx, task.ID); err != nil {
		return s.markExpiredGenerationUsageCommitPending(ctx, task, err)
	}
	return nil
}

func (s *taskRecoveryService) markExpiredGenerationUsageCommitPending(ctx context.Context, task *Task, commitErr error) error {
	if task == nil {
		return commitErr
	}
	now := s.currentTime()
	nextRetryAt := now
	block := &RetryableBlock{
		ReasonCode:           usageCommitPendingReason,
		ReasonMessage:        "usage settlement is pending",
		BlockedAt:            now,
		NextRetryAt:          &nextRetryAt,
		MaxAutoRetryAttempts: usageSettlementMaxAutoRetryAttempts,
		RecoveryScope:        usageSettlementRecoveryScope,
		AutoResumeEnabled:    true,
	}
	persistCtx, cancel := settlementPersistenceContext(ctx)
	defer cancel()
	if err := markRetryableTaskState(persistCtx, s.repo, task.ID, block, block.ReasonMessage); err != nil {
		return errors.Join(commitErr, err)
	}
	return commitErr
}

func (s *taskRecoveryService) markGenerationUsageReconciliation(ctx context.Context, task *Task, errorMsg string) error {
	if task == nil {
		return core.ErrTaskNotRecoverable
	}
	persistCtx, cancel := settlementPersistenceContext(ctx)
	defer cancel()
	return markRetryableTaskState(persistCtx, s.repo, task.ID, generationUsageReconciliationBlock(s.currentTime(), task.RetryableBlock), errorMsg)
}

func generationUsageReconciliationBlock(now time.Time, previous *RetryableBlock) *RetryableBlock {
	block := &RetryableBlock{
		ReasonCode:        generationUsageReconciliationPendingReason,
		ReasonMessage:     "generation usage requires reconciliation",
		BlockedAt:         now.UTC(),
		RecoveryScope:     usageSettlementRecoveryScope,
		AutoResumeEnabled: false,
	}
	if previous != nil {
		block.UsageReleaseReason = previous.UsageReleaseReason
		block.TerminalError = previous.TerminalError
	}
	return block
}

func generationUsageReconciliationError(task *Task, recoveryErr error) string {
	if task != nil && isUsageReleasePending(task) {
		if terminalError := terminalRecoveryError(task); terminalError != "" {
			return terminalError
		}
	}
	return fmt.Sprintf("generation usage settlement retry limit reached: %v", recoveryErr)
}

func recoverySettlementReasonCodes() []string {
	return []string{
		usageCommitPendingReason,
		usageReleasePendingReason,
		terminalPersistencePendingReason,
		committedReplayPersistencePendingReason,
	}
}

func (s *taskRecoveryService) BulkRecoverTasks(ctx context.Context, query *RecoverBlockedTasksQuery) (int64, error) {
	if s == nil || s.repo == nil {
		return 0, fmt.Errorf("task recovery repository is not configured")
	}
	if s.recoveryBatch == nil {
		return 0, fmt.Errorf("task recovery batch runner is not configured")
	}
	request := &submissiondomain.RecoveryBatchRequest{}
	if query != nil {
		request.DueBefore = query.DueBefore
		request.RecoverAt = query.RecoverAt
		request.Limit = query.Limit
	}
	dueBefore := request.DueBefore
	if dueBefore.IsZero() {
		dueBefore = s.currentTime()
	}
	settled, settleErr := s.recoverExpiredGenerationUsageReservations(ctx, dueBefore, request.Limit)
	if request.Limit > 0 && settled >= int64(request.Limit) {
		return settled, settleErr
	}
	remainingLimit := request.Limit
	if remainingLimit > 0 {
		remainingLimit -= int(settled)
	}
	// Filter settlement work in the repository before applying the remaining
	// limit, so a provider backlog cannot starve quota settlement and neither
	// scan deserializes an unbounded blocked-task backlog.
	candidates, err := s.repo.ListRecoverableTasks(ctx, &RecoverableTaskQuery{
		DueBefore:   dueBefore,
		Limit:       remainingLimit,
		ReasonCodes: recoverySettlementReasonCodes(),
	})
	if err != nil {
		return settled, errors.Join(settleErr, err)
	}
	settlementCandidates := make([]Task, 0, len(candidates))
	for i := 0; i < len(candidates); i++ {
		if isCommittedReplayPersistencePending(&candidates[i]) || isTerminalPersistencePending(&candidates[i]) || (s.generationUsage != nil && isUsageSettlementPending(&candidates[i])) {
			settlementCandidates = append(settlementCandidates, candidates[i])
		}
	}
	if remainingLimit > 0 && len(settlementCandidates) > remainingLimit {
		settlementCandidates = settlementCandidates[:remainingLimit]
	}
	for i := 0; i < len(settlementCandidates); i++ {
		if isCommittedReplayPersistencePending(&settlementCandidates[i]) {
			var err error
			_, err = s.recoverCommittedReplayPersistence(ctx, &settlementCandidates[i])
			if err != nil {
				settleErr = errors.Join(settleErr, err)
				continue
			}
			settled++
			continue
		}
		if isTerminalPersistencePending(&settlementCandidates[i]) {
			var err error
			_, err = s.recoverTerminalPersistence(ctx, &settlementCandidates[i])
			if err != nil {
				settleErr = errors.Join(settleErr, err)
				continue
			}
			settled++
			continue
		}
		if s.generationUsage == nil || !isUsageSettlementPending(&settlementCandidates[i]) {
			continue
		}
		var err error
		if isUsageReleasePending(&settlementCandidates[i]) {
			_, err = s.recoverUsageRelease(ctx, &settlementCandidates[i])
		} else {
			_, err = s.recoverUsageCommit(ctx, &settlementCandidates[i])
		}
		if err != nil {
			settleErr = errors.Join(settleErr, err)
			continue
		}
		settled++
	}
	if settled == 0 && settleErr == nil {
		return s.recoveryBatch.RecoverBatch(ctx, request)
	}
	remaining := request.Limit
	if remaining > 0 {
		remaining -= int(settled)
	}
	if remaining <= 0 && request.Limit > 0 {
		return settled, settleErr
	}
	batchRequest := request
	if request.Limit > 0 {
		copyRequest := *request
		copyRequest.Limit = remaining
		batchRequest = &copyRequest
	}
	recovered, err := s.recoveryBatch.RecoverBatch(ctx, batchRequest)
	return settled + recovered, errors.Join(settleErr, err)
}

func (s *taskRecoveryService) submitRecoveredTask(ctx context.Context, submit submissiondomain.RecoverySubmitFunc, task *Task, recoveredAt time.Time) error {
	if submit == nil {
		return core.ErrTaskRecoveryUnavailable
	}
	if task == nil {
		return core.ErrTaskNotFound
	}
	return submissiondomain.SubmitRecoveredWithRetryablePersistence(submissiondomain.RecoveredSubmitPersistenceRequest{
		TaskID:               task.ID,
		PreviousBlock:        adaptRetryableBlockState(task.RetryableBlock),
		RecoveredAt:          recoveredAt,
		DefaultRecoveryScope: submissiondomain.RetryableRecoveryScopeTask,
		Submit:               submit,
		MarkBlockedRetryable: func(block *submissiondomain.RetryableBlockState, errorMsg string) error {
			if block != nil && block.AutoRetryPaused && task.GenerationUsageReservationState != "" {
				return s.markGenerationUsageReconciliation(ctx, task, fmt.Sprintf("generation usage reservation retry limit reached: %s", errorMsg))
			}
			return markTaskBlockedRetryableState(ctx, s.repo, task.ID, block, errorMsg)
		},
		PersistFailure: func(errorMsg string, submitErr error) error {
			return persistClassifiedTaskFailure(ctx, s.repo, task.ID, errorMsg, submitErr)
		},
		RestoreDurability: func(errorMsg string, submitErr error, persistErr error) error {
			return s.restoreRecoveryDurability(ctx, task.ID, task.RetryableBlock, errorMsg, submitErr, persistErr)
		},
	})
}

func (s *taskRecoveryService) currentSubmitter() TaskSubmitter {
	if s == nil || s.taskSubmitter == nil {
		return nil
	}
	return s.taskSubmitter()
}

func (s *taskRecoveryService) currentTime() time.Time {
	if s == nil || s.now == nil {
		return time.Now().UTC()
	}
	return s.now().UTC()
}

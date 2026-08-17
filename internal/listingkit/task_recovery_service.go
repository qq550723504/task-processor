package listingkit

import (
	"context"
	"errors"
	"fmt"
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
	taskRecoveryBackfillPageSize            = 100
	taskRecoveryBackfillMaxAutoRetryAttempt = 8
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
	return w.svc.repo.ListRecoverableTasks(ctx, &RecoverableTaskQuery{
		DueBefore: dueBefore,
		Limit:     limit,
	})
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
	return w.svc.repo.RecoverBlockedTaskNow(ctx, taskID, recoverAt)
}

func (w taskRecoveryRunnerWiring) submitRecoveredNow(ctx context.Context, submit submissiondomain.RecoverySubmitFunc, taskID string, current *Task) error {
	return w.svc.submitRecoveredTask(ctx, submit, taskID, current.RetryableBlock, w.svc.currentTime())
}

func (w taskRecoveryRunnerWiring) submitRecoveredBatch(ctx context.Context, submit submissiondomain.RecoverySubmitFunc, task Task, recoverAt time.Time) error {
	return w.svc.submitRecoveredTask(ctx, submit, task.ID, task.RetryableBlock, recoverAt)
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
	} else if isUsageCommitPending(task) {
		return s.recoverUsageCommit(ctx, task)
	} else if isUsageReleasePending(task) {
		return s.recoverUsageRelease(ctx, task)
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

func (s *taskRecoveryService) recoverUsageCommit(ctx context.Context, task *Task) (*Task, error) {
	if s.generationUsage == nil {
		return nil, fmt.Errorf("generation usage settlement is not configured")
	}
	if err := s.generationUsage.CommitGeneration(ctx, task.TenantID, task.ID); err != nil {
		return nil, s.reblockUsageSettlement(ctx, task, err)
	}
	settlementRepo, ok := s.repo.(UsageSettlementRepository)
	if !ok {
		return nil, fmt.Errorf("usage settlement repository is not configured")
	}
	if err := settlementRepo.ResolveUsageSettlement(ctx, task.ID); err != nil {
		return nil, err
	}
	return s.repo.GetTask(ctx, task.ID)
}

func (s *taskRecoveryService) recoverUsageRelease(ctx context.Context, task *Task) (*Task, error) {
	if s.generationUsage == nil {
		return nil, fmt.Errorf("generation usage settlement is not configured")
	}
	if err := s.generationUsage.ReleaseGeneration(ctx, task.TenantID, task.ID, "terminal_persistence_failed"); err != nil {
		return nil, s.reblockUsageSettlement(ctx, task, err)
	}
	if err := s.repo.MarkFailed(ctx, task.ID, task.Error); err != nil {
		return nil, err
	}
	return s.repo.GetTask(ctx, task.ID)
}

func (s *taskRecoveryService) reblockUsageSettlement(ctx context.Context, task *Task, settlementErr error) error {
	if task == nil || task.RetryableBlock == nil {
		return settlementErr
	}
	classified, _ := submissiondomain.ClassifyRetryableFailure(settlementErr, usageSettlementRecoveryScope)
	updated := submissiondomain.BuildReblockedRetryableBlock(
		adaptRetryableBlockState(task.RetryableBlock),
		classified,
		s.currentTime(),
		usageSettlementRecoveryScope,
	)
	// Keep the settlement-only route even when the underlying ledger error is
	// classified as a generic upstream timeout/unavailable failure. Otherwise
	// the next sweep could send a terminal generation task back to the provider.
	updated.ReasonCode = task.RetryableBlock.ReasonCode
	updated.ReasonMessage = task.RetryableBlock.ReasonMessage
	updated.RecoveryScope = usageSettlementRecoveryScope
	updated.AutoResumeEnabled = task.RetryableBlock.AutoResumeEnabled
	if markErr := markTaskBlockedRetryableState(ctx, s.repo, task.ID, updated, settlementErr.Error()); markErr != nil {
		return errors.Join(settlementErr, markErr)
	}
	return settlementErr
}

func (s *taskRecoveryService) RunRecoverySweep(ctx context.Context, now time.Time, limit int) (int64, error) {
	return s.BulkRecoverTasks(ctx, &RecoverBlockedTasksQuery{
		DueBefore: now,
		RecoverAt: now,
		Limit:     limit,
	})
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
	var settled int64
	var settleErr error
	if s.generationUsage != nil {
		dueBefore := request.DueBefore
		if dueBefore.IsZero() {
			dueBefore = s.currentTime()
		}
		candidates, err := s.repo.ListRecoverableTasks(ctx, &RecoverableTaskQuery{DueBefore: dueBefore, Limit: request.Limit})
		if err != nil {
			return 0, err
		}
		for i := range candidates {
			if !isUsageSettlementPending(&candidates[i]) {
				continue
			}
			var err error
			if isUsageReleasePending(&candidates[i]) {
				_, err = s.recoverUsageRelease(ctx, &candidates[i])
			} else {
				_, err = s.recoverUsageCommit(ctx, &candidates[i])
			}
			if err != nil {
				settleErr = errors.Join(settleErr, err)
				continue
			}
			settled++
		}
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

func (s *taskRecoveryService) submitRecoveredTask(ctx context.Context, submit submissiondomain.RecoverySubmitFunc, taskID string, previousBlock *RetryableBlock, recoveredAt time.Time) error {
	if submit == nil {
		return core.ErrTaskRecoveryUnavailable
	}
	return submissiondomain.SubmitRecoveredWithRetryablePersistence(submissiondomain.RecoveredSubmitPersistenceRequest{
		TaskID:               taskID,
		PreviousBlock:        adaptRetryableBlockState(previousBlock),
		RecoveredAt:          recoveredAt,
		DefaultRecoveryScope: submissiondomain.RetryableRecoveryScopeTask,
		Submit:               submit,
		MarkBlockedRetryable: func(block *submissiondomain.RetryableBlockState, errorMsg string) error {
			return markTaskBlockedRetryableState(ctx, s.repo, taskID, block, errorMsg)
		},
		PersistFailure: func(errorMsg string, submitErr error) error {
			return persistClassifiedTaskFailure(ctx, s.repo, taskID, errorMsg, submitErr)
		},
		RestoreDurability: func(errorMsg string, submitErr error, persistErr error) error {
			return s.restoreRecoveryDurability(ctx, taskID, previousBlock, errorMsg, submitErr, persistErr)
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

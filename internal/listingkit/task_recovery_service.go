package listingkit

import (
	"context"
	"errors"
	"fmt"
	"time"

	submissiondomain "task-processor/internal/listing/submission"
)

type taskRecoveryServiceConfig struct {
	repo          Repository
	taskSubmitter func() TaskSubmitter
	now           func() time.Time
}

type taskRecoveryService struct {
	repo          Repository
	taskSubmitter func() TaskSubmitter
	now           func() time.Time
	recoveryNow   *submissiondomain.RecoveryNowService[Task]
	recoveryBatch *submissiondomain.RecoveryBatchService[Task]
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
		repo:          config.repo,
		taskSubmitter: config.taskSubmitter,
		now:           nowFn,
	}
	wiring := buildTaskRecoveryRunnerWiring(svc)
	svc.recoveryNow = submissiondomain.NewRecoveryNowService(submissiondomain.RecoveryNowServiceConfig[Task]{
		LoadTask:         wiring.loadTask,
		CurrentSubmitter: wiring.currentSubmitter,
		MarkRecovered:    wiring.markRecoveredNow,
		SubmitRecovered:  wiring.submitRecoveredNow,
		ReloadTask:       wiring.loadTask,
		ErrUnavailable:   ErrTaskRecoveryUnavailable,
		ErrEmptyTaskID:   ErrTaskNotFound,
	})
	svc.recoveryBatch = submissiondomain.NewRecoveryBatchService(submissiondomain.RecoveryBatchServiceConfig[Task]{
		ListCandidates:       wiring.listCandidates,
		CurrentSubmitter:     wiring.currentSubmitter,
		MarkRecovered:        wiring.markRecoveredBatch,
		SubmitRecovered:      wiring.submitRecoveredBatch,
		TaskID:               wiring.taskID,
		IsTaskNotRecoverable: wiring.isTaskNotRecoverable,
		Now:                  svc.currentTime,
		ErrUnavailable:       ErrTaskRecoveryUnavailable,
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
	return w.svc.repo.RecoverBlockedTaskNow(ctx, taskID, recoverAt)
}

func (w taskRecoveryRunnerWiring) submitRecoveredNow(ctx context.Context, submit submissiondomain.RecoverySubmitFunc, taskID string, current *Task) error {
	return w.svc.submitRecoveredTask(ctx, taskRecoverySubmitterFunc(submit), taskID, current.RetryableBlock, w.svc.currentTime())
}

func (w taskRecoveryRunnerWiring) submitRecoveredBatch(ctx context.Context, submit submissiondomain.RecoverySubmitFunc, task Task, recoverAt time.Time) error {
	return w.svc.submitRecoveredTask(ctx, taskRecoverySubmitterFunc(submit), task.ID, task.RetryableBlock, recoverAt)
}

func (w taskRecoveryRunnerWiring) taskID(task Task) string {
	return task.ID
}

func (w taskRecoveryRunnerWiring) isTaskNotRecoverable(err error) bool {
	return errors.Is(err, ErrTaskNotRecoverable)
}

func (s *taskRecoveryService) RecoverTaskNow(ctx context.Context, taskID string) (*Task, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("task recovery repository is not configured")
	}
	if s.recoveryNow == nil {
		return nil, fmt.Errorf("task recovery runner is not configured")
	}
	return s.recoveryNow.RecoverNow(ctx, taskID)
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
	return s.recoveryBatch.RecoverBatch(ctx, request)
}

func (s *taskRecoveryService) submitRecoveredTask(ctx context.Context, submitter TaskSubmitter, taskID string, previousBlock *RetryableBlock, recoveredAt time.Time) error {
	if submitter == nil {
		return ErrTaskRecoveryUnavailable
	}
	return submissiondomain.SubmitRecoveredWithRetryablePersistence(submissiondomain.RecoveredSubmitPersistenceRequest{
		TaskID:               taskID,
		PreviousBlock:        adaptRetryableBlockState(previousBlock),
		RecoveredAt:          recoveredAt,
		DefaultRecoveryScope: submissiondomain.RetryableRecoveryScopeTask,
		Submit:               submitter.Submit,
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

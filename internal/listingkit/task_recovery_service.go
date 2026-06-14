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
	svc.recoveryNow = submissiondomain.NewRecoveryNowService(submissiondomain.RecoveryNowServiceConfig[Task]{
		LoadTask: func(ctx context.Context, taskID string) (*Task, error) {
			return svc.repo.GetTask(ctx, taskID)
		},
		CurrentSubmitter: func() submissiondomain.RecoverySubmitFunc {
			submitter := svc.currentSubmitter()
			if submitter == nil {
				return nil
			}
			return submitter.Submit
		},
		MarkRecovered: func(ctx context.Context, taskID string) error {
			return svc.repo.RecoverBlockedTaskNow(ctx, taskID, time.Time{})
		},
		SubmitRecovered: func(ctx context.Context, submit submissiondomain.RecoverySubmitFunc, taskID string, current *Task) error {
			return svc.submitRecoveredTask(ctx, taskRecoverySubmitterFunc(submit), taskID, current.RetryableBlock, svc.currentTime())
		},
		ReloadTask: func(ctx context.Context, taskID string) (*Task, error) {
			return svc.repo.GetTask(ctx, taskID)
		},
		ErrUnavailable: ErrTaskRecoveryUnavailable,
		ErrEmptyTaskID: ErrTaskNotFound,
	})
	svc.recoveryBatch = submissiondomain.NewRecoveryBatchService(submissiondomain.RecoveryBatchServiceConfig[Task]{
		ListCandidates: func(ctx context.Context, dueBefore time.Time, limit int) ([]Task, error) {
			return svc.repo.ListRecoverableTasks(ctx, &RecoverableTaskQuery{
				DueBefore: dueBefore,
				Limit:     limit,
			})
		},
		CurrentSubmitter: func() submissiondomain.RecoverySubmitFunc {
			submitter := svc.currentSubmitter()
			if submitter == nil {
				return nil
			}
			return submitter.Submit
		},
		MarkRecovered: func(ctx context.Context, taskID string, recoverAt time.Time) error {
			return svc.repo.RecoverBlockedTaskNow(ctx, taskID, recoverAt)
		},
		SubmitRecovered: func(ctx context.Context, submit submissiondomain.RecoverySubmitFunc, task Task, recoverAt time.Time) error {
			return svc.submitRecoveredTask(ctx, taskRecoverySubmitterFunc(submit), task.ID, task.RetryableBlock, recoverAt)
		},
		TaskID: func(task Task) string { return task.ID },
		IsTaskNotRecoverable: func(err error) bool {
			return errors.Is(err, ErrTaskNotRecoverable)
		},
		Now:            svc.currentTime,
		ErrUnavailable: ErrTaskRecoveryUnavailable,
	})
	return svc
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
	if err := submitter.Submit(taskID); err != nil {
		if block, ok := classifyRetryableTaskFailure(err); ok {
			updated := s.buildReblockedTask(previousBlock, block, recoveredAt)
			errorMsg := fmt.Sprintf("failed to submit task: %v", err)
			if markErr := s.repo.MarkBlockedRetryable(ctx, taskID, updated, errorMsg); markErr != nil {
				return s.restoreRecoveryDurability(ctx, taskID, previousBlock, errorMsg, err, fmt.Errorf("mark blocked retryable: %w", markErr))
			}
			return fmt.Errorf("submit recovered task %s: %w", taskID, err)
		}
		if persistErr := persistClassifiedTaskFailure(ctx, s.repo, taskID, fmt.Sprintf("failed to submit task: %v", err), err); persistErr != nil {
			return s.restoreRecoveryDurability(ctx, taskID, previousBlock, fmt.Sprintf("failed to submit task: %v", err), err, persistErr)
		}
		return fmt.Errorf("submit recovered task %s: %w", taskID, err)
	}
	return nil
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

package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
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
	return &taskRecoveryService{
		repo:          config.repo,
		taskSubmitter: config.taskSubmitter,
		now:           nowFn,
	}
}

func (s *taskRecoveryService) RecoverTaskNow(ctx context.Context, taskID string) (*Task, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("task recovery repository is not configured")
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, ErrTaskNotFound
	}
	submitter := s.currentSubmitter()
	if submitter == nil {
		return nil, ErrTaskRecoveryUnavailable
	}
	current, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	recoveredAt := s.currentTime()
	if err := s.repo.RecoverBlockedTaskNow(ctx, taskID, time.Time{}); err != nil {
		return nil, err
	}
	if err := s.submitRecoveredTask(ctx, submitter, taskID, current.RetryableBlock, recoveredAt); err != nil {
		return nil, err
	}
	return s.repo.GetTask(ctx, taskID)
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
	submitter := s.currentSubmitter()
	if submitter == nil {
		return 0, ErrTaskRecoveryUnavailable
	}

	recoverAt := s.currentTime()
	listQuery := &RecoverableTaskQuery{}
	if query != nil {
		listQuery.DueBefore = query.DueBefore
		listQuery.Limit = query.Limit
		if !query.RecoverAt.IsZero() {
			recoverAt = query.RecoverAt
		}
	}
	if listQuery.DueBefore.IsZero() {
		listQuery.DueBefore = recoverAt
	}

	tasks, err := s.repo.ListRecoverableTasks(ctx, listQuery)
	if err != nil {
		return 0, err
	}

	var recovered int64
	var runErr error
	for i := range tasks {
		task := tasks[i]
		if err := s.repo.RecoverBlockedTaskNow(ctx, task.ID, recoverAt); err != nil {
			if errors.Is(err, ErrTaskNotRecoverable) {
				continue
			}
			return recovered, err
		}
		if err := s.submitRecoveredTask(ctx, submitter, task.ID, task.RetryableBlock, recoverAt); err != nil {
			runErr = errors.Join(runErr, err)
			continue
		}
		recovered++
	}
	return recovered, runErr
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

func (s *service) RecoverTaskNow(ctx context.Context, taskID string) (*Task, error) {
	return s.taskRecoveryOrDefault().RecoverTaskNow(ctx, taskID)
}

func (s *service) RunRecoverySweep(ctx context.Context, now time.Time, limit int) (int64, error) {
	return s.taskRecoveryOrDefault().RunRecoverySweep(ctx, now, limit)
}

func (s *service) BulkRecoverTasks(ctx context.Context, query *RecoverBlockedTasksQuery) (int64, error) {
	return s.taskRecoveryOrDefault().BulkRecoverTasks(ctx, query)
}

func (s *service) taskRecoveryOrDefault() *taskRecoveryService {
	if s == nil {
		return nil
	}
	if s.submission.taskRecovery != nil {
		return s.submission.taskRecovery
	}
	s.submission.taskRecovery = newTaskRecoveryService(buildTaskRecoveryServiceConfig(s))
	return s.submission.taskRecovery
}

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
	if attempt <= 1 {
		return listingKitAsyncEnqueueRetryDelay
	}
	delay := listingKitAsyncEnqueueRetryDelay
	for i := 1; i < attempt; i++ {
		delay *= 2
		if delay >= listingKitAsyncEnqueueRetryMaxDelay {
			return listingKitAsyncEnqueueRetryMaxDelay
		}
	}
	return delay
}

func cloneTimePointer(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copied := value
	return &copied
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

func backfillRetryableBlockedTasks(ctx context.Context, repo Repository, createdAfter time.Time) (int, error) {
	if repo == nil {
		return 0, fmt.Errorf("task recovery repository is not configured")
	}

	tasks, _, err := repo.ListTasks(ctx, &TaskListQuery{
		Status:   string(TaskStatusFailed),
		Page:     1,
		PageSize: taskRecoveryBackfillPageSize,
	})
	if err != nil {
		return 0, err
	}

	backfilledAt := time.Now().UTC()
	count := 0
	for i := range tasks {
		task := tasks[i]
		if !createdAfter.IsZero() && task.CreatedAt.Before(createdAfter) {
			continue
		}
		if strings.TrimSpace(task.Error) == "" {
			continue
		}
		block, ok := classifyRetryableTaskFailure(errors.New(task.Error))
		if !ok {
			continue
		}
		block.BlockedAt = task.UpdatedAt.UTC()
		if block.BlockedAt.IsZero() {
			block.BlockedAt = backfilledAt
		}
		block.RetryAttempts = 0
		block.LastRetryAt = nil
		block.AutoRetryPaused = false
		block.MaxAutoRetryAttempts = taskRecoveryBackfillMaxAutoRetryAttempt
		nextRetryAt := backfilledAt.Add(boundedRecoveryRetryDelay(1))
		block.NextRetryAt = cloneTimePointer(nextRetryAt)
		if err := repo.MarkBlockedRetryable(ctx, task.ID, block, task.Error); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
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
	return newTaskRecoveryService(taskRecoveryServiceConfig{
		repo: s.repo,
		taskSubmitter: func() TaskSubmitter {
			return s.taskSubmitter
		},
	})
}

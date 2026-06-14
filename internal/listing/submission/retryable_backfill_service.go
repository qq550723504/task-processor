package submission

import (
	"context"
	"strings"
	"time"
)

type RetryableBackfillRecord struct {
	ID         string
	Error      string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type RetryableBackfillRequest struct {
	CreatedAfter time.Time
}

type RetryableBackfillService[T any] struct {
	listFailedTasks       func(context.Context) ([]T, error)
	record                func(T) RetryableBackfillRecord
	markBlockedRetryable  func(context.Context, T, *RetryableBlockState, string) error
	now                   func() time.Time
	maxAutoRetryAttempts  int
	defaultRecoveryScope  string
}

type RetryableBackfillServiceConfig[T any] struct {
	ListFailedTasks      func(context.Context) ([]T, error)
	Record               func(T) RetryableBackfillRecord
	MarkBlockedRetryable func(context.Context, T, *RetryableBlockState, string) error
	Now                  func() time.Time
	MaxAutoRetryAttempts int
	DefaultRecoveryScope string
}

func NewRetryableBackfillService[T any](config RetryableBackfillServiceConfig[T]) *RetryableBackfillService[T] {
	return &RetryableBackfillService[T]{
		listFailedTasks:      config.ListFailedTasks,
		record:               config.Record,
		markBlockedRetryable: config.MarkBlockedRetryable,
		now:                  config.Now,
		maxAutoRetryAttempts: config.MaxAutoRetryAttempts,
		defaultRecoveryScope: config.DefaultRecoveryScope,
	}
}

func (s *RetryableBackfillService[T]) Backfill(ctx context.Context, request RetryableBackfillRequest) (int, error) {
	if s == nil || s.listFailedTasks == nil || s.record == nil || s.markBlockedRetryable == nil {
		return 0, nil
	}

	tasks, err := s.listFailedTasks(ctx)
	if err != nil {
		return 0, err
	}

	backfilledAt := time.Now().UTC()
	if s.now != nil {
		backfilledAt = s.now().UTC()
	}

	count := 0
	for _, task := range tasks {
		record := s.record(task)
		if !request.CreatedAfter.IsZero() && record.CreatedAt.Before(request.CreatedAfter) {
			continue
		}
		if strings.TrimSpace(record.Error) == "" {
			continue
		}
		block, ok := BuildBackfilledRetryableBlock(
			contextlessError(record.Error),
			record.UpdatedAt,
			backfilledAt,
			s.maxAutoRetryAttempts,
			s.defaultRecoveryScope,
		)
		if !ok {
			continue
		}
		if err := s.markBlockedRetryable(ctx, task, block, record.Error); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

type contextlessError string

func (e contextlessError) Error() string { return string(e) }

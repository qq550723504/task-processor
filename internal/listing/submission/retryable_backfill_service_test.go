package submission

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetryableBackfillServiceBackfill(t *testing.T) {
	t.Parallel()

	type taskRecord struct {
		id        string
		err       string
		createdAt time.Time
		updatedAt time.Time
	}

	now := time.Date(2026, 6, 6, 18, 0, 0, 0, time.UTC)
	tasks := []taskRecord{
		{
			id:        "retryable",
			err:       "openai request failed: insufficient credits",
			createdAt: now.Add(-time.Hour),
			updatedAt: now.Add(-30 * time.Minute),
		},
		{
			id:        "permanent",
			err:       "validation failed",
			createdAt: now.Add(-20 * time.Minute),
			updatedAt: now.Add(-10 * time.Minute),
		},
	}

	marked := make(map[string]*RetryableBlockState)
	service := NewRetryableBackfillService(RetryableBackfillServiceConfig[taskRecord]{
		ListFailedTasks: func(context.Context) ([]taskRecord, error) {
			return tasks, nil
		},
		Record: func(task taskRecord) RetryableBackfillRecord {
			return RetryableBackfillRecord{
				ID:        task.id,
				Error:     task.err,
				CreatedAt: task.createdAt,
				UpdatedAt: task.updatedAt,
			}
		},
		MarkBlockedRetryable: func(_ context.Context, task taskRecord, block *RetryableBlockState, _ string) error {
			marked[task.id] = CloneRetryableBlockState(block)
			return nil
		},
		Now:                  func() time.Time { return now },
		MaxAutoRetryAttempts: 8,
		DefaultRecoveryScope: "task",
	})

	count, err := service.Backfill(context.Background(), RetryableBackfillRequest{
		CreatedAfter: now.Add(-24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("Backfill() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("Backfill() count = %d, want 1", count)
	}
	if _, ok := marked["retryable"]; !ok {
		t.Fatal("retryable task was not marked blocked retryable")
	}
	if _, ok := marked["permanent"]; ok {
		t.Fatal("permanent task was marked blocked retryable")
	}
}

func TestRetryableBackfillServiceReturnsMarkErrorAfterPartialProgress(t *testing.T) {
	t.Parallel()

	type taskRecord struct {
		id        string
		err       string
		createdAt time.Time
		updatedAt time.Time
	}

	now := time.Date(2026, 6, 6, 18, 0, 0, 0, time.UTC)
	tasks := []taskRecord{
		{id: "one", err: "queue full", createdAt: now, updatedAt: now},
		{id: "two", err: "queue full", createdAt: now, updatedAt: now},
	}

	callCount := 0
	service := NewRetryableBackfillService(RetryableBackfillServiceConfig[taskRecord]{
		ListFailedTasks: func(context.Context) ([]taskRecord, error) {
			return tasks, nil
		},
		Record: func(task taskRecord) RetryableBackfillRecord {
			return RetryableBackfillRecord{
				ID:        task.id,
				Error:     task.err,
				CreatedAt: task.createdAt,
				UpdatedAt: task.updatedAt,
			}
		},
		MarkBlockedRetryable: func(_ context.Context, task taskRecord, _ *RetryableBlockState, _ string) error {
			callCount++
			if task.id == "two" {
				return errors.New("persist failed")
			}
			return nil
		},
		Now:                  func() time.Time { return now },
		MaxAutoRetryAttempts: 8,
		DefaultRecoveryScope: "task",
	})

	count, err := service.Backfill(context.Background(), RetryableBackfillRequest{})
	if err == nil {
		t.Fatal("Backfill() error = nil, want persist failure")
	}
	if count != 1 {
		t.Fatalf("Backfill() count = %d, want 1 before failure", count)
	}
	if callCount != 2 {
		t.Fatalf("MarkBlockedRetryable calls = %d, want 2", callCount)
	}
}

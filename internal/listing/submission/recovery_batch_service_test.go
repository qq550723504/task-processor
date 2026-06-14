package submission

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRecoveryBatchServiceRecoverBatch(t *testing.T) {
	t.Parallel()

	type task struct {
		ID string
	}

	errUnavailable := errors.New("submitter unavailable")
	errNotRecoverable := errors.New("not recoverable")
	now := time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC)

	t.Run("recovers due tasks and aggregates submit errors", func(t *testing.T) {
		t.Parallel()

		var seenDueBefore time.Time
		var seenLimit int
		var marked []string
		service := NewRecoveryBatchService(RecoveryBatchServiceConfig[task]{
			ListCandidates: func(_ context.Context, dueBefore time.Time, limit int) ([]task, error) {
				seenDueBefore = dueBefore
				seenLimit = limit
				return []task{{ID: "task-1"}, {ID: "task-2"}}, nil
			},
			CurrentSubmitter: func() RecoverySubmitFunc {
				return func(taskID string) error {
					if taskID == "task-2" {
						return errors.New("queue full")
					}
					return nil
				}
			},
			MarkRecovered: func(_ context.Context, taskID string, recoverAt time.Time) error {
				if !recoverAt.Equal(now) {
					t.Fatalf("recoverAt = %v, want %v", recoverAt, now)
				}
				marked = append(marked, taskID)
				return nil
			},
			SubmitRecovered: func(_ context.Context, submit RecoverySubmitFunc, current task, _ time.Time) error {
				return submit(current.ID)
			},
			TaskID:         func(current task) string { return current.ID },
			Now:            func() time.Time { return now },
			ErrUnavailable: errUnavailable,
		})

		recovered, err := service.RecoverBatch(context.Background(), &RecoveryBatchRequest{Limit: 20})
		if recovered != 1 {
			t.Fatalf("recovered = %d, want 1", recovered)
		}
		if err == nil || err.Error() == "" {
			t.Fatal("expected aggregated submit error")
		}
		if !seenDueBefore.Equal(now) {
			t.Fatalf("dueBefore = %v, want %v", seenDueBefore, now)
		}
		if seenLimit != 20 {
			t.Fatalf("limit = %d, want 20", seenLimit)
		}
		if len(marked) != 2 {
			t.Fatalf("marked = %#v", marked)
		}
	})

	t.Run("skips not recoverable mark failures", func(t *testing.T) {
		t.Parallel()

		service := NewRecoveryBatchService(RecoveryBatchServiceConfig[task]{
			ListCandidates: func(_ context.Context, _ time.Time, _ int) ([]task, error) {
				return []task{{ID: "task-1"}}, nil
			},
			CurrentSubmitter: func() RecoverySubmitFunc {
				return func(string) error { return nil }
			},
			MarkRecovered: func(_ context.Context, _ string, _ time.Time) error {
				return errNotRecoverable
			},
			SubmitRecovered: func(_ context.Context, _ RecoverySubmitFunc, _ task, _ time.Time) error {
				t.Fatal("submit should not be called for skipped task")
				return nil
			},
			TaskID: func(current task) string { return current.ID },
			IsTaskNotRecoverable: func(err error) bool {
				return errors.Is(err, errNotRecoverable)
			},
			Now:            func() time.Time { return now },
			ErrUnavailable: errUnavailable,
		})

		recovered, err := service.RecoverBatch(context.Background(), &RecoveryBatchRequest{})
		if err != nil {
			t.Fatalf("RecoverBatch() error = %v", err)
		}
		if recovered != 0 {
			t.Fatalf("recovered = %d, want 0", recovered)
		}
	})

	t.Run("rejects unavailable submitter", func(t *testing.T) {
		t.Parallel()

		service := NewRecoveryBatchService(RecoveryBatchServiceConfig[task]{
			ListCandidates: func(_ context.Context, _ time.Time, _ int) ([]task, error) {
				return nil, nil
			},
			Now:            func() time.Time { return now },
			ErrUnavailable: errUnavailable,
		})

		if _, err := service.RecoverBatch(context.Background(), &RecoveryBatchRequest{}); !errors.Is(err, errUnavailable) {
			t.Fatalf("RecoverBatch() error = %v, want %v", err, errUnavailable)
		}
	})
}

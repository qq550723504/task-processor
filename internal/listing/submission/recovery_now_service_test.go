package submission

import (
	"context"
	"errors"
	"testing"
)

func TestRecoveryNowServiceRecoverNow(t *testing.T) {
	t.Parallel()

	type task struct{ ID string }

	errUnavailable := errors.New("submitter unavailable")
	errEmpty := errors.New("task not found")

	t.Run("loads recovers submits and reloads", func(t *testing.T) {
		t.Parallel()

		var submitted []string
		var recovered []string
		service := NewRecoveryNowService(RecoveryNowServiceConfig[task]{
			LoadTask: func(_ context.Context, taskID string) (*task, error) {
				return &task{ID: taskID}, nil
			},
			CurrentSubmitter: func() RecoverySubmitFunc {
				return func(taskID string) error {
					submitted = append(submitted, taskID)
					return nil
				}
			},
			MarkRecovered: func(_ context.Context, taskID string) error {
				recovered = append(recovered, taskID)
				return nil
			},
			SubmitRecovered: func(ctx context.Context, submit RecoverySubmitFunc, taskID string, current *task) error {
				if current == nil || current.ID != taskID {
					t.Fatalf("current = %+v", current)
				}
				return submit(taskID)
			},
			ReloadTask: func(_ context.Context, taskID string) (*task, error) {
				return &task{ID: taskID + "-reloaded"}, nil
			},
			ErrUnavailable: errUnavailable,
			ErrEmptyTaskID: errEmpty,
		})

		got, err := service.RecoverNow(context.Background(), " task-1 ")
		if err != nil {
			t.Fatalf("RecoverNow() error = %v", err)
		}
		if got == nil || got.ID != "task-1-reloaded" {
			t.Fatalf("got = %+v", got)
		}
		if len(recovered) != 1 || recovered[0] != "task-1" {
			t.Fatalf("recovered = %#v", recovered)
		}
		if len(submitted) != 1 || submitted[0] != "task-1" {
			t.Fatalf("submitted = %#v", submitted)
		}
	})

	t.Run("rejects empty task id", func(t *testing.T) {
		t.Parallel()

		service := NewRecoveryNowService(RecoveryNowServiceConfig[task]{
			ErrUnavailable: errUnavailable,
			ErrEmptyTaskID: errEmpty,
		})
		if _, err := service.RecoverNow(context.Background(), " "); !errors.Is(err, errEmpty) {
			t.Fatalf("RecoverNow() error = %v, want %v", err, errEmpty)
		}
	})

	t.Run("rejects unavailable submitter", func(t *testing.T) {
		t.Parallel()

		service := NewRecoveryNowService(RecoveryNowServiceConfig[task]{
			LoadTask: func(_ context.Context, taskID string) (*task, error) {
				return &task{ID: taskID}, nil
			},
			ErrUnavailable: errUnavailable,
			ErrEmptyTaskID: errEmpty,
		})
		if _, err := service.RecoverNow(context.Background(), "task-2"); !errors.Is(err, errUnavailable) {
			t.Fatalf("RecoverNow() error = %v, want %v", err, errUnavailable)
		}
	})
}

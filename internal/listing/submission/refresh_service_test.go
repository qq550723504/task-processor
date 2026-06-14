package submission

import (
	"context"
	"errors"
	"testing"
)

func TestStatusRefreshServiceRefreshStatus(t *testing.T) {
	t.Parallel()

	type state struct{ value string }
	type confirmation struct{ value string }

	t.Run("finishes with confirmation on success", func(t *testing.T) {
		t.Parallel()

		var gotLockKey string
		var unlocked bool
		var gotFinishErr error
		service := NewStatusRefreshService(StatusRefreshServiceConfig[state, confirmation, string]{
			LockKeySuffix: "refresh_submission_status",
			LockSubmit: func(key string) func() {
				gotLockKey = key
				return func() { unlocked = true }
			},
			LoadState: func(context.Context, string) (*state, error) {
				return &state{value: "loaded"}, nil
			},
			ResolveConfirmation: func(string, *state) (*confirmation, error) {
				return &confirmation{value: "confirmed"}, nil
			},
			Finish: func(_ context.Context, _ string, loaded *state, resolved *confirmation, resolveErr error) (*string, error) {
				gotFinishErr = resolveErr
				if loaded == nil || resolved == nil {
					t.Fatalf("loaded/resolved = %+v / %+v", loaded, resolved)
				}
				result := loaded.value + ":" + resolved.value
				return &result, nil
			},
		})

		result, err := service.RefreshStatus(context.Background(), "task-1")
		if err != nil {
			t.Fatalf("RefreshStatus() error = %v", err)
		}
		if result == nil || *result != "loaded:confirmed" {
			t.Fatalf("result = %+v", result)
		}
		if gotLockKey != "task-1:refresh_submission_status" {
			t.Fatalf("lock key = %q", gotLockKey)
		}
		if !unlocked {
			t.Fatal("expected unlock to be called")
		}
		if gotFinishErr != nil {
			t.Fatalf("finish resolve err = %v, want nil", gotFinishErr)
		}
	})

	t.Run("finishes with nil confirmation when resolve fails before confirmation", func(t *testing.T) {
		t.Parallel()

		resolveErr := errors.New("remote failed")
		service := NewStatusRefreshService(StatusRefreshServiceConfig[state, confirmation, string]{
			LoadState: func(context.Context, string) (*state, error) {
				return &state{value: "loaded"}, nil
			},
			ResolveConfirmation: func(string, *state) (*confirmation, error) {
				return nil, resolveErr
			},
			Finish: func(_ context.Context, _ string, _ *state, resolved *confirmation, gotErr error) (*string, error) {
				if resolved != nil {
					t.Fatalf("resolved = %+v, want nil", resolved)
				}
				if !errors.Is(gotErr, resolveErr) {
					t.Fatalf("gotErr = %v, want %v", gotErr, resolveErr)
				}
				result := "fallback"
				return &result, nil
			},
		})

		result, err := service.RefreshStatus(context.Background(), "task-2")
		if err != nil {
			t.Fatalf("RefreshStatus() error = %v", err)
		}
		if result == nil || *result != "fallback" {
			t.Fatalf("result = %+v", result)
		}
	})

	t.Run("passes both confirmation and warning error through finish", func(t *testing.T) {
		t.Parallel()

		resolveErr := errors.New("remote partial")
		service := NewStatusRefreshService(StatusRefreshServiceConfig[state, confirmation, string]{
			LoadState: func(context.Context, string) (*state, error) {
				return &state{value: "loaded"}, nil
			},
			ResolveConfirmation: func(string, *state) (*confirmation, error) {
				return &confirmation{value: "confirmed"}, resolveErr
			},
			Finish: func(_ context.Context, _ string, _ *state, resolved *confirmation, gotErr error) (*string, error) {
				if resolved == nil || resolved.value != "confirmed" {
					t.Fatalf("resolved = %+v", resolved)
				}
				if !errors.Is(gotErr, resolveErr) {
					t.Fatalf("gotErr = %v, want %v", gotErr, resolveErr)
				}
				result := "confirmed-with-warning"
				return &result, nil
			},
		})

		result, err := service.RefreshStatus(context.Background(), "task-3")
		if err != nil {
			t.Fatalf("RefreshStatus() error = %v", err)
		}
		if result == nil || *result != "confirmed-with-warning" {
			t.Fatalf("result = %+v", result)
		}
	})
}

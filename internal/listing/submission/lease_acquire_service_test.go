package submission

import (
	"context"
	"errors"
	"testing"
)

func TestLeaseAcquireServiceAcquireReturnsTaskOnSuccess(t *testing.T) {
	t.Parallel()

	service := NewLeaseAcquireService(LeaseAcquireServiceConfig[string, string]{
		BeginLease: func(context.Context, string, string, string) (*string, error) {
			task := "task-1"
			return &task, nil
		},
	})

	task, preview, err := service.Acquire(context.Background(), "task-1", "publish", "req-1")
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	if task == nil || *task != "task-1" {
		t.Fatalf("task = %+v", task)
	}
	if preview != nil {
		t.Fatalf("preview = %+v, want nil", preview)
	}
}

func TestLeaseAcquireServiceAcquireBuildsReplayPreview(t *testing.T) {
	t.Parallel()

	replayErr := errors.New("replay")
	service := NewLeaseAcquireService(LeaseAcquireServiceConfig[string, string]{
		BeginLease: func(context.Context, string, string, string) (*string, error) {
			task := "task-1"
			return &task, replayErr
		},
		IsReplayExisting: func(err error) bool { return errors.Is(err, replayErr) },
		BuildReplayPreview: func(_ context.Context, task *string) (*string, error) {
			preview := "preview:" + *task
			return &preview, nil
		},
	})

	task, preview, err := service.Acquire(context.Background(), "task-1", "publish", "req-1")
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	if task != nil {
		t.Fatalf("task = %+v, want nil", task)
	}
	if preview == nil || *preview != "preview:task-1" {
		t.Fatalf("preview = %+v", preview)
	}
}

func TestLeaseAcquireServiceAcquireRecoversRemotePreview(t *testing.T) {
	t.Parallel()

	recoverErr := errors.New("recover")
	service := NewLeaseAcquireService(LeaseAcquireServiceConfig[string, string]{
		BeginLease: func(context.Context, string, string, string) (*string, error) {
			task := "task-1"
			return &task, recoverErr
		},
		IsRecoverRemote: func(err error) bool { return errors.Is(err, recoverErr) },
		RecoverRemote: func(_ context.Context, task *string, action string) (*string, error) {
			preview := *task + ":" + action
			return &preview, nil
		},
	})

	task, preview, err := service.Acquire(context.Background(), "task-1", "publish", "req-1")
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	if task != nil {
		t.Fatalf("task = %+v, want nil", task)
	}
	if preview == nil || *preview != "task-1:publish" {
		t.Fatalf("preview = %+v", preview)
	}
}

func TestLeaseAcquireServiceAcquireMapsMissingPackageError(t *testing.T) {
	t.Parallel()

	missingErr := errors.New("missing")
	mappedErr := errors.New("blocked")
	service := NewLeaseAcquireService(LeaseAcquireServiceConfig[string, string]{
		BeginLease:         func(context.Context, string, string, string) (*string, error) { return nil, missingErr },
		IsMissingPackage:   func(err error) bool { return errors.Is(err, missingErr) },
		BuildMissingPkgErr: func(err error) error { return mappedErr },
	})

	_, _, err := service.Acquire(context.Background(), "task-1", "publish", "req-1")
	if !errors.Is(err, mappedErr) {
		t.Fatalf("Acquire() error = %v, want %v", err, mappedErr)
	}
}

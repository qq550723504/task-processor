package submission

import (
	"context"
	"errors"
	"testing"
)

func TestStartFailureServiceHandleReturnsOriginalErrorAfterRecordAndClear(t *testing.T) {
	t.Parallel()

	originalErr := errors.New("start failed")
	var calls []string
	service := NewStartFailureService(StartFailureServiceConfig[error]{
		RecordFailure: func(context.Context, error) error {
			calls = append(calls, "record")
			return nil
		},
		ClearFailure: func(context.Context, error) error {
			calls = append(calls, "clear")
			return nil
		},
		OriginalError: func(err error) error { return err },
	})

	err := service.Handle(context.Background(), originalErr)
	if !errors.Is(err, originalErr) {
		t.Fatalf("Handle() error = %v, want %v", err, originalErr)
	}
	if len(calls) != 2 || calls[0] != "record" || calls[1] != "clear" {
		t.Fatalf("calls = %+v", calls)
	}
}

func TestStartFailureServiceHandlePrefersRecordFailureError(t *testing.T) {
	t.Parallel()

	originalErr := errors.New("start failed")
	recordErr := errors.New("persist failed")
	service := NewStartFailureService(StartFailureServiceConfig[error]{
		RecordFailure: func(context.Context, error) error { return recordErr },
		OriginalError: func(err error) error { return err },
	})

	err := service.Handle(context.Background(), originalErr)
	if !errors.Is(err, recordErr) {
		t.Fatalf("Handle() error = %v, want %v", err, recordErr)
	}
}

func TestStartFailureServiceHandleJoinsRecordAndClearErrors(t *testing.T) {
	t.Parallel()

	originalErr := errors.New("start failed")
	recordErr := errors.New("persist failed")
	clearErr := errors.New("clear failed")
	service := NewStartFailureService(StartFailureServiceConfig[error]{
		RecordFailure: func(context.Context, error) error { return recordErr },
		ClearFailure:  func(context.Context, error) error { return clearErr },
		OriginalError: func(err error) error { return err },
	})

	err := service.Handle(context.Background(), originalErr)
	if !errors.Is(err, recordErr) || !errors.Is(err, clearErr) {
		t.Fatalf("Handle() error = %v, want joined %v and %v", err, recordErr, clearErr)
	}
}

func TestStartFailureServiceHandlePrefersClearErrorWhenRecordReturnsOriginal(t *testing.T) {
	t.Parallel()

	originalErr := errors.New("start failed")
	clearErr := errors.New("clear failed")
	service := NewStartFailureService(StartFailureServiceConfig[error]{
		RecordFailure: func(context.Context, error) error { return originalErr },
		ClearFailure:  func(context.Context, error) error { return clearErr },
		OriginalError: func(err error) error { return err },
	})

	err := service.Handle(context.Background(), originalErr)
	if !errors.Is(err, clearErr) {
		t.Fatalf("Handle() error = %v, want %v", err, clearErr)
	}
}

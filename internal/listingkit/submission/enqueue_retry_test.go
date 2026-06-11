package submission

import (
	"errors"
	"testing"
	"time"

	"task-processor/internal/infra/worker"
)

func TestNextEnqueueRetryDelay(t *testing.T) {
	t.Parallel()

	if got := NextEnqueueRetryDelay(0); got != EnqueueRetryDelay {
		t.Fatalf("NextEnqueueRetryDelay(0) = %v, want %v", got, EnqueueRetryDelay)
	}
	if got := NextEnqueueRetryDelay(EnqueueRetryDelay); got != 2*EnqueueRetryDelay {
		t.Fatalf("NextEnqueueRetryDelay(base) = %v, want %v", got, 2*EnqueueRetryDelay)
	}
	if got := NextEnqueueRetryDelay(EnqueueRetryMaxDelay); got != EnqueueRetryMaxDelay {
		t.Fatalf("NextEnqueueRetryDelay(max) = %v, want %v", got, EnqueueRetryMaxDelay)
	}
}

func TestBoundedEnqueueRetryDelay(t *testing.T) {
	t.Parallel()

	if got := BoundedEnqueueRetryDelay(1); got != EnqueueRetryDelay {
		t.Fatalf("BoundedEnqueueRetryDelay(1) = %v, want %v", got, EnqueueRetryDelay)
	}
	if got := BoundedEnqueueRetryDelay(2); got != 2*EnqueueRetryDelay {
		t.Fatalf("BoundedEnqueueRetryDelay(2) = %v, want %v", got, 2*EnqueueRetryDelay)
	}
	if got := BoundedEnqueueRetryDelay(10); got != EnqueueRetryMaxDelay {
		t.Fatalf("BoundedEnqueueRetryDelay(10) = %v, want %v", got, EnqueueRetryMaxDelay)
	}
}

func TestRetryEnqueueSubmit(t *testing.T) {
	t.Parallel()

	t.Run("succeeds after queue full retry", func(t *testing.T) {
		t.Parallel()

		attempts := 0
		err := RetryEnqueueSubmit("task-1", 300*time.Millisecond, func(string) error {
			attempts++
			if attempts == 1 {
				return worker.ErrQueueFull
			}
			return nil
		})
		if err != nil {
			t.Fatalf("RetryEnqueueSubmit() error = %v", err)
		}
		if attempts != 2 {
			t.Fatalf("attempts = %d, want 2", attempts)
		}
	})

	t.Run("returns non queue full error immediately", func(t *testing.T) {
		t.Parallel()

		wantErr := errors.New("submit failed")
		err := RetryEnqueueSubmit("task-2", time.Second, func(string) error {
			return wantErr
		})
		if !errors.Is(err, wantErr) {
			t.Fatalf("RetryEnqueueSubmit() error = %v, want %v", err, wantErr)
		}
	})
}

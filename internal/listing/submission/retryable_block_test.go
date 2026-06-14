package submission

import (
	"errors"
	"testing"
	"time"
)

func TestClassifyRetryableFailure(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name           string
		err            error
		wantReasonCode string
		wantRetryable  bool
	}{
		{
			name:           "insufficient credits",
			err:            errors.New("OpenAI API error: insufficient credits in account balance"),
			wantReasonCode: retryableBlockReasonCodeOpenAIInsufficientCredits,
			wantRetryable:  true,
		},
		{
			name:           "worker queue full",
			err:            errors.New("queue full"),
			wantReasonCode: retryableBlockReasonCodeWorkerQueueBackpressure,
			wantRetryable:  true,
		},
		{
			name:           "non retryable",
			err:            errors.New("validation failed: missing required category_id"),
			wantReasonCode: "",
			wantRetryable:  false,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			block, ok := ClassifyRetryableFailure(tc.err, "task")
			if ok != tc.wantRetryable {
				t.Fatalf("ClassifyRetryableFailure() ok = %t, want %t", ok, tc.wantRetryable)
			}
			if !tc.wantRetryable {
				if block != nil {
					t.Fatalf("ClassifyRetryableFailure() block = %+v, want nil", block)
				}
				return
			}
			if block == nil {
				t.Fatal("ClassifyRetryableFailure() block = nil, want retryable block")
			}
			if block.ReasonCode != tc.wantReasonCode {
				t.Fatalf("ReasonCode = %q, want %q", block.ReasonCode, tc.wantReasonCode)
			}
			if block.RecoveryScope != "task" {
				t.Fatalf("RecoveryScope = %q, want task", block.RecoveryScope)
			}
			if !block.AutoResumeEnabled {
				t.Fatal("AutoResumeEnabled = false, want true")
			}
		})
	}
}

func TestBuildReblockedRetryableBlockPreservesManualPause(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 6, 17, 45, 0, 0, time.UTC)
	previous := &RetryableBlockState{
		ReasonCode:           retryableBlockReasonCodeWorkerQueueBackpressure,
		ReasonMessage:        "queue full",
		BlockedAt:            now.Add(-5 * time.Minute),
		RetryAttempts:        1,
		MaxAutoRetryAttempts: 8,
		RecoveryScope:        "task",
		AutoResumeEnabled:    false,
		AutoRetryPaused:      true,
	}
	classified, ok := ClassifyRetryableFailure(errors.New("queue full"), "task")
	if !ok {
		t.Fatal("ClassifyRetryableFailure() = not retryable, want queue full classification")
	}

	reblocked := BuildReblockedRetryableBlock(previous, classified, now, "task")
	if reblocked.AutoResumeEnabled {
		t.Fatal("AutoResumeEnabled = true, want preserved false")
	}
	if !reblocked.AutoRetryPaused {
		t.Fatal("AutoRetryPaused = false, want preserved true")
	}
	if reblocked.NextRetryAt != nil {
		t.Fatalf("NextRetryAt = %v, want nil while pause is preserved", reblocked.NextRetryAt)
	}
}

func TestBuildBackfilledRetryableBlock(t *testing.T) {
	t.Parallel()

	backfilledAt := time.Date(2026, 6, 6, 18, 0, 0, 0, time.UTC)
	blockedAt := backfilledAt.Add(-time.Hour)

	block, ok := BuildBackfilledRetryableBlock(
		errors.New("openai request failed: insufficient credits"),
		blockedAt,
		backfilledAt,
		8,
		"task",
	)
	if !ok || block == nil {
		t.Fatalf("BuildBackfilledRetryableBlock() = (%+v, %t), want retryable block", block, ok)
	}
	if block.ReasonCode != retryableBlockReasonCodeOpenAIInsufficientCredits {
		t.Fatalf("ReasonCode = %q, want %q", block.ReasonCode, retryableBlockReasonCodeOpenAIInsufficientCredits)
	}
	if !block.BlockedAt.Equal(blockedAt) {
		t.Fatalf("BlockedAt = %v, want %v", block.BlockedAt, blockedAt)
	}
	if block.RetryAttempts != 0 {
		t.Fatalf("RetryAttempts = %d, want 0", block.RetryAttempts)
	}
	if block.NextRetryAt == nil || !block.NextRetryAt.Equal(backfilledAt.Add(BoundedEnqueueRetryDelay(1))) {
		t.Fatalf("NextRetryAt = %v, want %v", block.NextRetryAt, backfilledAt.Add(BoundedEnqueueRetryDelay(1)))
	}
}

func TestPersistClassifiedRetryableFailure(t *testing.T) {
	t.Parallel()

	t.Run("marks blocked retryable when classified", func(t *testing.T) {
		t.Parallel()

		calledBlocked := false
		calledFailed := false
		err := PersistClassifiedRetryableFailure(RetryableFailurePersistenceRequest{
			DefaultRecoveryScope: "task",
			ErrorMessage:         "queue full",
			Cause:                errors.New("queue full"),
			MarkBlockedRetryable: func(block *RetryableBlockState, errorMessage string) error {
				calledBlocked = true
				if block == nil {
					t.Fatal("block = nil, want retryable block")
				}
				if block.ReasonCode != retryableBlockReasonCodeWorkerQueueBackpressure {
					t.Fatalf("ReasonCode = %q, want %q", block.ReasonCode, retryableBlockReasonCodeWorkerQueueBackpressure)
				}
				if errorMessage != "queue full" {
					t.Fatalf("error message = %q, want queue full", errorMessage)
				}
				return nil
			},
			MarkFailed: func(string) error {
				calledFailed = true
				return nil
			},
		})
		if err != nil {
			t.Fatalf("PersistClassifiedRetryableFailure() error = %v", err)
		}
		if !calledBlocked {
			t.Fatal("MarkBlockedRetryable was not called")
		}
		if calledFailed {
			t.Fatal("MarkFailed was called for retryable failure")
		}
	})

	t.Run("marks failed when not retryable", func(t *testing.T) {
		t.Parallel()

		calledBlocked := false
		calledFailed := false
		err := PersistClassifiedRetryableFailure(RetryableFailurePersistenceRequest{
			DefaultRecoveryScope: "task",
			ErrorMessage:         "validation failed",
			Cause:                errors.New("validation failed"),
			MarkBlockedRetryable: func(*RetryableBlockState, string) error {
				calledBlocked = true
				return nil
			},
			MarkFailed: func(errorMessage string) error {
				calledFailed = true
				if errorMessage != "validation failed" {
					t.Fatalf("error message = %q, want validation failed", errorMessage)
				}
				return nil
			},
		})
		if err != nil {
			t.Fatalf("PersistClassifiedRetryableFailure() error = %v", err)
		}
		if calledBlocked {
			t.Fatal("MarkBlockedRetryable was called for non-retryable failure")
		}
		if !calledFailed {
			t.Fatal("MarkFailed was not called")
		}
	})
}

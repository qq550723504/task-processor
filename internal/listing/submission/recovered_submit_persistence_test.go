package submission

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSubmitRecoveredWithRetryablePersistenceReblocksRetryableSubmitFailure(t *testing.T) {
	t.Parallel()

	recoveredAt := time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC)
	previous := &RetryableBlockState{
		ReasonCode:           RetryableReasonCodeWorkerQueueBackpressure,
		ReasonMessage:        "queue full",
		BlockedAt:            recoveredAt.Add(-time.Hour),
		RetryAttempts:        1,
		MaxAutoRetryAttempts: 8,
		RecoveryScope:        RetryableRecoveryScopeTask,
		AutoResumeEnabled:    true,
	}
	var marked *RetryableBlockState
	var markedError string

	err := SubmitRecoveredWithRetryablePersistence(RecoveredSubmitPersistenceRequest{
		TaskID:               "task-1",
		PreviousBlock:        previous,
		RecoveredAt:          recoveredAt,
		DefaultRecoveryScope: RetryableRecoveryScopeTask,
		Submit: func(taskID string) error {
			if taskID != "task-1" {
				t.Fatalf("taskID = %q, want task-1", taskID)
			}
			return errors.New("queue full")
		},
		MarkBlockedRetryable: func(block *RetryableBlockState, errorMsg string) error {
			marked = CloneRetryableBlockState(block)
			markedError = errorMsg
			return nil
		},
	})

	if err == nil || !strings.Contains(err.Error(), "submit recovered task task-1") {
		t.Fatalf("error = %v, want submit recovered task failure", err)
	}
	if marked == nil {
		t.Fatal("marked block = nil, want retryable block")
	}
	if marked.RetryAttempts != 2 {
		t.Fatalf("RetryAttempts = %d, want 2", marked.RetryAttempts)
	}
	if marked.NextRetryAt == nil {
		t.Fatal("NextRetryAt = nil, want next retry time")
	}
	if markedError != "failed to submit task: queue full" {
		t.Fatalf("marked error = %q, want formatted submit failure", markedError)
	}
}

func TestSubmitRecoveredWithRetryablePersistenceRestoresWhenReblockPersistenceFails(t *testing.T) {
	t.Parallel()

	recoveredAt := time.Date(2026, 6, 17, 11, 0, 0, 0, time.UTC)
	restoreCalled := false

	err := SubmitRecoveredWithRetryablePersistence(RecoveredSubmitPersistenceRequest{
		TaskID:               "task-restore",
		RecoveredAt:          recoveredAt,
		DefaultRecoveryScope: RetryableRecoveryScopeTask,
		Submit: func(string) error {
			return errors.New("queue full")
		},
		MarkBlockedRetryable: func(*RetryableBlockState, string) error {
			return errors.New("persist blocked state failed")
		},
		RestoreDurability: func(errorMsg string, submitErr error, persistErr error) error {
			restoreCalled = true
			if errorMsg != "failed to submit task: queue full" {
				t.Fatalf("errorMsg = %q, want formatted submit failure", errorMsg)
			}
			if submitErr == nil || submitErr.Error() != "queue full" {
				t.Fatalf("submitErr = %v, want queue full", submitErr)
			}
			if persistErr == nil || !strings.Contains(persistErr.Error(), "mark blocked retryable") {
				t.Fatalf("persistErr = %v, want wrapped mark blocked retryable", persistErr)
			}
			return errors.Join(submitErr, persistErr)
		},
	})

	if err == nil {
		t.Fatal("error = nil, want restore error")
	}
	if !restoreCalled {
		t.Fatal("RestoreDurability was not called")
	}
}

func TestSubmitRecoveredWithRetryablePersistencePersistsNonRetryableFailure(t *testing.T) {
	t.Parallel()

	persistCalled := false

	err := SubmitRecoveredWithRetryablePersistence(RecoveredSubmitPersistenceRequest{
		TaskID: "task-failed",
		Submit: func(string) error {
			return errors.New("validation failed")
		},
		PersistFailure: func(errorMsg string, submitErr error) error {
			persistCalled = true
			if errorMsg != "failed to submit task: validation failed" {
				t.Fatalf("errorMsg = %q, want formatted submit failure", errorMsg)
			}
			if submitErr == nil || submitErr.Error() != "validation failed" {
				t.Fatalf("submitErr = %v, want validation failed", submitErr)
			}
			return nil
		},
	})

	if err == nil || !strings.Contains(err.Error(), "submit recovered task task-failed") {
		t.Fatalf("error = %v, want submit recovered task failure", err)
	}
	if !persistCalled {
		t.Fatal("PersistFailure was not called")
	}
}

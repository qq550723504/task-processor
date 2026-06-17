package listingkit

import (
	"testing"
	"time"
)

func TestBuildRecoveredRetryableBlockAdaptsSubmissionPolicy(t *testing.T) {
	t.Parallel()

	recoveredAt := time.Date(2026, 6, 17, 13, 0, 0, 0, time.UTC)
	nextRetryAt := recoveredAt.Add(time.Minute)
	previous := &RetryableBlock{
		ReasonCode:           "worker_queue_backpressure",
		ReasonMessage:        "queue full",
		BlockedAt:            recoveredAt.Add(-time.Hour),
		NextRetryAt:          &nextRetryAt,
		RetryAttempts:        2,
		MaxAutoRetryAttempts: 8,
		RecoveryScope:        "task",
		AutoResumeEnabled:    true,
		AutoRetryPaused:      true,
	}

	recovered := BuildRecoveredRetryableBlock(previous, recoveredAt)
	if recovered == nil {
		t.Fatal("BuildRecoveredRetryableBlock() = nil, want retryable block")
	}
	if recovered == previous {
		t.Fatal("BuildRecoveredRetryableBlock() reused previous pointer, want clone")
	}
	if recovered.LastRetryAt == nil || !recovered.LastRetryAt.Equal(recoveredAt) {
		t.Fatalf("LastRetryAt = %v, want %v", recovered.LastRetryAt, recoveredAt)
	}
	if recovered.NextRetryAt != nil {
		t.Fatalf("NextRetryAt = %v, want nil", recovered.NextRetryAt)
	}
	if recovered.AutoRetryPaused {
		t.Fatal("AutoRetryPaused = true, want false")
	}
	if recovered.ReasonCode != previous.ReasonCode || recovered.RetryAttempts != previous.RetryAttempts {
		t.Fatalf("recovered block = %+v, want preserved reason and attempts from %+v", recovered, previous)
	}
}

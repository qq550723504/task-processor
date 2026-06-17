package listingkit

import (
	"os"
	"strings"
	"testing"
	"time"

	submissiondomain "task-processor/internal/listing/submission"
)

func TestTaskStatusBlockedRetryable_IsNotTerminal(t *testing.T) {
	t.Parallel()

	if taskStatusIsTerminal(TaskStatusBlockedRetryable) {
		t.Fatalf("taskStatusIsTerminal(%q) = true, want false", TaskStatusBlockedRetryable)
	}
}

func TestTaskResultLifecycleCarriesRetryableBlock(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	nextRetryAt := now.Add(30 * time.Minute)
	task := &Task{
		ID:        "task-retryable-block-1",
		TenantID:  "tenant-1",
		Status:    TaskStatusBlockedRetryable,
		Error:     "worker queue temporarily unavailable",
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now,
		RetryableBlock: &RetryableBlock{
			ReasonCode:           submissiondomain.RetryableReasonCodeWorkerQueueBackpressure,
			ReasonMessage:        "工作队列已满",
			BlockedAt:            now.Add(-15 * time.Minute),
			LastRetryAt:          &now,
			NextRetryAt:          &nextRetryAt,
			RetryAttempts:        3,
			MaxAutoRetryAttempts: 12,
			RecoveryScope:        submissiondomain.RetryableRecoveryScopeTask,
			AutoResumeEnabled:    true,
		},
	}

	result := buildTaskResult(task, nil)
	if result == nil {
		t.Fatal("buildTaskResult() = nil, want payload")
	}
	if result.CompletedAt != nil {
		t.Fatalf("CompletedAt = %v, want nil for non-terminal blocked_retryable", result.CompletedAt)
	}
	if result.RetryableBlock == nil {
		t.Fatal("RetryableBlock = nil, want carried retryable block")
	}
	if result.RetryableBlock.ReasonCode != submissiondomain.RetryableReasonCodeWorkerQueueBackpressure {
		t.Fatalf("ReasonCode = %q, want %q", result.RetryableBlock.ReasonCode, submissiondomain.RetryableReasonCodeWorkerQueueBackpressure)
	}
	if result.RetryableBlock.NextRetryAt == nil || !result.RetryableBlock.NextRetryAt.Equal(nextRetryAt) {
		t.Fatalf("NextRetryAt = %v, want %v", result.RetryableBlock.NextRetryAt, nextRetryAt)
	}
}

func TestRetryableBlockScanNilValue(t *testing.T) {
	t.Parallel()

	block := &RetryableBlock{
		ReasonCode:    "should-reset",
		ReasonMessage: "should-reset",
	}
	if err := block.Scan(nil); err != nil {
		t.Fatalf("Scan(nil) error = %v, want nil", err)
	}
	if block.ReasonCode != "" || block.ReasonMessage != "" || block.LastRetryAt != nil || block.NextRetryAt != nil {
		t.Fatalf("Scan(nil) = %+v, want zero value", *block)
	}
}

func TestRetryableBlockCloneDeepCopiesRetryPointers(t *testing.T) {
	t.Parallel()

	lastRetryAt := time.Now().UTC()
	nextRetryAt := lastRetryAt.Add(45 * time.Minute)
	src := &RetryableBlock{
		ReasonCode:        " worker_queue_backpressure ",
		ReasonMessage:     " queue full ",
		RecoveryScope:     " task ",
		LastRetryAt:       &lastRetryAt,
		NextRetryAt:       &nextRetryAt,
		AutoResumeEnabled: true,
	}

	cloned := cloneRetryableBlock(src)
	if cloned == nil {
		t.Fatal("cloneRetryableBlock() = nil, want clone")
	}
	if cloned.LastRetryAt == src.LastRetryAt {
		t.Fatal("LastRetryAt pointers alias, want deep copy")
	}
	if cloned.NextRetryAt == src.NextRetryAt {
		t.Fatal("NextRetryAt pointers alias, want deep copy")
	}
	if !cloned.LastRetryAt.Equal(lastRetryAt) {
		t.Fatalf("LastRetryAt = %v, want %v", cloned.LastRetryAt, lastRetryAt)
	}
	if !cloned.NextRetryAt.Equal(nextRetryAt) {
		t.Fatalf("NextRetryAt = %v, want %v", cloned.NextRetryAt, nextRetryAt)
	}

	updatedLastRetryAt := lastRetryAt.Add(2 * time.Minute)
	updatedNextRetryAt := nextRetryAt.Add(2 * time.Minute)
	*src.LastRetryAt = updatedLastRetryAt
	*src.NextRetryAt = updatedNextRetryAt

	if cloned.LastRetryAt.Equal(updatedLastRetryAt) {
		t.Fatalf("cloned LastRetryAt mutated to %v, want original %v", cloned.LastRetryAt, lastRetryAt)
	}
	if cloned.NextRetryAt.Equal(updatedNextRetryAt) {
		t.Fatalf("cloned NextRetryAt mutated to %v, want original %v", cloned.NextRetryAt, nextRetryAt)
	}
}

func TestRetryableBlockReasonCodesUseSubmissionDomainConstants(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("retryable_block.go")
	if err != nil {
		t.Fatalf("ReadFile(retryable_block.go) error = %v", err)
	}
	content := string(source)
	if strings.Contains(content, "retryableBlockReasonCode") {
		t.Fatal("retryable_block.go should not keep root retryable reason-code aliases")
	}
	if strings.Contains(content, "retryableRecoveryScopeTask") {
		t.Fatal("retryable_block.go should not keep root retryable recovery-scope aliases")
	}
}

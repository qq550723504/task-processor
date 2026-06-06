package listingkit

import (
	"context"
	"testing"
	"time"

	listingsubmission "task-processor/internal/listingkit/submission"
	sheinpub "task-processor/internal/publishing/shein"
)

func TestTaskSubmissionRecoveryServiceBeginSheinSubmitLeaseReplaysExistingRequest(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	now := time.Now().Add(-time.Minute)
	record := completeSheinSubmitAttempt(task.Result.Shein, "publish", "replay-123", &sheinpub.SubmissionResponse{
		Code:    "0",
		Message: "success",
		Success: true,
		SPUName: "SPU-123",
	}, nil, now)
	appendSheinSubmissionEvent(task.Result.Shein, listingsubmission.BuildEvent(task.ID, "publish", record, record.Result, nil, record.StartedAt))
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	recovery := newTaskSubmissionRecoveryService(taskSubmissionRecoveryServiceConfig{
		repo: repo,
	})

	got, err := recovery.beginSheinSubmitLease(context.Background(), task.ID, "publish", "replay-123", time.Now())
	if err != errSheinSubmitReplayExisting {
		t.Fatalf("beginSheinSubmitLease() err = %v, want %v", err, errSheinSubmitReplayExisting)
	}
	if got == nil || got.ID != task.ID {
		t.Fatalf("task = %+v, want original task", got)
	}
}

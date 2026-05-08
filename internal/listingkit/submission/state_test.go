package submission

import (
	"errors"
	"testing"
	"time"

	sheinpub "task-processor/internal/publishing/shein"
)

func TestBeginAttemptRecordsCurrentState(t *testing.T) {
	t.Parallel()

	pkg := &sheinpub.Package{}
	startedAt := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)

	record := BeginAttempt(pkg, "publish", "idem-1", sheinpub.SubmissionPhaseValidate, startedAt, InFlightTTL)

	if record == nil || record.Status != sheinpub.SubmissionStatusRunning {
		t.Fatalf("record = %+v", record)
	}
	if pkg.Submission == nil || pkg.Submission.CurrentRequestID != "idem-1" {
		t.Fatalf("submission = %+v", pkg.Submission)
	}
	if pkg.Submission.Publish == nil || pkg.Submission.Publish.RequestID != "idem-1" {
		t.Fatalf("publish record = %+v", pkg.Submission.Publish)
	}
}

func TestCompleteAttemptClearsInFlightAndWritesResult(t *testing.T) {
	t.Parallel()

	pkg := &sheinpub.Package{}
	startedAt := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	finishedAt := startedAt.Add(2 * time.Minute)
	BeginAttempt(pkg, "publish", "idem-1", sheinpub.SubmissionPhaseValidate, startedAt, InFlightTTL)

	record := CompleteAttempt(pkg, "publish", "idem-1", &sheinpub.SubmissionResponse{Success: true}, nil, finishedAt)

	if record == nil || record.Status != sheinpub.SubmissionStatusSuccess {
		t.Fatalf("record = %+v", record)
	}
	if pkg.Submission.CurrentRequestID != "" || pkg.Submission.InFlightStartedAt != nil {
		t.Fatalf("expected in-flight state cleared, got %+v", pkg.Submission)
	}
	if pkg.Submission.LastResult == nil || !pkg.Submission.LastResult.Success {
		t.Fatalf("last result = %+v", pkg.Submission.LastResult)
	}
}

func TestFailAttemptRecordsError(t *testing.T) {
	t.Parallel()

	pkg := &sheinpub.Package{}
	startedAt := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	finishedAt := startedAt.Add(time.Minute)
	BeginAttempt(pkg, "publish", "idem-1", sheinpub.SubmissionPhasePrepareProduct, startedAt, InFlightTTL)

	record := FailAttempt(pkg, "publish", "idem-1", sheinpub.SubmissionPhasePrepareProduct, errors.New("submit failed"), finishedAt)

	if record == nil || record.Status != sheinpub.SubmissionStatusFailed || record.Error != "submit failed" {
		t.Fatalf("record = %+v", record)
	}
	if pkg.Submission.CurrentRequestID != "" {
		t.Fatalf("expected current request cleared, got %+v", pkg.Submission)
	}
}

func TestFindActiveAttemptHonorsLeaseExpiry(t *testing.T) {
	t.Parallel()

	pkg := &sheinpub.Package{}
	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	BeginAttempt(pkg, "publish", "idem-1", sheinpub.SubmissionPhaseSubmitRemote, now.Add(-InFlightTTL-time.Minute), InFlightTTL)

	if active := FindActiveAttempt(pkg, "publish", now, InFlightTTL); active != nil {
		t.Fatalf("active = %+v, want nil after lease expiry", active)
	}
	if !NeedsRemoteRecovery(pkg.Submission, "publish", now, InFlightTTL) {
		t.Fatal("expected remote recovery after lease expiry")
	}
}

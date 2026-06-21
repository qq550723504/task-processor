package shein

import (
	"testing"
	"time"
)

const submitStateTestTTL = 10 * time.Minute

func TestBeginSubmitAttemptRecordsCurrentState(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)
	pkg := &Package{}

	record := BeginSubmitAttempt(pkg, "publish", "idem-1", SubmissionPhaseValidate, startedAt, submitStateTestTTL)

	if record == nil {
		t.Fatal("expected attempt record")
	}
	if pkg.SubmissionState == nil {
		t.Fatal("expected submission report")
	}
	if pkg.SubmissionState.CurrentAction != "publish" {
		t.Fatalf("current action = %q, want publish", pkg.SubmissionState.CurrentAction)
	}
	if pkg.SubmissionState.CurrentPhase != SubmissionPhaseValidate {
		t.Fatalf("current phase = %q, want %q", pkg.SubmissionState.CurrentPhase, SubmissionPhaseValidate)
	}
	if pkg.SubmissionState.CurrentRequestID != "idem-1" {
		t.Fatalf("current request id = %q, want idem-1", pkg.SubmissionState.CurrentRequestID)
	}
	if pkg.SubmissionState.InFlightStartedAt == nil || !pkg.SubmissionState.InFlightStartedAt.Equal(startedAt) {
		t.Fatalf("in-flight started at = %v, want %v", pkg.SubmissionState.InFlightStartedAt, startedAt)
	}
	if pkg.SubmissionState.AttemptCount != 1 {
		t.Fatalf("attempt count = %d, want 1", pkg.SubmissionState.AttemptCount)
	}
	if record.RequestID != "idem-1" || record.Attempt != 1 || !record.StartedAt.Equal(startedAt) {
		t.Fatalf("record = %+v, want request id idem-1 attempt 1 started_at %v", record, startedAt)
	}
}

func TestAdvanceSubmitPhasePreservesAttempt(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)
	pkg := &Package{}
	BeginSubmitAttempt(pkg, "publish", "idem-1", SubmissionPhaseValidate, startedAt, submitStateTestTTL)

	AdvanceSubmitPhaseAt(pkg, "publish", "idem-1", SubmissionPhaseUploadImages, startedAt.Add(time.Second), submitStateTestTTL)

	if pkg.SubmissionState.CurrentPhase != SubmissionPhaseUploadImages {
		t.Fatalf("current phase = %q, want %q", pkg.SubmissionState.CurrentPhase, SubmissionPhaseUploadImages)
	}
	if pkg.SubmissionState.CurrentRequestID != "idem-1" {
		t.Fatalf("current request id = %q, want idem-1", pkg.SubmissionState.CurrentRequestID)
	}
	if pkg.SubmissionState.AttemptCount != 1 {
		t.Fatalf("attempt count = %d, want 1", pkg.SubmissionState.AttemptCount)
	}
	if pkg.SubmissionState.Publish == nil || pkg.SubmissionState.Publish.Phase != SubmissionPhaseUploadImages {
		t.Fatalf("publish record = %+v, want upload_images phase", pkg.SubmissionState.Publish)
	}
}

func TestCompleteSubmitAttemptClearsInFlightAndWritesResult(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)
	finishedAt := startedAt.Add(2 * time.Second)
	pkg := &Package{}
	BeginSubmitAttempt(pkg, "publish", "idem-1", SubmissionPhaseValidate, startedAt, submitStateTestTTL)

	record := CompleteSubmitAttemptAt(pkg, "publish", "idem-1", &SubmissionResponse{Success: true}, nil, finishedAt)

	if record == nil {
		t.Fatal("expected completed record")
	}
	if pkg.SubmissionState.CurrentAction != "" || pkg.SubmissionState.CurrentPhase != "" || pkg.SubmissionState.CurrentRequestID != "" || pkg.SubmissionState.InFlightStartedAt != nil {
		t.Fatalf("current state was not cleared: %+v", pkg.SubmissionState)
	}
	if record.Status != SubmissionStatusSuccess {
		t.Fatalf("status = %q, want %q", record.Status, SubmissionStatusSuccess)
	}
	if record.RequestID != "idem-1" || !record.StartedAt.Equal(startedAt) || record.FinishedAt == nil || !record.FinishedAt.Equal(finishedAt) {
		t.Fatalf("record timing/request = %+v, want request id and start/finish times", record)
	}
	if !record.SubmittedAt.Equal(startedAt) {
		t.Fatalf("submitted_at = %v, want original started_at %v", record.SubmittedAt, startedAt)
	}
	if pkg.SubmissionState.SubmittedAt == nil || !pkg.SubmissionState.SubmittedAt.Equal(startedAt) {
		t.Fatalf("submission submitted_at = %v, want original started_at %v", pkg.SubmissionState.SubmittedAt, startedAt)
	}
}

func TestFailSubmitAttemptWithResponseAndBuildEventPreservesResponse(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)
	finishedAt := startedAt.Add(time.Second)
	pkg := &Package{}
	BeginSubmitAttempt(pkg, "publish", "idem-1", SubmissionPhaseConfirmRemote, startedAt, submitStateTestTTL)
	response := &SubmissionResponse{Success: true, Message: "remote payload"}

	record, event := FailSubmitAttemptWithResponseAndBuildEvent(pkg, "task-1", "publish", "idem-1", SubmissionPhaseConfirmRemote, response, errSubmitStateTest, finishedAt)

	if record == nil {
		t.Fatal("expected failed record")
	}
	if event.Response != response {
		t.Fatalf("event response = %+v, want original response", event.Response)
	}
	if event.Status != SubmissionStatusFailed || event.Phase != SubmissionPhaseConfirmRemote {
		t.Fatalf("event = %+v, want failed confirm_remote event", event)
	}
}

var errSubmitStateTest = submitStateError("submit state test error")

type submitStateError string

func (e submitStateError) Error() string {
	return string(e)
}

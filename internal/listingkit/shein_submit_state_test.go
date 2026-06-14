package listingkit

import (
	"testing"
	"time"

	sheinpub "task-processor/internal/publishing/shein"
)

func TestBeginSheinSubmitAttemptRecordsCurrentState(t *testing.T) {
	startedAt := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)
	pkg := &SheinPackage{}

	record := beginSheinSubmitAttempt(pkg, "publish", "idem-1", sheinpub.SubmissionPhaseValidate, startedAt)

	if record == nil {
		t.Fatal("expected attempt record")
	}
	if pkg.Submission == nil {
		t.Fatal("expected submission report")
	}
	if pkg.Submission.CurrentAction != "publish" {
		t.Fatalf("current action = %q, want publish", pkg.Submission.CurrentAction)
	}
	if pkg.Submission.CurrentPhase != sheinpub.SubmissionPhaseValidate {
		t.Fatalf("current phase = %q, want %q", pkg.Submission.CurrentPhase, sheinpub.SubmissionPhaseValidate)
	}
	if pkg.Submission.CurrentRequestID != "idem-1" {
		t.Fatalf("current request id = %q, want idem-1", pkg.Submission.CurrentRequestID)
	}
	if pkg.Submission.InFlightStartedAt == nil || !pkg.Submission.InFlightStartedAt.Equal(startedAt) {
		t.Fatalf("in-flight started at = %v, want %v", pkg.Submission.InFlightStartedAt, startedAt)
	}
	if pkg.Submission.AttemptCount != 1 {
		t.Fatalf("attempt count = %d, want 1", pkg.Submission.AttemptCount)
	}
	if record.RequestID != "idem-1" || record.Attempt != 1 || !record.StartedAt.Equal(startedAt) {
		t.Fatalf("record = %+v, want request id idem-1 attempt 1 started_at %v", record, startedAt)
	}
}

func TestAdvanceSheinSubmitPhasePreservesAttempt(t *testing.T) {
	startedAt := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)
	pkg := &SheinPackage{}
	beginSheinSubmitAttempt(pkg, "publish", "idem-1", sheinpub.SubmissionPhaseValidate, startedAt)

	advanceSheinSubmitPhase(pkg, "publish", "idem-1", sheinpub.SubmissionPhaseUploadImages)

	if pkg.Submission.CurrentPhase != sheinpub.SubmissionPhaseUploadImages {
		t.Fatalf("current phase = %q, want %q", pkg.Submission.CurrentPhase, sheinpub.SubmissionPhaseUploadImages)
	}
	if pkg.Submission.CurrentRequestID != "idem-1" {
		t.Fatalf("current request id = %q, want idem-1", pkg.Submission.CurrentRequestID)
	}
	if pkg.Submission.AttemptCount != 1 {
		t.Fatalf("attempt count = %d, want 1", pkg.Submission.AttemptCount)
	}
	if pkg.Submission.Publish == nil || pkg.Submission.Publish.Phase != sheinpub.SubmissionPhaseUploadImages {
		t.Fatalf("publish record = %+v, want upload_images phase", pkg.Submission.Publish)
	}
}

func TestCompleteSheinSubmitAttemptClearsInFlightAndWritesResult(t *testing.T) {
	startedAt := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)
	finishedAt := startedAt.Add(2 * time.Second)
	pkg := &SheinPackage{}
	beginSheinSubmitAttempt(pkg, "publish", "idem-1", sheinpub.SubmissionPhaseValidate, startedAt)

	record := completeSheinSubmitAttempt(pkg, "publish", "idem-1", &sheinpub.SubmissionResponse{Success: true}, nil, finishedAt)

	if record == nil {
		t.Fatal("expected completed record")
	}
	if pkg.Submission.CurrentAction != "" || pkg.Submission.CurrentPhase != "" || pkg.Submission.CurrentRequestID != "" || pkg.Submission.InFlightStartedAt != nil {
		t.Fatalf("current state was not cleared: %+v", pkg.Submission)
	}
	if record.Status != sheinpub.SubmissionStatusSuccess {
		t.Fatalf("status = %q, want %q", record.Status, sheinpub.SubmissionStatusSuccess)
	}
	if record.RequestID != "idem-1" || !record.StartedAt.Equal(startedAt) || record.FinishedAt == nil || !record.FinishedAt.Equal(finishedAt) {
		t.Fatalf("record timing/request = %+v, want request id and start/finish times", record)
	}
	if !record.SubmittedAt.Equal(startedAt) {
		t.Fatalf("submitted_at = %v, want original started_at %v", record.SubmittedAt, startedAt)
	}
	if pkg.Submission.SubmittedAt == nil || !pkg.Submission.SubmittedAt.Equal(startedAt) {
		t.Fatalf("submission submitted_at = %v, want original started_at %v", pkg.Submission.SubmittedAt, startedAt)
	}
}

func TestFailSheinSubmitAttemptRecordsFailedPhaseAndError(t *testing.T) {
	startedAt := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)
	finishedAt := startedAt.Add(time.Second)
	pkg := &SheinPackage{}
	beginSheinSubmitAttempt(pkg, "publish", "idem-1", sheinpub.SubmissionPhasePrepareProduct, startedAt)

	record := failSheinSubmitAttempt(pkg, "publish", "idem-1", sheinpub.SubmissionPhasePrepareProduct, errTestSubmitState, finishedAt)

	if record.Status != sheinpub.SubmissionStatusFailed {
		t.Fatalf("status = %q, want %q", record.Status, sheinpub.SubmissionStatusFailed)
	}
	if record.Phase != sheinpub.SubmissionPhasePrepareProduct {
		t.Fatalf("phase = %q, want %q", record.Phase, sheinpub.SubmissionPhasePrepareProduct)
	}
	if record.Error != errTestSubmitState.Error() {
		t.Fatalf("error = %q, want %q", record.Error, errTestSubmitState.Error())
	}
	if !record.SubmittedAt.Equal(startedAt) {
		t.Fatalf("submitted_at = %v, want original started_at %v", record.SubmittedAt, startedAt)
	}
	if pkg.Submission.LastError != errTestSubmitState.Error() {
		t.Fatalf("last error = %q, want %q", pkg.Submission.LastError, errTestSubmitState.Error())
	}
}

func TestFailSheinSubmitAttemptWithResponseAndBuildEventPreservesResponse(t *testing.T) {
	startedAt := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)
	finishedAt := startedAt.Add(time.Second)
	pkg := &SheinPackage{}
	beginSheinSubmitAttempt(pkg, "publish", "idem-1", sheinpub.SubmissionPhaseConfirmRemote, startedAt)
	response := &sheinpub.SubmissionResponse{Success: true, Message: "remote payload"}

	record, event := failSheinSubmitAttemptWithResponseAndBuildEvent(pkg, "task-1", "publish", "idem-1", sheinpub.SubmissionPhaseConfirmRemote, response, errTestSubmitState, finishedAt)

	if record == nil {
		t.Fatal("expected failed record")
	}
	if event.Response != response {
		t.Fatalf("event response = %+v, want original response", event.Response)
	}
	if event.Status != sheinpub.SubmissionStatusFailed || event.Phase != sheinpub.SubmissionPhaseConfirmRemote {
		t.Fatalf("event = %+v, want failed confirm_remote event", event)
	}
}

func TestFindSheinSubmissionRecordByRequestIDReturnsCompletedRecord(t *testing.T) {
	startedAt := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)
	finishedAt := startedAt.Add(time.Second)
	pkg := &SheinPackage{}
	beginSheinSubmitAttempt(pkg, "save_draft", "idem-1", sheinpub.SubmissionPhaseValidate, startedAt)
	completeSheinSubmitAttempt(pkg, "save_draft", "idem-1", &sheinpub.SubmissionResponse{Code: "0"}, nil, finishedAt)

	record := findSheinSubmissionRecordByRequestID(pkg, "save_draft", "idem-1")

	if record == nil {
		t.Fatal("expected replay record")
	}
	if record.Action != "save_draft" || record.RequestID != "idem-1" {
		t.Fatalf("record = %+v, want save_draft idem-1", record)
	}
}

func TestFindActiveSheinSubmitAttemptHonorsLeaseExpiry(t *testing.T) {
	startedAt := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	now := startedAt.Add(sheinSubmitInFlightTTL + time.Minute)
	pkg := &SheinPackage{}
	beginSheinSubmitAttempt(pkg, "publish", "idem-1", sheinpub.SubmissionPhaseSubmitRemote, startedAt)

	if active := findActiveSheinSubmitAttempt(pkg, "publish", now); active != nil {
		t.Fatalf("active = %+v, want nil after lease expiry", active)
	}
	if !sheinSubmitAttemptNeedsRemoteRecovery(pkg.Submission, "publish", now) {
		t.Fatal("expected remote recovery after lease expiry")
	}
}

var errTestSubmitState = submitStateTestError("submit state test error")

type submitStateTestError string

func (e submitStateTestError) Error() string {
	return string(e)
}

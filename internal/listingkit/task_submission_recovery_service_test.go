package listingkit

import (
	"context"
	"errors"
	"testing"
	"time"

	"task-processor/internal/listingkit/submission"
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
	appendSheinSubmissionEvent(task.Result.Shein, submission.BuildEvent(task.ID, "publish", record, record.Result, nil, record.StartedAt))
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

func TestTaskSubmissionRecoveryServiceBeginSheinSubmitLeaseReturnsRecoverRemoteWhenSupplierCodeExists(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	startedAt := time.Now().Add(-sheinSubmitInFlightTTL - time.Minute)
	beginSheinSubmitAttempt(task.Result.Shein, "publish", "recover-remote-123", sheinpub.SubmissionPhasePrepareProduct, startedAt)
	task.Result.Shein.Submission.Publish = &sheinpub.SubmissionRecord{
		Action:       "publish",
		RequestID:    "recover-remote-123",
		Status:       sheinpub.SubmissionStatusRunning,
		Phase:        sheinpub.SubmissionPhasePrepareProduct,
		SupplierCode: "SUP-submit-task-1",
		StartedAt:    startedAt,
	}
	task.Result.Shein.Submission.CurrentRequestID = "recover-remote-123"
	task.Result.Shein.Submission.CurrentPhase = sheinpub.SubmissionPhasePrepareProduct
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	recovery := newTaskSubmissionRecoveryService(taskSubmissionRecoveryServiceConfig{
		repo: repo,
	})

	got, err := recovery.beginSheinSubmitLease(context.Background(), task.ID, "publish", "recover-remote-123", time.Now())
	if !errors.Is(err, errSheinSubmitRecoverRemote) {
		t.Fatalf("beginSheinSubmitLease() err = %v, want %v", err, errSheinSubmitRecoverRemote)
	}
	if got == nil || got.ID != task.ID {
		t.Fatalf("task = %+v, want original task", got)
	}
}

func TestTaskSubmissionRecoveryServiceRefreshSheinSubmitRemoteStatusHandlesMissingSupplierCode(t *testing.T) {
	t.Parallel()

	recovery := newTaskSubmissionRecoveryService(taskSubmissionRecoveryServiceConfig{})
	pkg := sheinpub.NormalizePackageSemanticFields(makeReadySheinTask().Result.Shein)
	if pkg == nil {
		t.Fatal("expected shein package")
	}
	requestID := "refresh-no-supplier-123"
	startedAt := time.Now().Add(-time.Minute)
	pkg.SubmissionState = &sheinpub.SubmissionReport{
		LastAction: "publish",
		LastStatus: sheinpub.SubmissionStatusSuccess,
		LastResult: &sheinpub.SubmissionResponse{
			Code:    "0",
			Message: "success",
			Success: true,
			SPUName: "SPU-123",
		},
		Publish: &sheinpub.SubmissionRecord{
			Action:    "publish",
			RequestID: requestID,
			Status:    sheinpub.SubmissionStatusSuccess,
			StartedAt: startedAt,
			Result: &sheinpub.SubmissionResponse{
				Code:    "0",
				Message: "success",
				Success: true,
				SPUName: "SPU-123",
			},
		},
	}
	pkg.PreviewPayload.SupplierCode = ""
	for i := range pkg.PreviewPayload.SKCList {
		pkg.PreviewPayload.SKCList[i].SupplierCode = nil
		for j := range pkg.PreviewPayload.SKCList[i].SKUS {
			pkg.PreviewPayload.SKCList[i].SKUS[j].SupplierSKU = ""
		}
	}

	event, err := recovery.refreshSheinSubmitRemoteStatus(
		context.Background(),
		nil,
		"task-refresh-no-supplier",
		pkg,
		stubSheinProductAPI{},
		"publish",
		requestID,
		"",
		startedAt,
	)
	if err != nil {
		t.Fatalf("refreshSheinSubmitRemoteStatus() err = %v", err)
	}
	if event == nil {
		t.Fatal("event = nil, want confirm remote event")
	}
	if event.Status != sheinpub.SubmissionRemoteStatusConfirmed {
		t.Fatalf("event status = %q, want %q", event.Status, sheinpub.SubmissionRemoteStatusConfirmed)
	}
	record := sheinSubmissionRecordForAction(pkg.SubmissionState, "publish")
	if record == nil {
		t.Fatalf("publish record = nil, want remote confirmation state")
	}
	if pkg.SubmissionState == nil || pkg.SubmissionState.RemoteStatus != sheinpub.SubmissionRemoteStatusConfirmed {
		t.Fatalf("submission remote status = %+v, want %q", pkg.SubmissionState, sheinpub.SubmissionRemoteStatusConfirmed)
	}
	if record.RemoteRecordID != "" {
		t.Fatalf("remote record id = %q, want empty without supplier code", record.RemoteRecordID)
	}
	if record.RemoteMessage == "" {
		t.Fatal("remote message = empty, want fallback detail")
	}
}

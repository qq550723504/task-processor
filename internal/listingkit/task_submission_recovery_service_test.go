package listingkit

import (
	"context"
	"errors"
	"testing"
	"time"

	"task-processor/internal/listingkit/submission"
	sheinpub "task-processor/internal/publishing/shein"
	sheinother "task-processor/internal/shein/api/other"
	sheinproduct "task-processor/internal/shein/api/product"
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

func TestTaskSubmissionRecoveryServiceBeginSheinSubmitLeaseReturnsSubmitInProgressForDifferentRequest(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	startedAt := time.Now().Add(-time.Minute)
	beginSheinSubmitAttempt(task.Result.Shein, "publish", "in-flight-123", sheinpub.SubmissionPhaseSubmitRemote, startedAt)
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	recovery := newTaskSubmissionRecoveryService(taskSubmissionRecoveryServiceConfig{
		repo: repo,
	})

	got, err := recovery.beginSheinSubmitLease(context.Background(), task.ID, "publish", "different-123", time.Now())
	var inProgress *submission.SubmitInProgressError
	if !errors.As(err, &inProgress) {
		t.Fatalf("beginSheinSubmitLease() err = %v, want SubmitInProgressError", err)
	}
	if inProgress.RequestID != "in-flight-123" {
		t.Fatalf("in-progress request id = %q, want in-flight-123", inProgress.RequestID)
	}
	if inProgress.Phase != sheinpub.SubmissionPhaseSubmitRemote {
		t.Fatalf("in-progress phase = %q, want %q", inProgress.Phase, sheinpub.SubmissionPhaseSubmitRemote)
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

func TestTaskSubmissionRecoveryServiceRefreshSheinSubmitRemoteStatusAppliesResolvedConfirmation(t *testing.T) {
	t.Parallel()

	requestID := "refresh-confirmed-123"
	startedAt := time.Now().Add(-time.Minute)
	checkedAt := time.Now()
	pkg := sheinpub.NormalizePackageSemanticFields(makeReadySheinTask().Result.Shein)
	if pkg == nil {
		t.Fatal("expected shein package")
	}
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
			Action:       "publish",
			RequestID:    requestID,
			Status:       sheinpub.SubmissionStatusSuccess,
			SupplierCode: "SKC-1",
			StartedAt:    startedAt,
			Result: &sheinpub.SubmissionResponse{
				Code:    "0",
				Message: "success",
				Success: true,
				SPUName: "SPU-123",
			},
		},
	}
	var gotLookupCodes []string
	var gotDefaultConfirmed bool
	var gotSPUName string
	recovery := newTaskSubmissionRecoveryService(taskSubmissionRecoveryServiceConfig{
		resolveRemoteStatusCallback: func(productAPI sheinproduct.ProductAPI, otherAPI sheinother.OtherAPI, action, requestID string, lookupCodes []string, spuName string, defaultConfirmed bool, fallbackMessage string, startedAt time.Time, taskID string) (*sheinRemoteConfirmation, error) {
			gotLookupCodes = append([]string(nil), lookupCodes...)
			gotDefaultConfirmed = defaultConfirmed
			gotSPUName = spuName
			event := submission.BuildConfirmRemoteEvent(taskID, action, sheinpub.SubmissionRemoteStatusConfirmed, requestID, startedAt, "resolved by callback", nil)
			return &sheinRemoteConfirmation{
				remoteStatus: sheinpub.SubmissionRemoteStatusConfirmed,
				record: &sheinproduct.RecordItem{
					RecordID:     "record-123",
					SupplierCode: "SKC-1",
					State:        4,
					AuditState:   5,
				},
				checkedAt: checkedAt,
				message:   "resolved by callback",
				event:     &event,
			}, nil
		},
	})

	event, err := recovery.refreshSheinSubmitRemoteStatus(
		context.Background(),
		nil,
		"task-refresh-confirmed",
		pkg,
		stubSheinProductAPI{},
		"publish",
		requestID,
		"SKC-1",
		startedAt,
	)
	if err != nil {
		t.Fatalf("refreshSheinSubmitRemoteStatus() err = %v", err)
	}
	if event == nil || event.Status != sheinpub.SubmissionRemoteStatusConfirmed {
		t.Fatalf("event = %+v, want confirmed remote event", event)
	}
	if !gotDefaultConfirmed {
		t.Fatal("defaultConfirmed = false, want true for accepted publish")
	}
	if gotSPUName != "SPU-123" {
		t.Fatalf("spu name = %q, want SPU-123", gotSPUName)
	}
	if len(gotLookupCodes) == 0 || gotLookupCodes[0] != "SKC-1" {
		t.Fatalf("lookup codes = %+v, want supplier code first", gotLookupCodes)
	}
	record := sheinSubmissionRecordForAction(pkg.SubmissionState, "publish")
	if record == nil {
		t.Fatal("publish record = nil, want remote confirmation state")
	}
	if record.RemoteRecordID != "record-123" {
		t.Fatalf("remote record id = %q, want record-123", record.RemoteRecordID)
	}
	if record.RemoteState != 4 || record.RemoteAuditState != 5 {
		t.Fatalf("remote state = (%d,%d), want (4,5)", record.RemoteState, record.RemoteAuditState)
	}
	if record.RemoteCheckedAt == nil || !record.RemoteCheckedAt.Equal(checkedAt) {
		t.Fatalf("remote checked at = %+v, want %v", record.RemoteCheckedAt, checkedAt)
	}
	if pkg.SubmissionState.RemoteStatus != sheinpub.SubmissionRemoteStatusConfirmed {
		t.Fatalf("submission remote status = %q, want confirmed", pkg.SubmissionState.RemoteStatus)
	}
}

func TestTaskSubmissionRecoveryServiceClearSheinSubmitLeaseAfterStartFailureMarksFailedAttempt(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	startedAt := time.Now().Add(-time.Minute)
	beginSheinSubmitAttempt(task.Result.Shein, "publish", "start-fail-123", sheinpub.SubmissionPhaseSubmitRemote, startedAt)
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	recovery := newTaskSubmissionRecoveryService(taskSubmissionRecoveryServiceConfig{
		repo: repo,
	})

	startErr := errors.New("workflow start failed")
	if err := recovery.clearSheinSubmitLeaseAfterStartFailure(context.Background(), task.ID, "publish", "start-fail-123", startErr); err != nil {
		t.Fatalf("clearSheinSubmitLeaseAfterStartFailure() err = %v", err)
	}

	saved, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	record := saved.Result.Shein.Submission.Publish
	if record == nil {
		t.Fatal("publish record = nil, want failed attempt")
	}
	if record.Status != sheinpub.SubmissionStatusFailed {
		t.Fatalf("publish status = %q, want failed", record.Status)
	}
	if record.Phase != sheinpub.SubmissionPhaseValidate {
		t.Fatalf("publish phase = %q, want validate", record.Phase)
	}
	if record.FinishedAt == nil {
		t.Fatalf("publish record = %+v, want finished_at", record)
	}
	if record.Error != startErr.Error() {
		t.Fatalf("publish error = %q, want %q", record.Error, startErr.Error())
	}
	if saved.Result.Shein.Submission.CurrentAction != "" || saved.Result.Shein.Submission.CurrentPhase != "" || saved.Result.Shein.Submission.CurrentRequestID != "" {
		t.Fatalf("submission current state = %+v, want cleared lease", saved.Result.Shein.Submission)
	}
}

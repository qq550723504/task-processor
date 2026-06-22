package listingkit

import (
	"context"
	"errors"
	"testing"
	"time"

	listingsubmission "task-processor/internal/listing/submission"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func TestTaskSubmissionRecoveryServiceBeginSheinSubmitLeaseReplaysExistingRequest(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	now := time.Now().Add(-time.Minute)
	record := sheinpub.CompleteSubmitAttemptAt(task.Result.Shein, "publish", "replay-123", &sheinpub.SubmissionResponse{
		Code:    "0",
		Message: "success",
		Success: true,
		SPUName: "SPU-123",
	}, nil, now)
	sheinpub.AppendSubmissionEvent(task.Result.Shein, sheinpub.BuildSubmissionAttemptEvent(task.ID, "publish", record, record.Result, nil, record.StartedAt))
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
	var inProgress *listingsubmission.SubmitInProgressError
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

func TestTaskSubmissionRecoveryServiceBeginSheinSubmitLeaseStartsNewLease(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	startedAt := time.Now().Add(-time.Minute)
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	recovery := newTaskSubmissionRecoveryService(taskSubmissionRecoveryServiceConfig{
		repo: repo,
	})

	got, err := recovery.beginSheinSubmitLease(context.Background(), task.ID, "publish", "new-lease-123", startedAt)
	if err != nil {
		t.Fatalf("beginSheinSubmitLease() err = %v", err)
	}
	if got == nil || got.ID != task.ID {
		t.Fatalf("task = %+v, want original task", got)
	}
	if got.Result == nil || got.Result.Shein == nil || got.Result.Shein.Submission == nil {
		t.Fatalf("submission = %+v, want initialized lease state", got.Result)
	}
	if got.Result.Shein.Submission.CurrentAction != "publish" {
		t.Fatalf("current action = %q, want publish", got.Result.Shein.Submission.CurrentAction)
	}
	if got.Result.Shein.Submission.CurrentRequestID != "new-lease-123" {
		t.Fatalf("current request id = %q, want new-lease-123", got.Result.Shein.Submission.CurrentRequestID)
	}
	if got.Result.Shein.Submission.CurrentPhase != sheinpub.SubmissionPhaseValidate {
		t.Fatalf("current phase = %q, want %q", got.Result.Shein.Submission.CurrentPhase, sheinpub.SubmissionPhaseValidate)
	}
	if len(got.Result.Shein.SubmissionEvents) == 0 {
		t.Fatal("expected lease-start event to be appended")
	}
	if got.Result.Shein.SubmissionEvents[0].Status != sheinpub.SubmissionStatusRunning || got.Result.Shein.SubmissionEvents[0].Phase != sheinpub.SubmissionPhaseValidate {
		t.Fatalf("lease event = %+v, want running validate event", got.Result.Shein.SubmissionEvents[0])
	}
}

func TestTaskSubmissionRecoveryServiceAcquireSheinSubmitTaskBuildsReplayPreview(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	now := time.Now().Add(-time.Minute)
	record := sheinpub.CompleteSubmitAttemptAt(task.Result.Shein, "publish", "replay-preview-123", &sheinpub.SubmissionResponse{
		Code:    "0",
		Message: "success",
		Success: true,
		SPUName: "SPU-123",
	}, nil, now)
	sheinpub.AppendSubmissionEvent(task.Result.Shein, sheinpub.BuildSubmissionAttemptEvent(task.ID, "publish", record, record.Result, nil, record.StartedAt))
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	expectedPreview := &ListingKitPreview{TaskID: task.ID}
	recovery := newTaskSubmissionRecoveryService(taskSubmissionRecoveryServiceConfig{
		repo: repo,
		buildTaskPreview: func(_ context.Context, gotTask *Task, platform string) (*ListingKitPreview, error) {
			if gotTask == nil || gotTask.ID != task.ID {
				t.Fatalf("preview task = %+v, want original task", gotTask)
			}
			if platform != "shein" {
				t.Fatalf("platform = %q, want shein", platform)
			}
			return expectedPreview, nil
		},
	})

	gotTask, preview, err := recovery.acquireSheinSubmitTask(context.Background(), task.ID, "publish", "replay-preview-123", time.Now())
	if err != nil {
		t.Fatalf("acquireSheinSubmitTask() err = %v", err)
	}
	if gotTask != nil {
		t.Fatalf("task = %+v, want nil when replay preview is returned", gotTask)
	}
	if preview != expectedPreview {
		t.Fatalf("preview = %+v, want %+v", preview, expectedPreview)
	}
}

func TestTaskSubmissionRecoveryServiceAcquireSheinSubmitTaskRecoversRemotePreview(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	startedAt := time.Now().Add(-3 * time.Minute)
	beginSheinSubmitAttempt(task.Result.Shein, "publish", "recover-preview-123", sheinpub.SubmissionPhaseSubmitRemote, startedAt)
	response := &sheinpub.SubmissionResponse{
		Code:    "0",
		Message: "accepted",
		Success: false,
		SPUName: "SPU-recover",
	}
	sheinpub.SetSubmissionRemoteResponse(task.Result.Shein, "publish", "recover-preview-123", "SUP-recover", response)
	task.Result.Shein.Submission.Publish.SupplierCode = "SUP-recover"
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	expectedPreview := &ListingKitPreview{TaskID: task.ID}
	recovery := newTaskSubmissionRecoveryService(taskSubmissionRecoveryServiceConfig{
		repo: repo,
		buildSheinSubmitProductAPI: func(_ context.Context, gotTask *Task) (sheinproduct.ProductAPI, error) {
			if gotTask == nil || gotTask.ID != task.ID {
				t.Fatalf("build api task = %+v, want original task", gotTask)
			}
			return stubSheinProductAPI{}, nil
		},
		resolveRemoteStatusCallback: func(request *sheinRemoteStatusRequest) (*sheinRemoteConfirmation, error) {
			event := sheinpub.BuildSubmissionConfirmRemoteEvent(task.ID, "publish", sheinpub.SubmissionRemoteStatusConfirmed, request.requestID, request.startedAt, "remote confirmed", nil)
			return &sheinRemoteConfirmation{
				RemoteStatus: sheinpub.SubmissionRemoteStatusConfirmed,
				Record: &sheinproduct.RecordItem{
					RecordID:     "record-recover-preview",
					SupplierCode: "SUP-recover",
				},
				CheckedAt: time.Now(),
				Message:   "remote confirmed",
				Event:     &event,
			}, nil
		},
		rememberSheinSubmitted: func(gotTask *Task, action string) {
			if gotTask == nil || gotTask.ID != task.ID || action != "publish" {
				t.Fatalf("remember args = %+v/%q, want task %q/publish", gotTask, action, task.ID)
			}
		},
		persistSuccessfulSubmission: func(_ context.Context, taskID string, gotTask *Task, action string) error {
			if taskID != task.ID || gotTask == nil || gotTask.ID != task.ID || action != "publish" {
				t.Fatalf("persist args = %q/%+v/%q, want task %q/publish", taskID, gotTask, action, task.ID)
			}
			return nil
		},
		buildTaskPreview: func(_ context.Context, gotTask *Task, platform string) (*ListingKitPreview, error) {
			if gotTask == nil || gotTask.ID != task.ID || platform != "shein" {
				t.Fatalf("preview args = %+v/%q, want task %q/shein", gotTask, platform, task.ID)
			}
			return expectedPreview, nil
		},
	})

	gotTask, preview, err := recovery.acquireSheinSubmitTask(context.Background(), task.ID, "publish", "recover-preview-123", time.Now())
	if err != nil {
		t.Fatalf("acquireSheinSubmitTask() err = %v", err)
	}
	if gotTask != nil {
		t.Fatalf("task = %+v, want nil when recovery preview is returned", gotTask)
	}
	if preview != expectedPreview {
		t.Fatalf("preview = %+v, want %+v", preview, expectedPreview)
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
		&sheinRemoteRefreshRequest{
			taskID:       "task-refresh-no-supplier",
			pkg:          pkg,
			productAPI:   stubSheinProductAPI{},
			action:       "publish",
			requestID:    requestID,
			supplierCode: "",
			startedAt:    startedAt,
		},
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
	record := sheinpub.SubmissionRecordForAction(pkg.SubmissionState, "publish")
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
		resolveRemoteStatusCallback: func(request *sheinRemoteStatusRequest) (*sheinRemoteConfirmation, error) {
			gotLookupCodes = append([]string(nil), request.lookupCodes...)
			gotDefaultConfirmed = request.defaultConfirmed
			gotSPUName = request.spuName
			event := sheinpub.BuildSubmissionConfirmRemoteEvent(request.taskID, request.action, sheinpub.SubmissionRemoteStatusConfirmed, request.requestID, request.startedAt, "resolved by callback", nil)
			return &sheinRemoteConfirmation{
				RemoteStatus: sheinpub.SubmissionRemoteStatusConfirmed,
				Record: &sheinproduct.RecordItem{
					RecordID:     "record-123",
					SupplierCode: "SKC-1",
					State:        4,
					AuditState:   5,
				},
				CheckedAt: checkedAt,
				Message:   "resolved by callback",
				Event:     &event,
			}, nil
		},
	})

	event, err := recovery.refreshSheinSubmitRemoteStatus(
		context.Background(),
		&sheinRemoteRefreshRequest{
			taskID:       "task-refresh-confirmed",
			pkg:          pkg,
			productAPI:   stubSheinProductAPI{},
			action:       "publish",
			requestID:    requestID,
			supplierCode: "SKC-1",
			startedAt:    startedAt,
		},
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
	record := sheinpub.SubmissionRecordForAction(pkg.SubmissionState, "publish")
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

func TestTaskSubmissionRecoveryServiceHandleSheinWorkflowStartFailureReturnsOriginalErrorAfterPersistAndClear(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	startedAt := time.Now().Add(-time.Minute)
	beginSheinSubmitAttempt(task.Result.Shein, "publish", "workflow-start-fail-123", sheinpub.SubmissionPhaseValidate, startedAt)
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	var failureCalls int
	recovery := newTaskSubmissionRecoveryService(taskSubmissionRecoveryServiceConfig{
		repo: repo,
		recordSubmissionFailure: func(_ context.Context, taskID string, result *ListingKitResult, pkg *SheinPackage, action, requestID, phase string, submitErr error) error {
			failureCalls++
			if taskID != task.ID {
				t.Fatalf("taskID = %q, want %q", taskID, task.ID)
			}
			if result != task.Result {
				t.Fatalf("result = %+v, want original task result", result)
			}
			if pkg != task.Result.Shein {
				t.Fatalf("pkg = %+v, want original shein package", pkg)
			}
			if action != "publish" || requestID != "workflow-start-fail-123" {
				t.Fatalf("action/requestID = %q/%q, want publish/workflow-start-fail-123", action, requestID)
			}
			if phase != sheinpub.SubmissionPhaseValidate {
				t.Fatalf("phase = %q, want %q", phase, sheinpub.SubmissionPhaseValidate)
			}
			if submitErr == nil || submitErr.Error() != "workflow start failed" {
				t.Fatalf("submitErr = %v, want workflow start failed", submitErr)
			}
			return nil
		},
	})

	startErr := errors.New("workflow start failed")
	err := recovery.handleSheinWorkflowStartFailure(context.Background(), task.ID, task, sheinWorkflowSubmitOptions{
		action:    "publish",
		requestID: "workflow-start-fail-123",
		startedAt: startedAt,
	}, startErr)
	if !errors.Is(err, startErr) {
		t.Fatalf("handleSheinWorkflowStartFailure() err = %v, want %v", err, startErr)
	}
	if failureCalls != 1 {
		t.Fatalf("record submission failure calls = %d, want 1", failureCalls)
	}

	saved, getErr := repo.GetTask(context.Background(), task.ID)
	if getErr != nil {
		t.Fatalf("get task: %v", getErr)
	}
	record := saved.Result.Shein.Submission.Publish
	if record == nil {
		t.Fatal("publish record = nil, want failed attempt")
	}
	if record.Status != sheinpub.SubmissionStatusFailed {
		t.Fatalf("publish status = %q, want failed", record.Status)
	}
	if saved.Result.Shein.Submission.CurrentAction != "" || saved.Result.Shein.Submission.CurrentPhase != "" || saved.Result.Shein.Submission.CurrentRequestID != "" {
		t.Fatalf("submission current state = %+v, want cleared lease", saved.Result.Shein.Submission)
	}
}

func TestTaskSubmissionRecoveryServiceClearSheinSubmitLeaseClearsInFlightState(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	startedAt := time.Now().Add(-time.Minute)
	beginSheinSubmitAttempt(task.Result.Shein, "publish", "clear-lease-123", sheinpub.SubmissionPhaseSubmitRemote, startedAt)
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	recovery := newTaskSubmissionRecoveryService(taskSubmissionRecoveryServiceConfig{
		repo: repo,
	})

	if err := recovery.clearSheinSubmitLease(context.Background(), task.ID, "publish", "clear-lease-123"); err != nil {
		t.Fatalf("clearSheinSubmitLease() err = %v", err)
	}

	saved, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if saved.Result.Shein.Submission.CurrentAction != "" || saved.Result.Shein.Submission.CurrentPhase != "" || saved.Result.Shein.Submission.CurrentRequestID != "" {
		t.Fatalf("submission current state = %+v, want cleared lease", saved.Result.Shein.Submission)
	}
	if saved.Result.Shein.Submission.Publish == nil || saved.Result.Shein.Submission.Publish.Status != sheinpub.SubmissionStatusRunning {
		t.Fatalf("publish record = %+v, want unchanged running record", saved.Result.Shein.Submission.Publish)
	}
}

func TestTaskSubmissionRecoveryServiceRecoverSheinSubmitLocallyFinalizesRecoveredSubmission(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	startedAt := time.Now().Add(-2 * time.Minute)
	beginSheinSubmitAttempt(task.Result.Shein, "publish", "recover-local-123", sheinpub.SubmissionPhasePersistResult, startedAt)
	response := &sheinpub.SubmissionResponse{
		Code:    "0",
		Message: "success",
		Success: true,
		SPUName: "SPU-local",
	}
	sheinpub.SetSubmissionRemoteResponse(task.Result.Shein, "publish", "recover-local-123", "SUP-local", response)
	state, err := buildRecoveredSheinRemoteState(task, "publish")
	if err != nil {
		t.Fatalf("buildRecoveredSheinRemoteState() err = %v", err)
	}
	expectedPreview := &ListingKitPreview{TaskID: task.ID}
	var calls []string

	recovery := newTaskSubmissionRecoveryService(taskSubmissionRecoveryServiceConfig{
		rememberSheinSubmitted: func(gotTask *Task, action string) {
			calls = append(calls, "remember")
			if gotTask != task {
				t.Fatalf("remember task = %+v, want original task", gotTask)
			}
			if action != "publish" {
				t.Fatalf("remember action = %q, want publish", action)
			}
		},
		persistSuccessfulSubmission: func(_ context.Context, taskID string, gotTask *Task, action string) error {
			calls = append(calls, "persist")
			if taskID != task.ID {
				t.Fatalf("persist taskID = %q, want %q", taskID, task.ID)
			}
			if gotTask != task {
				t.Fatalf("persist task = %+v, want original task", gotTask)
			}
			if action != "publish" {
				t.Fatalf("persist action = %q, want publish", action)
			}
			if gotTask.Result.Shein.Submission.LastStatus != sheinpub.SubmissionStatusSuccess {
				t.Fatalf("persist submission = %+v, want success state", gotTask.Result.Shein.Submission)
			}
			return nil
		},
		buildTaskPreview: func(_ context.Context, gotTask *Task, platform string) (*ListingKitPreview, error) {
			calls = append(calls, "preview")
			if gotTask != task {
				t.Fatalf("preview task = %+v, want original task", gotTask)
			}
			if platform != "shein" {
				t.Fatalf("platform = %q, want shein", platform)
			}
			return expectedPreview, nil
		},
	})

	preview, err := recovery.recoverSheinSubmitLocally(context.Background(), state)
	if err != nil {
		t.Fatalf("recoverSheinSubmitLocally() err = %v", err)
	}
	if preview != expectedPreview {
		t.Fatalf("preview = %+v, want %+v", preview, expectedPreview)
	}
	wantCalls := []string{"remember", "persist", "preview"}
	if len(calls) != len(wantCalls) {
		t.Fatalf("calls = %+v, want %+v", calls, wantCalls)
	}
	for i := range wantCalls {
		if calls[i] != wantCalls[i] {
			t.Fatalf("calls[%d] = %q, want %q; full calls = %+v", i, calls[i], wantCalls[i], calls)
		}
	}
	if task.Result.Shein.Submission.CurrentAction != "" || task.Result.Shein.Submission.CurrentPhase != "" || task.Result.Shein.Submission.CurrentRequestID != "" {
		t.Fatalf("submission current state = %+v, want cleared in-flight fields", task.Result.Shein.Submission)
	}
	if len(task.Result.Shein.SubmissionEvents) < 2 {
		t.Fatalf("submission events = %+v, want local persist and completion events", task.Result.Shein.SubmissionEvents)
	}
	if task.Result.Shein.SubmissionEvents[0].Status != sheinpub.SubmissionStatusSuccess || task.Result.Shein.SubmissionEvents[0].Phase != sheinpub.SubmissionPhasePersistResult {
		t.Fatalf("completion event = %+v, want successful persist_result completion", task.Result.Shein.SubmissionEvents[0])
	}
	if task.Result.Shein.SubmissionEvents[1].Status != sheinpub.SubmissionStatusRunning || task.Result.Shein.SubmissionEvents[1].Phase != sheinpub.SubmissionPhasePersistResult {
		t.Fatalf("persist event = %+v, want running persist_result event", task.Result.Shein.SubmissionEvents[1])
	}
}

func TestBuildRecoveredSheinRemoteStateCopiesSupplierCodeIntoRecoverySnapshot(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	startedAt := time.Now().Add(-2 * time.Minute)
	beginSheinSubmitAttempt(task.Result.Shein, "publish", "recover-snapshot-123", sheinpub.SubmissionPhaseSubmitRemote, startedAt)
	task.Result.Shein.Submission.Publish.SupplierCode = "SUP-snapshot"

	state, err := buildRecoveredSheinRemoteState(task, "publish")
	if err != nil {
		t.Fatalf("buildRecoveredSheinRemoteState() err = %v", err)
	}
	if state == nil {
		t.Fatal("state = nil, want recovered remote state")
	}
	if state.selection.SupplierCode != "SUP-snapshot" {
		t.Fatalf("state.selection.SupplierCode = %q, want SUP-snapshot", state.selection.SupplierCode)
	}

	refreshState := buildRecoveredSheinRemoteRefreshState(state)
	if refreshState == nil {
		t.Fatal("refresh state = nil, want remote refresh execution state")
	}
	if refreshState.supplierCode != "SUP-snapshot" {
		t.Fatalf("refresh state supplier code = %q, want SUP-snapshot", refreshState.supplierCode)
	}
}

func TestTaskSubmissionRecoveryServiceRecoverSheinSubmitViaRemoteConfirmationFinalizesOnSuccess(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	startedAt := time.Now().Add(-3 * time.Minute)
	beginSheinSubmitAttempt(task.Result.Shein, "publish", "recover-remote-ok-123", sheinpub.SubmissionPhaseSubmitRemote, startedAt)
	response := &sheinpub.SubmissionResponse{
		Code:    "0",
		Message: "success",
		Success: true,
		SPUName: "SPU-remote",
	}
	sheinpub.SetSubmissionRemoteResponse(task.Result.Shein, "publish", "recover-remote-ok-123", "SUP-remote", response)
	task.Result.Shein.Submission.Publish.SupplierCode = "SUP-remote"
	state, err := buildRecoveredSheinRemoteState(task, "publish")
	if err != nil {
		t.Fatalf("buildRecoveredSheinRemoteState() err = %v", err)
	}
	expectedPreview := &ListingKitPreview{TaskID: task.ID}
	productAPI := stubSheinProductAPI{}
	checkedAt := time.Now()
	var calls []string

	recovery := newTaskSubmissionRecoveryService(taskSubmissionRecoveryServiceConfig{
		buildSheinSubmitProductAPI: func(_ context.Context, gotTask *Task) (sheinproduct.ProductAPI, error) {
			calls = append(calls, "build_api")
			if gotTask != task {
				t.Fatalf("build api task = %+v, want original task", gotTask)
			}
			return productAPI, nil
		},
		resolveRemoteStatusCallback: func(request *sheinRemoteStatusRequest) (*sheinRemoteConfirmation, error) {
			calls = append(calls, "resolve_remote")
			if request.productAPI == nil {
				t.Fatal("product api = nil, want built api")
			}
			if request.otherAPI != nil {
				t.Fatalf("other api = %+v, want nil", request.otherAPI)
			}
			if request.action != "publish" || request.requestID != "recover-remote-ok-123" {
				t.Fatalf("resolve args = %q/%q, want publish/recover-remote-ok-123", request.action, request.requestID)
			}
			if len(request.lookupCodes) == 0 || request.lookupCodes[0] != "SUP-remote" {
				t.Fatalf("lookup codes = %+v, want supplier code first", request.lookupCodes)
			}
			if request.spuName != "SPU-remote" {
				t.Fatalf("spu name = %q, want SPU-remote", request.spuName)
			}
			if !request.defaultConfirmed {
				t.Fatal("defaultConfirmed = false, want true")
			}
			if request.startedAt.IsZero() || request.taskID != task.ID {
				t.Fatalf("resolve start/task = %v/%q, want non-zero/%q", request.startedAt, request.taskID, task.ID)
			}
			event := sheinpub.BuildSubmissionConfirmRemoteEvent(task.ID, "publish", sheinpub.SubmissionRemoteStatusConfirmed, "recover-remote-ok-123", request.startedAt, "remote confirmed", nil)
			return &sheinRemoteConfirmation{
				RemoteStatus: sheinpub.SubmissionRemoteStatusConfirmed,
				Record: &sheinproduct.RecordItem{
					RecordID:     "record-remote-ok",
					SupplierCode: "SUP-remote",
					State:        6,
					AuditState:   7,
				},
				CheckedAt: checkedAt,
				Message:   "remote confirmed",
				Event:     &event,
			}, nil
		},
		rememberSheinSubmitted: func(gotTask *Task, action string) {
			calls = append(calls, "remember")
			if gotTask != task || action != "publish" {
				t.Fatalf("remember args = %+v/%q, want original task/publish", gotTask, action)
			}
		},
		persistSuccessfulSubmission: func(_ context.Context, taskID string, gotTask *Task, action string) error {
			calls = append(calls, "persist")
			if taskID != task.ID || gotTask != task || action != "publish" {
				t.Fatalf("persist args = %q/%+v/%q, want %q/original/publish", taskID, gotTask, action, task.ID)
			}
			return nil
		},
		buildTaskPreview: func(_ context.Context, gotTask *Task, platform string) (*ListingKitPreview, error) {
			calls = append(calls, "preview")
			if gotTask != task || platform != "shein" {
				t.Fatalf("preview args = %+v/%q, want original task/shein", gotTask, platform)
			}
			return expectedPreview, nil
		},
	})

	preview, err := recovery.recoverSheinSubmitViaRemoteConfirmation(context.Background(), state)
	if err != nil {
		t.Fatalf("recoverSheinSubmitViaRemoteConfirmation() err = %v", err)
	}
	if preview != expectedPreview {
		t.Fatalf("preview = %+v, want %+v", preview, expectedPreview)
	}
	wantCalls := []string{"build_api", "resolve_remote", "remember", "persist", "preview"}
	if len(calls) != len(wantCalls) {
		t.Fatalf("calls = %+v, want %+v", calls, wantCalls)
	}
	for i := range wantCalls {
		if calls[i] != wantCalls[i] {
			t.Fatalf("calls[%d] = %q, want %q; full calls = %+v", i, calls[i], wantCalls[i], calls)
		}
	}
	if task.Result.Shein.Submission.RemoteStatus != sheinpub.SubmissionRemoteStatusConfirmed {
		t.Fatalf("remote status = %q, want confirmed", task.Result.Shein.Submission.RemoteStatus)
	}
	if task.Result.Shein.Submission.CurrentAction != "" || task.Result.Shein.Submission.CurrentPhase != "" || task.Result.Shein.Submission.CurrentRequestID != "" {
		t.Fatalf("submission current state = %+v, want cleared in-flight fields", task.Result.Shein.Submission)
	}
	if task.Result.Shein.Submission.Publish == nil || task.Result.Shein.Submission.Publish.RemoteRecordID != "record-remote-ok" {
		t.Fatalf("publish record = %+v, want remote record id record-remote-ok", task.Result.Shein.Submission.Publish)
	}
	if len(task.Result.Shein.SubmissionEvents) < 3 {
		t.Fatalf("submission events = %+v, want completion, confirm, and running confirm events", task.Result.Shein.SubmissionEvents)
	}
	if task.Result.Shein.SubmissionEvents[0].Status != sheinpub.SubmissionStatusSuccess {
		t.Fatalf("completion event = %+v, want success status", task.Result.Shein.SubmissionEvents[0])
	}
	if task.Result.Shein.SubmissionEvents[1].Status != sheinpub.SubmissionRemoteStatusConfirmed || task.Result.Shein.SubmissionEvents[1].Phase != sheinpub.SubmissionPhaseConfirmRemote {
		t.Fatalf("confirm event = %+v, want confirmed confirm_remote event", task.Result.Shein.SubmissionEvents[1])
	}
	if task.Result.Shein.SubmissionEvents[2].Status != sheinpub.SubmissionStatusRunning || task.Result.Shein.SubmissionEvents[2].Phase != sheinpub.SubmissionPhaseConfirmRemote {
		t.Fatalf("confirm phase event = %+v, want running confirm_remote event", task.Result.Shein.SubmissionEvents[2])
	}
}

func TestTaskSubmissionRecoveryServicePersistSheinRecoveredRemoteFailurePersistsFailureEvent(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	startedAt := time.Now().Add(-4 * time.Minute)
	beginSheinSubmitAttempt(task.Result.Shein, "publish", "recover-remote-fail-123", sheinpub.SubmissionPhaseConfirmRemote, startedAt)
	response := &sheinpub.SubmissionResponse{
		Code:    "0",
		Message: "success",
		Success: true,
		SPUName: "SPU-fail",
	}
	sheinpub.SetSubmissionRemoteResponse(task.Result.Shein, "publish", "recover-remote-fail-123", "SUP-fail", response)
	task.Result.Shein.Submission.Publish.SupplierCode = "SUP-fail"
	state, err := buildRecoveredSheinRemoteState(task, "publish")
	if err != nil {
		t.Fatalf("buildRecoveredSheinRemoteState() err = %v", err)
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	recovery := newTaskSubmissionRecoveryService(taskSubmissionRecoveryServiceConfig{
		repo: repo,
	})
	remoteErr := errors.New("remote confirmation failed")

	err = recovery.persistSheinRecoveredRemoteFailure(context.Background(), state, remoteErr)
	if !errors.Is(err, remoteErr) {
		t.Fatalf("persistSheinRecoveredRemoteFailure() err = %v, want %v", err, remoteErr)
	}

	saved, getErr := repo.GetTask(context.Background(), task.ID)
	if getErr != nil {
		t.Fatalf("get task: %v", getErr)
	}
	if saved.Result.Shein.Submission.LastStatus != sheinpub.SubmissionStatusFailed {
		t.Fatalf("last status = %q, want failed", saved.Result.Shein.Submission.LastStatus)
	}
	if saved.Result.Shein.Submission.CurrentAction != "" || saved.Result.Shein.Submission.CurrentPhase != "" || saved.Result.Shein.Submission.CurrentRequestID != "" {
		t.Fatalf("submission current state = %+v, want cleared in-flight fields", saved.Result.Shein.Submission)
	}
	if len(saved.Result.Shein.SubmissionEvents) == 0 {
		t.Fatal("expected failure event to be appended")
	}
	if saved.Result.Shein.SubmissionEvents[0].Status != sheinpub.SubmissionStatusFailed || saved.Result.Shein.SubmissionEvents[0].Phase != sheinpub.SubmissionPhaseConfirmRemote {
		t.Fatalf("failure event = %+v, want failed confirm_remote event", saved.Result.Shein.SubmissionEvents[0])
	}
	if saved.Result.Shein.SubmissionEvents[0].ErrorMessage != remoteErr.Error() {
		t.Fatalf("failure event error = %q, want %q", saved.Result.Shein.SubmissionEvents[0].ErrorMessage, remoteErr.Error())
	}
}

func TestTaskSubmissionRecoveryServiceRecoverSheinSubmitRemoteUsesLocalRecoveryForAcceptedResponse(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	startedAt := time.Now().Add(-2 * time.Minute)
	beginSheinSubmitAttempt(task.Result.Shein, "publish", "recover-route-local-123", sheinpub.SubmissionPhasePersistResult, startedAt)
	response := &sheinpub.SubmissionResponse{
		Code:    "0",
		Message: "success",
		Success: true,
		SPUName: "SPU-route-local",
	}
	sheinpub.SetSubmissionRemoteResponse(task.Result.Shein, "publish", "recover-route-local-123", "SUP-route-local", response)
	expectedPreview := &ListingKitPreview{TaskID: task.ID}
	var calls []string

	recovery := newTaskSubmissionRecoveryService(taskSubmissionRecoveryServiceConfig{
		rememberSheinSubmitted: func(gotTask *Task, action string) {
			calls = append(calls, "remember")
			if gotTask != task || action != "publish" {
				t.Fatalf("remember args = %+v/%q, want original task/publish", gotTask, action)
			}
		},
		persistSuccessfulSubmission: func(_ context.Context, taskID string, gotTask *Task, action string) error {
			calls = append(calls, "persist")
			if taskID != task.ID || gotTask != task || action != "publish" {
				t.Fatalf("persist args = %q/%+v/%q, want %q/original/publish", taskID, gotTask, action, task.ID)
			}
			return nil
		},
		buildTaskPreview: func(_ context.Context, gotTask *Task, platform string) (*ListingKitPreview, error) {
			calls = append(calls, "preview")
			if gotTask != task || platform != "shein" {
				t.Fatalf("preview args = %+v/%q, want original task/shein", gotTask, platform)
			}
			return expectedPreview, nil
		},
		buildSheinSubmitProductAPI: func(_ context.Context, _ *Task) (sheinproduct.ProductAPI, error) {
			t.Fatal("buildSheinSubmitProductAPI should not be called for local recovery route")
			return nil, nil
		},
	})

	preview, err := recovery.recoverSheinSubmitRemote(context.Background(), task, "publish")
	if err != nil {
		t.Fatalf("recoverSheinSubmitRemote() err = %v", err)
	}
	if preview != expectedPreview {
		t.Fatalf("preview = %+v, want %+v", preview, expectedPreview)
	}
	wantCalls := []string{"remember", "persist", "preview"}
	if len(calls) != len(wantCalls) {
		t.Fatalf("calls = %+v, want %+v", calls, wantCalls)
	}
	for i := range wantCalls {
		if calls[i] != wantCalls[i] {
			t.Fatalf("calls[%d] = %q, want %q; full calls = %+v", i, calls[i], wantCalls[i], calls)
		}
	}
}

func TestTaskSubmissionRecoveryServiceRecoverSheinSubmitRemoteUsesRemoteConfirmationForUnknownResponse(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	startedAt := time.Now().Add(-3 * time.Minute)
	beginSheinSubmitAttempt(task.Result.Shein, "publish", "recover-route-remote-123", sheinpub.SubmissionPhaseSubmitRemote, startedAt)
	response := &sheinpub.SubmissionResponse{
		Code:    "0",
		Message: "accepted",
		Success: false,
		SPUName: "SPU-route-remote",
	}
	sheinpub.SetSubmissionRemoteResponse(task.Result.Shein, "publish", "recover-route-remote-123", "SUP-route-remote", response)
	task.Result.Shein.Submission.Publish.SupplierCode = "SUP-route-remote"
	expectedPreview := &ListingKitPreview{TaskID: task.ID}
	var calls []string

	recovery := newTaskSubmissionRecoveryService(taskSubmissionRecoveryServiceConfig{
		buildSheinSubmitProductAPI: func(_ context.Context, gotTask *Task) (sheinproduct.ProductAPI, error) {
			calls = append(calls, "build_api")
			if gotTask != task {
				t.Fatalf("build api task = %+v, want original task", gotTask)
			}
			return stubSheinProductAPI{}, nil
		},
		resolveRemoteStatusCallback: func(request *sheinRemoteStatusRequest) (*sheinRemoteConfirmation, error) {
			calls = append(calls, "resolve_remote")
			if request.action != "publish" || request.requestID != "recover-route-remote-123" {
				t.Fatalf("resolve args = %q/%q, want publish/recover-route-remote-123", request.action, request.requestID)
			}
			event := sheinpub.BuildSubmissionConfirmRemoteEvent(task.ID, "publish", sheinpub.SubmissionRemoteStatusConfirmed, request.requestID, request.startedAt, "remote confirmed", nil)
			return &sheinRemoteConfirmation{
				RemoteStatus: sheinpub.SubmissionRemoteStatusConfirmed,
				Record: &sheinproduct.RecordItem{
					RecordID:     "record-route-remote",
					SupplierCode: "SUP-route-remote",
				},
				CheckedAt: time.Now(),
				Message:   "remote confirmed",
				Event:     &event,
			}, nil
		},
		rememberSheinSubmitted: func(gotTask *Task, action string) {
			calls = append(calls, "remember")
			if gotTask != task || action != "publish" {
				t.Fatalf("remember args = %+v/%q, want original task/publish", gotTask, action)
			}
		},
		persistSuccessfulSubmission: func(_ context.Context, taskID string, gotTask *Task, action string) error {
			calls = append(calls, "persist")
			if taskID != task.ID || gotTask != task || action != "publish" {
				t.Fatalf("persist args = %q/%+v/%q, want %q/original/publish", taskID, gotTask, action, task.ID)
			}
			return nil
		},
		buildTaskPreview: func(_ context.Context, gotTask *Task, platform string) (*ListingKitPreview, error) {
			calls = append(calls, "preview")
			if gotTask != task || platform != "shein" {
				t.Fatalf("preview args = %+v/%q, want original task/shein", gotTask, platform)
			}
			return expectedPreview, nil
		},
	})

	preview, err := recovery.recoverSheinSubmitRemote(context.Background(), task, "publish")
	if err != nil {
		t.Fatalf("recoverSheinSubmitRemote() err = %v", err)
	}
	if preview != expectedPreview {
		t.Fatalf("preview = %+v, want %+v", preview, expectedPreview)
	}
	wantCalls := []string{"build_api", "resolve_remote", "remember", "persist", "preview"}
	if len(calls) != len(wantCalls) {
		t.Fatalf("calls = %+v, want %+v", calls, wantCalls)
	}
	for i := range wantCalls {
		if calls[i] != wantCalls[i] {
			t.Fatalf("calls[%d] = %q, want %q; full calls = %+v", i, calls[i], wantCalls[i], calls)
		}
	}
}

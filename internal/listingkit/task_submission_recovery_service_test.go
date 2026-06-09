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
	setSheinSubmitRemoteResponse(task.Result.Shein, "publish", "recover-local-123", "SUP-local", response)
	state := buildRecoveredSheinRemoteState(task.Result.Shein.Submission, "publish")
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

	preview, err := recovery.recoverSheinSubmitLocally(context.Background(), task, task.Result.Shein, "publish", state)
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
	setSheinSubmitRemoteResponse(task.Result.Shein, "publish", "recover-remote-ok-123", "SUP-remote", response)
	task.Result.Shein.Submission.Publish.SupplierCode = "SUP-remote"
	state := buildRecoveredSheinRemoteState(task.Result.Shein.Submission, "publish")
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
		resolveRemoteStatusCallback: func(gotProductAPI sheinproduct.ProductAPI, otherAPI sheinother.OtherAPI, action, requestID string, lookupCodes []string, spuName string, defaultConfirmed bool, fallbackMessage string, gotStartedAt time.Time, taskID string) (*sheinRemoteConfirmation, error) {
			calls = append(calls, "resolve_remote")
			if gotProductAPI == nil {
				t.Fatal("product api = nil, want built api")
			}
			if otherAPI != nil {
				t.Fatalf("other api = %+v, want nil", otherAPI)
			}
			if action != "publish" || requestID != "recover-remote-ok-123" {
				t.Fatalf("resolve args = %q/%q, want publish/recover-remote-ok-123", action, requestID)
			}
			if len(lookupCodes) == 0 || lookupCodes[0] != "SUP-remote" {
				t.Fatalf("lookup codes = %+v, want supplier code first", lookupCodes)
			}
			if spuName != "SPU-remote" {
				t.Fatalf("spu name = %q, want SPU-remote", spuName)
			}
			if !defaultConfirmed {
				t.Fatal("defaultConfirmed = false, want true")
			}
			if gotStartedAt.IsZero() || taskID != task.ID {
				t.Fatalf("resolve start/task = %v/%q, want non-zero/%q", gotStartedAt, taskID, task.ID)
			}
			event := submission.BuildConfirmRemoteEvent(task.ID, "publish", sheinpub.SubmissionRemoteStatusConfirmed, "recover-remote-ok-123", gotStartedAt, "remote confirmed", nil)
			return &sheinRemoteConfirmation{
				remoteStatus: sheinpub.SubmissionRemoteStatusConfirmed,
				record: &sheinproduct.RecordItem{
					RecordID:     "record-remote-ok",
					SupplierCode: "SUP-remote",
					State:        6,
					AuditState:   7,
				},
				checkedAt: checkedAt,
				message:   "remote confirmed",
				event:     &event,
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

	preview, err := recovery.recoverSheinSubmitViaRemoteConfirmation(context.Background(), task, task.Result.Shein, "publish", state)
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
	setSheinSubmitRemoteResponse(task.Result.Shein, "publish", "recover-remote-fail-123", "SUP-fail", response)
	task.Result.Shein.Submission.Publish.SupplierCode = "SUP-fail"
	state := buildRecoveredSheinRemoteState(task.Result.Shein.Submission, "publish")
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	recovery := newTaskSubmissionRecoveryService(taskSubmissionRecoveryServiceConfig{
		repo: repo,
	})
	remoteErr := errors.New("remote confirmation failed")

	err := recovery.persistSheinRecoveredRemoteFailure(context.Background(), task, task.Result.Shein, "publish", state, remoteErr)
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

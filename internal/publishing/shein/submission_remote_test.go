package shein

import (
	"errors"
	"reflect"
	"testing"
	"time"

	listingsubmission "task-processor/internal/listing/submission"
	sheinproduct "task-processor/internal/shein/api/product"
)

func TestSubmissionResponseAccepted(t *testing.T) {
	t.Parallel()

	if !SubmissionResponseAccepted(&SubmissionResponse{Success: true}) {
		t.Fatal("SubmissionResponseAccepted(success) = false, want true")
	}
	if SubmissionResponseAccepted(nil) {
		t.Fatal("SubmissionResponseAccepted(nil) = true, want false")
	}
}

func TestSubmissionResponseAcceptedForAction(t *testing.T) {
	t.Parallel()

	if !SubmissionResponseAcceptedForAction("publish", &SubmissionResponse{Success: true}) {
		t.Fatal("SubmissionResponseAcceptedForAction(publish success) = false, want true")
	}
	if !SubmissionResponseAcceptedForAction("save_draft", &SubmissionResponse{Code: "0"}) {
		t.Fatal("SubmissionResponseAcceptedForAction(save_draft code=0) = false, want true")
	}
	if SubmissionResponseAcceptedForAction("publish", &SubmissionResponse{Code: "0"}) {
		t.Fatal("SubmissionResponseAcceptedForAction(publish code=0) = true, want false")
	}
}

func TestAppendSubmissionEventAssignsIDAndPrependsHistory(t *testing.T) {
	t.Parallel()

	pkg := &Package{}
	older := SubmissionEvent{ID: "older", Action: "submit_phase"}
	AppendSubmissionEvent(pkg, older)
	AppendSubmissionEvent(pkg, SubmissionEvent{Action: "publish"})

	if len(pkg.SubmissionEvents) != 2 {
		t.Fatalf("submission events = %d, want 2", len(pkg.SubmissionEvents))
	}
	if pkg.SubmissionEvents[0].ID == "" {
		t.Fatal("latest submission event id = empty, want generated id")
	}
	if pkg.SubmissionEvents[1].ID != "older" {
		t.Fatalf("older event id = %q, want older", pkg.SubmissionEvents[1].ID)
	}
}

func TestBuildSubmissionRefreshConfirmRemoteRunningEvent(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, 6, 14, 16, 0, 0, 0, time.UTC)
	event := BuildSubmissionRefreshConfirmRemoteRunningEvent("task-1", "publish", "req-1", startedAt)
	if event.TaskID != "task-1" || event.RequestID != "req-1" {
		t.Fatalf("event = %+v, want task/request ids", event)
	}
	if event.Phase != SubmissionPhaseConfirmRemote || event.Status != SubmissionStatusRunning {
		t.Fatalf("event = %+v, want running confirm_remote", event)
	}
	if event.Detail != "刷新 SHEIN 远端提交状态" {
		t.Fatalf("detail = %q, want refresh detail", event.Detail)
	}
}

func TestBuildSubmissionAttemptEventAndPhaseEvent(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, 6, 14, 16, 0, 0, 0, time.UTC)
	record := &SubmissionRecord{
		Action:         "publish",
		Status:         SubmissionStatusSuccess,
		RequestID:      "req-1",
		Phase:          SubmissionPhasePersistResult,
		StartedAt:      startedAt,
		RemoteRecordID: "record-1",
		Result:         &SubmissionResponse{Success: true, SPUName: "SPU-1"},
	}
	attemptEvent := BuildSubmissionAttemptEvent("task-1", "publish", record, record.Result, nil, startedAt)
	if attemptEvent.TaskID != "task-1" || attemptEvent.Phase != SubmissionPhasePersistResult {
		t.Fatalf("attempt event = %+v, want task and phase", attemptEvent)
	}
	if attemptEvent.RemoteRecordID != "record-1" || attemptEvent.Response == nil || attemptEvent.Response.SPUName != "SPU-1" {
		t.Fatalf("attempt event = %+v, want remote record id and response", attemptEvent)
	}

	phaseEvent := BuildSubmissionPhaseEvent("task-1", "publish", SubmissionPhaseSubmitRemote, SubmissionStatusRunning, "req-1", startedAt, "", nil)
	if phaseEvent.Detail != "提交 SHEIN 发布请求" {
		t.Fatalf("phase detail = %q, want publish default detail", phaseEvent.Detail)
	}
}

func TestBuildSubmissionConfirmRemoteUpdateWithEvent(t *testing.T) {
	t.Parallel()

	checkedAt := time.Date(2026, 6, 14, 16, 30, 0, 0, time.UTC)
	event := &SubmissionEvent{
		TaskID:    "task-1",
		Action:    "publish",
		Phase:     SubmissionPhaseConfirmRemote,
		Status:    SubmissionRemoteStatusConfirmed,
		RequestID: "req-1",
		StartedAt: checkedAt.Add(-time.Minute),
		Detail:    "confirmed remotely",
	}
	record := &sheinproduct.RecordItem{
		RecordID:     "record-123",
		SupplierCode: "SKC-1",
		State:        4,
		AuditState:   5,
	}

	update, ok := BuildSubmissionConfirmRemoteUpdateWithEvent(SubmissionRemoteStatusConfirmed, record, checkedAt, "confirmed remotely", event)
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if update.RemoteStatus != SubmissionRemoteStatusConfirmed || update.Record != record {
		t.Fatalf("update = %+v, want original status and record", update)
	}
	if !update.CheckedAt.Equal(checkedAt) || update.Message != "confirmed remotely" {
		t.Fatalf("update = %+v, want checkedAt/message copied", update)
	}
	if update.Event == nil || update.Event.RemoteRecordID != "record-123" {
		t.Fatalf("event = %+v, want remote record id", update.Event)
	}
}

func TestApplySubmissionConfirmRemoteWithEvent(t *testing.T) {
	t.Parallel()

	pkg := &Package{
		SubmissionState: &SubmissionReport{
			Publish: &SubmissionRecord{
				Action:    "publish",
				RequestID: "req-1",
			},
		},
	}
	checkedAt := time.Date(2026, 6, 14, 16, 30, 0, 0, time.UTC)
	ok := ApplySubmissionConfirmRemoteWithEvent(
		pkg,
		"publish",
		"req-1",
		SubmissionRemoteStatusConfirmed,
		&sheinproduct.RecordItem{
			RecordID:     "record-123",
			SupplierCode: "SKC-1",
			State:        4,
			AuditState:   5,
		},
		checkedAt,
		"confirmed remotely",
		&SubmissionEvent{
			TaskID:    "task-1",
			Action:    "publish",
			Phase:     SubmissionPhaseConfirmRemote,
			Status:    SubmissionRemoteStatusConfirmed,
			RequestID: "req-1",
			StartedAt: checkedAt.Add(-time.Minute),
			Detail:    "confirmed remotely",
		},
	)
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if len(pkg.SubmissionEvents) != 1 || pkg.SubmissionEvents[0].RemoteRecordID != "record-123" {
		t.Fatalf("submission events = %+v, want event with remote record id", pkg.SubmissionEvents)
	}
	if pkg.SubmissionState.Publish == nil || pkg.SubmissionState.Publish.RemoteRecordID != "record-123" {
		t.Fatalf("publish record = %+v, want remote record id", pkg.SubmissionState.Publish)
	}
}

func TestApplySubmissionConfirmRemoteUpdateWithoutEvent(t *testing.T) {
	t.Parallel()

	pkg := &Package{
		SubmissionState: &SubmissionReport{
			Publish: &SubmissionRecord{
				Action:    "publish",
				RequestID: "req-1",
			},
		},
	}
	checkedAt := time.Date(2026, 6, 14, 16, 30, 0, 0, time.UTC)
	ApplySubmissionConfirmRemoteUpdate(pkg, "publish", "req-1", SubmissionConfirmRemoteUpdate{
		RemoteStatus: SubmissionRemoteStatusPending,
		Record: &sheinproduct.RecordItem{
			RecordID:     "record-only",
			SupplierCode: "SKC-1",
			State:        1,
			AuditState:   2,
		},
		CheckedAt: checkedAt,
		Message:   "pending remotely",
	})
	if len(pkg.SubmissionEvents) != 0 {
		t.Fatalf("submission events = %+v, want no event appended", pkg.SubmissionEvents)
	}
	if pkg.SubmissionState.Publish == nil || pkg.SubmissionState.Publish.RemoteRecordID != "record-only" {
		t.Fatalf("publish record = %+v, want remote record id", pkg.SubmissionState.Publish)
	}
}

func TestSubmissionResponseAcceptedWithSPU(t *testing.T) {
	t.Parallel()

	if !SubmissionResponseAcceptedWithSPU(&SubmissionResponse{Success: true, SPUName: " SPU-123 "}) {
		t.Fatal("SubmissionResponseAcceptedWithSPU(success+spu) = false, want true")
	}
	if SubmissionResponseAcceptedWithSPU(&SubmissionResponse{Success: true}) {
		t.Fatal("SubmissionResponseAcceptedWithSPU(blank spu) = true, want false")
	}
}

func TestConfirmedSubmissionResponse(t *testing.T) {
	t.Parallel()

	existing := &SubmissionResponse{Success: true, Message: "existing"}
	if got := ConfirmedSubmissionResponse(existing, "publish"); got != existing {
		t.Fatalf("ConfirmedSubmissionResponse(existing) = %+v, want original response", got)
	}

	saveDraft := ConfirmedSubmissionResponse(nil, "save_draft")
	if saveDraft == nil || saveDraft.Code != "0" || !saveDraft.Success || saveDraft.Message != "save draft confirmed by remote check" {
		t.Fatalf("save draft confirmed response = %+v", saveDraft)
	}

	publish := ConfirmedSubmissionResponse(nil, "publish")
	if publish == nil || publish.Code != "0" || !publish.Success || publish.Message != "publish confirmed by remote check" {
		t.Fatalf("publish confirmed response = %+v", publish)
	}
}

func TestSubmissionStartedAtAndResponseForAction(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, 6, 14, 18, 30, 0, 0, time.UTC)
	fallback := startedAt.Add(-time.Minute)
	lastResult := &SubmissionResponse{Success: true, Message: "last"}
	pkg := &Package{
		SubmissionState: &SubmissionReport{
			InFlightStartedAt: &fallback,
			LastResult:        lastResult,
			Publish: &SubmissionRecord{
				Action:    "publish",
				RequestID: "req-1",
				StartedAt: startedAt,
				Result:    &SubmissionResponse{Success: true, Message: "record"},
			},
		},
	}

	if got := SubmissionStartedAt(pkg, "publish", "req-1", fallback.Add(-time.Minute)); !got.Equal(startedAt) {
		t.Fatalf("SubmissionStartedAt(match) = %v, want %v", got, startedAt)
	}
	if got := SubmissionStartedAt(pkg, "publish", "other", fallback); !got.Equal(fallback) {
		t.Fatalf("SubmissionStartedAt(fallback) = %v, want %v", got, fallback)
	}
	if got := SubmissionResponseForAction(pkg, "publish"); got == nil || got.Message != "record" {
		t.Fatalf("SubmissionResponseForAction(record) = %+v, want record result", got)
	}
	pkg.SubmissionState.Publish.Result = nil
	if got := SubmissionResponseForAction(pkg, "publish"); got != lastResult {
		t.Fatalf("SubmissionResponseForAction(last) = %+v, want last result", got)
	}
}

func TestSubmissionStatePackage(t *testing.T) {
	t.Parallel()

	if pkg, ok := SubmissionStatePackage(nil); ok || pkg != nil {
		t.Fatalf("SubmissionStatePackage(nil) = (%+v, %v), want (nil, false)", pkg, ok)
	}

	legacy := &SubmissionReport{LastAction: "publish"}
	pkg, ok := SubmissionStatePackage(&Package{Submission: legacy})
	if !ok {
		t.Fatal("SubmissionStatePackage(legacy submission) = false, want true")
	}
	if pkg == nil || pkg.SubmissionState != legacy || pkg.Submission != legacy {
		t.Fatalf("SubmissionStatePackage() = %+v, want canonicalized submission state", pkg)
	}
}

func TestPreviewPayloadPackage(t *testing.T) {
	t.Parallel()

	if pkg, ok := PreviewPayloadPackage(nil); ok || pkg != nil {
		t.Fatalf("PreviewPayloadPackage(nil) = (%+v, %v), want (nil, false)", pkg, ok)
	}

	legacy := &sheinproduct.Product{SPUName: "legacy-preview"}
	pkg, ok := PreviewPayloadPackage(&Package{PreviewProduct: legacy})
	if !ok {
		t.Fatal("PreviewPayloadPackage(legacy preview) = false, want true")
	}
	if pkg == nil || pkg.PreviewPayload != legacy || pkg.PreviewProduct != legacy {
		t.Fatalf("PreviewPayloadPackage() = %+v, want canonicalized preview payload", pkg)
	}
}

func TestResolveSubmissionRefreshSelection(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 14, 18, 30, 0, 0, time.UTC)
	pkg := &Package{
		SkcList: []SKCPackage{{SupplierCode: "PKG-SKC-1"}},
		SubmissionState: &SubmissionReport{
			Publish: &SubmissionRecord{
				Action:    "publish",
				RequestID: "refresh-123",
				StartedAt: now,
			},
		},
	}

	selection := ResolveSubmissionRefreshSelection(pkg)
	if selection.Action != "publish" {
		t.Fatalf("ResolveSubmissionRefreshSelection().Action = %q, want publish", selection.Action)
	}
	if selection.Record != pkg.SubmissionState.Publish {
		t.Fatalf("ResolveSubmissionRefreshSelection().Record = %+v, want publish record", selection.Record)
	}
	if selection.SupplierCode != "PKG-SKC-1" {
		t.Fatalf("ResolveSubmissionRefreshSelection().SupplierCode = %q, want PKG-SKC-1", selection.SupplierCode)
	}

	pkg.SubmissionState.Publish.SupplierCode = "REC-SKC-1"
	selection = ResolveSubmissionRefreshSelection(pkg)
	if selection.SupplierCode != "REC-SKC-1" {
		t.Fatalf("ResolveSubmissionRefreshSelection().SupplierCode = %q, want REC-SKC-1", selection.SupplierCode)
	}
}

func TestResolveSubmissionRecoverySelection(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, 6, 14, 18, 30, 0, 0, time.UTC)
	lastResult := &SubmissionResponse{Success: true, Message: "last"}
	pkg := &Package{
		SubmissionState: &SubmissionReport{
			CurrentRequestID: "recover-123",
			LastResult:       lastResult,
			Publish: &SubmissionRecord{
				Action:    "publish",
				RequestID: "recover-123",
				StartedAt: startedAt,
			},
		},
	}

	selection := ResolveSubmissionRecoverySelection(pkg, "publish")
	if selection.Report != pkg.SubmissionState {
		t.Fatalf("ResolveSubmissionRecoverySelection().Report = %+v, want submission report", selection.Report)
	}
	if selection.Record != pkg.SubmissionState.Publish {
		t.Fatalf("ResolveSubmissionRecoverySelection().Record = %+v, want publish record", selection.Record)
	}
	if selection.RequestID != "recover-123" {
		t.Fatalf("ResolveSubmissionRecoverySelection().RequestID = %q, want recover-123", selection.RequestID)
	}
	if selection.Response != lastResult {
		t.Fatalf("ResolveSubmissionRecoverySelection().Response = %+v, want last result fallback", selection.Response)
	}

	pkg.SubmissionState.Publish.Result = &SubmissionResponse{Success: true, Message: "record"}
	selection = ResolveSubmissionRecoverySelection(pkg, "publish")
	if selection.Response == nil || selection.Response.Message != "record" {
		t.Fatalf("ResolveSubmissionRecoverySelection().Response = %+v, want record result", selection.Response)
	}
}

func TestResolveSubmissionRemoteRefreshSelection(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, 6, 14, 18, 30, 0, 0, time.UTC)
	fallback := startedAt.Add(-time.Minute)
	lastResult := &SubmissionResponse{Success: true, Message: "last"}
	pkg := &Package{
		SubmissionState: &SubmissionReport{
			RemoteStatus: "confirmed",
			LastResult:   lastResult,
			Publish: &SubmissionRecord{
				Action:    "publish",
				RequestID: "req-1",
				StartedAt: startedAt,
			},
		},
	}

	selection := ResolveSubmissionRemoteRefreshSelection(pkg, "publish", "req-1", fallback)
	if !selection.StartedAt.Equal(startedAt) {
		t.Fatalf("ResolveSubmissionRemoteRefreshSelection().StartedAt = %v, want %v", selection.StartedAt, startedAt)
	}
	if selection.Response != lastResult {
		t.Fatalf("ResolveSubmissionRemoteRefreshSelection().Response = %+v, want last result fallback", selection.Response)
	}
	if selection.RemoteStatus != "confirmed" {
		t.Fatalf("ResolveSubmissionRemoteRefreshSelection().RemoteStatus = %q, want confirmed", selection.RemoteStatus)
	}

	pkg.SubmissionState.Publish.Result = &SubmissionResponse{Success: true, Message: "record"}
	selection = ResolveSubmissionRemoteRefreshSelection(pkg, "publish", "req-1", fallback)
	if selection.Response == nil || selection.Response.Message != "record" {
		t.Fatalf("ResolveSubmissionRemoteRefreshSelection().Response = %+v, want record result", selection.Response)
	}
}

func TestSubmissionRefreshMutationMatchHelpers(t *testing.T) {
	t.Parallel()

	pkg := &Package{
		SubmissionState: &SubmissionReport{
			LastAction: "save_draft",
			Publish: &SubmissionRecord{
				Action:    "publish",
				RequestID: "publish-req",
			},
			SaveDraft: &SubmissionRecord{
				Action:    "save_draft",
				RequestID: "save-draft-req",
			},
		},
	}

	if !SubmissionRefreshActionMatches(pkg, "save_draft") {
		t.Fatal("SubmissionRefreshActionMatches() = false, want true for current action")
	}
	if SubmissionRefreshActionMatches(pkg, "publish") {
		t.Fatal("SubmissionRefreshActionMatches() = true, want false for changed action")
	}
	if !SubmissionRefreshRequestMatches(pkg, "publish", " publish-req ") {
		t.Fatal("SubmissionRefreshRequestMatches() = false, want true after trimming request id")
	}
	if SubmissionRefreshRequestMatches(pkg, "publish", "other") {
		t.Fatal("SubmissionRefreshRequestMatches(other) = true, want false")
	}
	if SubmissionRefreshRequestMatches(pkg, "unknown", "publish-req") {
		t.Fatal("SubmissionRefreshRequestMatches(unknown action) = true, want false")
	}
}

func TestSubmissionRecordResult(t *testing.T) {
	t.Parallel()

	resp := &SubmissionResponse{Success: true}
	if got := SubmissionRecordResult(&SubmissionRecord{Result: resp}); got != resp {
		t.Fatalf("SubmissionRecordResult() = %+v, want original response", got)
	}
}

func TestEnsureSubmissionReportInitializesState(t *testing.T) {
	t.Parallel()

	pkg := &Package{}
	report := EnsureSubmissionReport(pkg)
	if report == nil || pkg.SubmissionState == nil {
		t.Fatalf("EnsureSubmissionReport() = %+v, want initialized state", report)
	}
}

func TestSubmissionRecordForActionAndFindCompletedRecord(t *testing.T) {
	t.Parallel()

	finishedAt := testSubmissionRemoteFinishedAt()
	pkg := &Package{
		SubmissionState: &SubmissionReport{
			Publish: &SubmissionRecord{
				Action:     "publish",
				RequestID:  "req-1",
				FinishedAt: &finishedAt,
			},
		},
	}
	if got := SubmissionRecordForAction(pkg.SubmissionState, "publish"); got != pkg.SubmissionState.Publish {
		t.Fatalf("SubmissionRecordForAction() = %+v, want publish record", got)
	}
	if got := FindCompletedSubmissionRecordByRequestID(pkg, "publish", " req-1 "); got != pkg.SubmissionState.Publish {
		t.Fatalf("FindCompletedSubmissionRecordByRequestID() = %+v, want publish record", got)
	}
}

func TestLatestSubmissionOutcomeEventPrimarySubmissionRecordAndWorkflowStatus(t *testing.T) {
	t.Parallel()

	pkg := &Package{
		SubmissionEvents: []SubmissionEvent{
			{Action: "submit_phase", Phase: SubmissionPhaseConfirmRemote, Status: SubmissionRemoteStatusPending},
			{Action: "save_draft", Status: SubmissionStatusSuccess},
		},
		SubmissionState: &SubmissionReport{
			LastAction: "publish",
			Publish: &SubmissionRecord{
				Action:         "publish",
				Status:         SubmissionStatusSuccess,
				RemoteRecordID: "record-1",
			},
		},
	}
	if got := LatestSubmissionOutcomeEvent(pkg); got == nil || got.Action != "save_draft" {
		t.Fatalf("LatestSubmissionOutcomeEvent() = %+v, want save_draft outcome", got)
	}
	if got := PrimarySubmissionRecord(pkg.SubmissionState); got != pkg.SubmissionState.Publish {
		t.Fatalf("PrimarySubmissionRecord() = %+v, want publish record", got)
	}
	if got := SubmissionWorkflowStatus(pkg, false); got != "draft_saved" {
		t.Fatalf("SubmissionWorkflowStatus() = %q, want draft_saved", got)
	}
	if got := SubmissionWorkflowStatus(&Package{}, true); got != "ready_to_submit" {
		t.Fatalf("SubmissionWorkflowStatus(ready) = %q, want ready_to_submit", got)
	}
}

func TestResolveSubmissionProjection(t *testing.T) {
	t.Parallel()

	checkedAt := testSubmissionRemoteFinishedAt()
	pkg := &Package{
		SubmissionEvents: []SubmissionEvent{
			{Action: "submit_phase", Phase: SubmissionPhaseConfirmRemote, Status: SubmissionRemoteStatusPending},
			{Action: "publish", Status: SubmissionStatusSuccess},
		},
		SubmissionState: &SubmissionReport{
			LastStatus:      SubmissionStatusSuccess,
			LastError:       "",
			RemoteStatus:    SubmissionRemoteStatusConfirmed,
			RemoteCheckedAt: &checkedAt,
			Publish: &SubmissionRecord{
				Action:         "publish",
				RemoteRecordID: "record-1",
			},
		},
	}

	projection := ResolveSubmissionProjection(pkg, false)
	if projection.WorkflowStatus != "published" {
		t.Fatalf("ResolveSubmissionProjection().WorkflowStatus = %q, want published", projection.WorkflowStatus)
	}
	if projection.LatestStatus != SubmissionStatusSuccess {
		t.Fatalf("ResolveSubmissionProjection().LatestStatus = %q, want success", projection.LatestStatus)
	}
	if projection.RemoteStatus != SubmissionRemoteStatusConfirmed {
		t.Fatalf("ResolveSubmissionProjection().RemoteStatus = %q, want confirmed", projection.RemoteStatus)
	}
	if projection.RemoteRecordID != "record-1" {
		t.Fatalf("ResolveSubmissionProjection().RemoteRecordID = %q, want record-1", projection.RemoteRecordID)
	}
	if projection.RemoteCheckedAt == nil || !projection.RemoteCheckedAt.Equal(checkedAt) {
		t.Fatalf("ResolveSubmissionProjection().RemoteCheckedAt = %v, want %v", projection.RemoteCheckedAt, checkedAt)
	}
}

func TestRemoteLookupSPUNamePrefersActionRecordThenLastResult(t *testing.T) {
	t.Parallel()

	pkg := &Package{
		SubmissionState: &SubmissionReport{
			LastResult: &SubmissionResponse{SPUName: "LAST"},
			Publish: &SubmissionRecord{
				Action: "publish",
				Result: &SubmissionResponse{SPUName: "PUBLISH"},
			},
		},
	}
	if got := RemoteLookupSPUName(pkg, "publish"); got != "PUBLISH" {
		t.Fatalf("RemoteLookupSPUName() = %q, want PUBLISH", got)
	}
	if got := RemoteLookupSPUName(&Package{SubmissionState: &SubmissionReport{LastResult: &SubmissionResponse{SPUName: "LAST"}}}, "publish"); got != "LAST" {
		t.Fatalf("RemoteLookupSPUName() fallback = %q, want LAST", got)
	}
}

func TestRemotePublishAccepted(t *testing.T) {
	t.Parallel()

	pkg := &Package{
		SubmissionState: &SubmissionReport{
			Publish: &SubmissionRecord{
				Action: "publish",
				Result: &SubmissionResponse{Success: true, SPUName: "SPU-123"},
			},
		},
	}
	if !RemotePublishAccepted(pkg, "publish") {
		t.Fatal("RemotePublishAccepted() = false, want true")
	}
	if RemotePublishAccepted(pkg, "save_draft") {
		t.Fatal("RemotePublishAccepted(save_draft) = true, want false")
	}
}

func TestSubmissionSucceededAndClearSubmissionInFlight(t *testing.T) {
	t.Parallel()

	finishedAt := testSubmissionRemoteFinishedAt()
	report := &SubmissionReport{
		CurrentAction:    "publish",
		CurrentPhase:     SubmissionPhaseSubmitRemote,
		CurrentRequestID: "req-1",
		Publish:          &SubmissionRecord{Status: SubmissionStatusSuccess, FinishedAt: &finishedAt},
	}
	if !SubmissionSucceeded(&Package{SubmissionState: report}, "publish") {
		t.Fatal("SubmissionSucceeded() = false, want true")
	}
	ClearSubmissionInFlight(report, "publish", "req-1")
	if report.CurrentAction != "" || report.CurrentPhase != "" || report.CurrentRequestID != "" {
		t.Fatalf("report = %+v, want cleared in-flight state", report)
	}
}

func TestSubmissionRemoteResponsePersisted(t *testing.T) {
	t.Parallel()

	pkg := &Package{
		SubmissionState: &SubmissionReport{
			Publish: &SubmissionRecord{
				Action:    "publish",
				RequestID: "req-1",
				Result:    &SubmissionResponse{Success: true},
			},
		},
	}
	if !SubmissionRemoteResponsePersisted(pkg, "publish", "req-1") {
		t.Fatal("SubmissionRemoteResponsePersisted() = false, want true")
	}
	if SubmissionRemoteResponsePersisted(pkg, "publish", "other") {
		t.Fatal("SubmissionRemoteResponsePersisted(other) = true, want false")
	}
	if SubmissionRemoteResponsePersisted(&Package{SubmissionState: &SubmissionReport{}}, "publish", "req-1") {
		t.Fatal("SubmissionRemoteResponsePersisted(empty) = true, want false")
	}
}

func TestSubmissionInFlightStateHelpers(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, 6, 14, 17, 0, 0, 0, time.UTC)
	expiresAt := startedAt.Add(5 * time.Minute)
	report := &SubmissionReport{
		AttemptCount:      3,
		CurrentAction:     "publish",
		CurrentPhase:      SubmissionPhaseSubmitRemote,
		CurrentRequestID:  "req-1",
		InFlightStartedAt: &startedAt,
		LeaseExpiresAt:    &expiresAt,
	}

	state := SubmissionInFlightState(report)
	if state.AttemptCount != 3 || state.CurrentRequestID != "req-1" {
		t.Fatalf("state = %+v, want attempt/request copied", state)
	}

	report = &SubmissionReport{}
	ApplySubmissionInFlightState(report, state)
	if report.AttemptCount != 3 || report.CurrentPhase != SubmissionPhaseSubmitRemote || report.CurrentRequestID != "req-1" {
		t.Fatalf("report = %+v, want state copied back", report)
	}
}

func TestSetSubmissionRecordPhaseAndResolveSubmissionAttemptRecord(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, 6, 14, 17, 0, 0, 0, time.UTC)
	finishedAt := startedAt.Add(2 * time.Minute)
	report := &SubmissionReport{
		AttemptCount:      2,
		InFlightStartedAt: &startedAt,
		Publish: &SubmissionRecord{
			Action:      "publish",
			RequestID:   "req-1",
			SubmittedAt: startedAt,
			StartedAt:   startedAt,
			Attempt:     2,
			Phase:       SubmissionPhaseValidate,
		},
	}

	if !SetSubmissionRecordPhase(report, "publish", "req-1", SubmissionPhaseSubmitRemote) {
		t.Fatal("SetSubmissionRecordPhase() = false, want true")
	}
	if report.Publish == nil || report.Publish.Phase != SubmissionPhaseSubmitRemote {
		t.Fatalf("publish record = %+v, want updated phase", report.Publish)
	}

	record := ResolveSubmissionAttemptRecord(report, "publish", "req-1", listingsubmission.AttemptSeedState{
		AttemptCount:      report.AttemptCount,
		InFlightStartedAt: report.InFlightStartedAt,
	}, finishedAt)
	if record == nil || record.RequestID != "req-1" || record.Attempt != 2 {
		t.Fatalf("record = %+v, want existing record reused", record)
	}
}

func TestSubmissionResponseOutcomeAndFinalizeHelpers(t *testing.T) {
	t.Parallel()

	finishedAt := time.Date(2026, 6, 14, 17, 30, 0, 0, time.UTC)
	outcome := SubmissionResponseOutcome(&SubmissionResponse{
		Success:         true,
		Code:            "0",
		Message:         "ok",
		ValidationNotes: []string{"note-1"},
	})
	if outcome == nil || !outcome.Success || outcome.Code != "0" || len(outcome.ValidationNotes) != 1 {
		t.Fatalf("outcome = %+v, want copied response outcome", outcome)
	}

	record := &SubmissionRecord{Action: "publish"}
	ApplySubmissionAttemptFinalizeState(record, &SubmissionResponse{Success: true}, listingsubmission.AttemptFinalizeState{
		Status:       SubmissionStatusSuccess,
		ErrorMessage: "",
		FinishedAt:   finishedAt,
	})
	if record.Status != SubmissionStatusSuccess || record.Result == nil || record.FinishedAt == nil || !record.FinishedAt.Equal(finishedAt) {
		t.Fatalf("record after finalize = %+v, want response/status/finishedAt", record)
	}

	ApplySubmissionAttemptFailureState(record, SubmissionPhaseSubmitRemote, listingsubmission.AttemptFinalizeState{
		Status:       SubmissionStatusFailed,
		ErrorMessage: "boom",
		FinishedAt:   finishedAt.Add(time.Minute),
	})
	if record.Status != SubmissionStatusFailed || record.Phase != SubmissionPhaseSubmitRemote || record.Error != "boom" {
		t.Fatalf("record after failure finalize = %+v, want failed phase/error", record)
	}
}

func TestBuildSubmissionRunningRecordAndAttemptRecordFromSeed(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, 6, 14, 18, 0, 0, 0, time.UTC)
	record := BuildSubmissionRunningRecord("publish", "req-1", SubmissionPhaseValidate, startedAt, 3)
	if record == nil || record.Status != SubmissionStatusRunning || record.Attempt != 3 || record.Phase != SubmissionPhaseValidate {
		t.Fatalf("running record = %+v, want running attempt/phase fields", record)
	}

	seedRecord := BuildSubmissionAttemptRecordFromSeed(listingsubmission.AttemptRecordSeed{
		Action:      "publish",
		SubmittedAt: startedAt,
		RequestID:   "req-1",
		StartedAt:   startedAt,
		Attempt:     3,
	})
	if seedRecord == nil || seedRecord.RequestID != "req-1" || seedRecord.Attempt != 3 || seedRecord.Status != "" {
		t.Fatalf("seed record = %+v, want seed fields only", seedRecord)
	}
}

func TestApplySubmissionRecordAndMutations(t *testing.T) {
	t.Parallel()

	now := time.Now()
	pkg := &Package{}
	record := &SubmissionRecord{
		Action:      "publish",
		Status:      SubmissionStatusRunning,
		SubmittedAt: now,
		RequestID:   "req-1",
	}
	ApplySubmissionRecord(pkg, record)
	if pkg.SubmissionState == nil || pkg.SubmissionState.Publish == nil || pkg.SubmissionState.Publish.RequestID != "req-1" {
		t.Fatalf("submission state = %+v, want publish record", pkg.SubmissionState)
	}

	SetSubmissionSupplierCode(pkg, "publish", "req-1", "SUP-1")
	SetSubmissionRemoteResponse(pkg, "publish", "req-1", "SUP-1", &SubmissionResponse{Success: true, SPUName: "SPU-1"})
	SetSubmissionSnapshot(pkg, "publish", "req-1", &SubmitSnapshot{SPUName: "SPU-1"})
	SetSubmissionRemoteRecord(pkg, "publish", "req-1", SubmissionRemoteStatusConfirmed, &sheinproduct.RecordItem{
		RecordID:     "record-1",
		SupplierCode: "SUP-1",
		State:        4,
		AuditState:   5,
	}, now.Add(time.Minute), "confirmed remotely")

	saved := pkg.SubmissionState.Publish
	if saved.SupplierCode != "SUP-1" {
		t.Fatalf("supplier code = %q, want SUP-1", saved.SupplierCode)
	}
	if saved.Result == nil || saved.Result.SPUName != "SPU-1" {
		t.Fatalf("result = %+v, want remote response", saved.Result)
	}
	if saved.SubmitSnapshot == nil || saved.SubmitSnapshot.SPUName != "SPU-1" {
		t.Fatalf("submit snapshot = %+v, want SPU-1", saved.SubmitSnapshot)
	}
	if saved.RemoteRecordID != "record-1" || saved.RemoteState != 4 || saved.RemoteAuditState != 5 {
		t.Fatalf("remote fields = %+v, want record/state/audit", saved)
	}
	if pkg.SubmissionState.RemoteStatus != SubmissionRemoteStatusConfirmed {
		t.Fatalf("report remote status = %q, want confirmed", pkg.SubmissionState.RemoteStatus)
	}
}

func TestApplySubmissionPersistenceInput(t *testing.T) {
	t.Parallel()

	now := time.Now()
	pkg := &Package{
		SubmissionState: &SubmissionReport{
			Publish: &SubmissionRecord{
				Action:      "publish",
				Status:      SubmissionStatusRunning,
				SubmittedAt: now,
				RequestID:   "req-1",
			},
		},
	}

	ApplySubmissionPersistenceInput(
		pkg,
		"publish",
		"req-1",
		"SUP-1",
		&SubmissionResponse{Success: true, SPUName: "SPU-1"},
		&SubmitSnapshot{SPUName: "SPU-1"},
	)

	saved := pkg.SubmissionState.Publish
	if saved.SupplierCode != "SUP-1" {
		t.Fatalf("supplier code = %q, want SUP-1", saved.SupplierCode)
	}
	if saved.Result == nil || saved.Result.SPUName != "SPU-1" {
		t.Fatalf("result = %+v, want remote response", saved.Result)
	}
	if saved.SubmitSnapshot == nil || saved.SubmitSnapshot.SPUName != "SPU-1" {
		t.Fatalf("submit snapshot = %+v, want SPU-1", saved.SubmitSnapshot)
	}
}

func TestApplySubmissionStartFailure(t *testing.T) {
	t.Parallel()

	startedAt := time.Now().Add(-time.Minute)
	finishedAt := startedAt.Add(10 * time.Second)
	pkg := &Package{
		SubmissionState: &SubmissionReport{
			Publish: &SubmissionRecord{
				Action:      "publish",
				RequestID:   "req-1",
				Status:      SubmissionStatusRunning,
				Phase:       SubmissionPhaseSubmitRemote,
				SubmittedAt: startedAt,
				StartedAt:   startedAt,
			},
		},
	}

	record := ApplySubmissionStartFailure(pkg, "publish", "req-1", errors.New("workflow start failed"), finishedAt)
	if record == nil {
		t.Fatal("ApplySubmissionStartFailure() = nil, want failed record")
	}
	if record.Status != SubmissionStatusFailed || record.Phase != SubmissionPhaseValidate {
		t.Fatalf("record status/phase = %q/%q, want failed/validate", record.Status, record.Phase)
	}
	if record.FinishedAt == nil || !record.FinishedAt.Equal(finishedAt) {
		t.Fatalf("record finished at = %+v, want %v", record.FinishedAt, finishedAt)
	}
	if record.Error != "workflow start failed" {
		t.Fatalf("record error = %q, want workflow start failed", record.Error)
	}
	if pkg.SubmissionState.LastStatus != SubmissionStatusFailed || pkg.SubmissionState.LastError != "workflow start failed" {
		t.Fatalf("submission state = %+v, want failed last status/error", pkg.SubmissionState)
	}
}

func TestFindActiveSubmissionAttemptAndNeedsRemoteRecovery(t *testing.T) {
	t.Parallel()

	now := time.Now()
	startedAt := now.Add(-time.Minute)
	report := &SubmissionReport{
		CurrentAction:     "publish",
		CurrentPhase:      SubmissionPhaseSubmitRemote,
		CurrentRequestID:  "req-1",
		InFlightStartedAt: &startedAt,
	}
	pkg := &Package{SubmissionState: report}
	if got := FindActiveSubmissionAttempt(pkg, "publish", now, 5*time.Minute); got != report {
		t.Fatalf("FindActiveSubmissionAttempt() = %+v, want active report", got)
	}
	staleNow := now.Add(10 * time.Minute)
	if active := FindActiveSubmissionAttempt(pkg, "publish", staleNow, 5*time.Minute); active != nil {
		t.Fatalf("FindActiveSubmissionAttempt(stale) = %+v, want nil", active)
	}
	if !SubmissionNeedsRemoteRecovery(report, "publish", staleNow, 5*time.Minute) {
		t.Fatal("SubmissionNeedsRemoteRecovery() = false, want true for stale remote submit")
	}
}

func TestSubmissionLeaseNeedsRemoteRecovery(t *testing.T) {
	t.Parallel()

	now := time.Now()
	startedAt := now.Add(-time.Minute)
	pkg := &Package{
		SubmissionState: &SubmissionReport{
			CurrentAction:     "publish",
			CurrentPhase:      SubmissionPhasePrepareProduct,
			CurrentRequestID:  "req-1",
			InFlightStartedAt: &startedAt,
			Publish: &SubmissionRecord{
				Action:       "publish",
				RequestID:    "req-1",
				SupplierCode: "SUP-1",
			},
		},
	}

	if !SubmissionLeaseNeedsRemoteRecovery(pkg, "publish", "req-1", now, 5*time.Minute) {
		t.Fatal("SubmissionLeaseNeedsRemoteRecovery(non-remote phase) = false, want true")
	}

	pkg.SubmissionState.CurrentPhase = SubmissionPhaseSubmitRemote
	pkg.SubmissionState.Publish.Result = &SubmissionResponse{Success: true}
	if !SubmissionLeaseNeedsRemoteRecovery(pkg, "publish", "req-1", now, 5*time.Minute) {
		t.Fatal("SubmissionLeaseNeedsRemoteRecovery(persisted response) = false, want true")
	}

	pkg.SubmissionState.Publish.Result = nil
	staleNow := now.Add(10 * time.Minute)
	if !SubmissionLeaseNeedsRemoteRecovery(pkg, "publish", "other", staleNow, 5*time.Minute) {
		t.Fatal("SubmissionLeaseNeedsRemoteRecovery(stale remote submit) = false, want true")
	}
}

func TestCollectRemoteLookupCodesIncludesNormalizedSupplierSKUs(t *testing.T) {
	t.Parallel()

	skcSupplierCode := " MG8089003001 "
	pkg := &Package{
		PreviewPayload: &sheinproduct.Product{
			SupplierCode: " MG8089003001 ",
			SKCList: []sheinproduct.SKC{
				{
					SupplierCode: &skcSupplierCode,
					SKUS: []sheinproduct.SKU{
						{SupplierSKU: " MG8089003001-RED "},
						{SupplierSKU: "MG8089003001-BLUE"},
					},
				},
			},
		},
	}

	got := CollectRemoteLookupCodes(pkg, "MG8089003001")
	want := []string{"MG8089003001", "MG8089003001-RED", "MG8089003001-BLUE"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CollectRemoteLookupCodes() = %#v, want %#v", got, want)
	}
}

func testSubmissionRemoteFinishedAt() (finishedAt time.Time) {
	return finishedAt.Add(5)
}

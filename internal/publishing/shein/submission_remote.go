package shein

import (
	"strings"
	"time"

	listingsubmission "task-processor/internal/listing/submission"
	sheinproduct "task-processor/internal/shein/api/product"
)

func SubmissionResponseAccepted(result *SubmissionResponse) bool {
	if result == nil {
		return false
	}
	return result.Success
}

func AppendSubmissionEvent(pkg *Package, event SubmissionEvent) {
	if pkg == nil {
		return
	}
	now := time.Now()
	if event.ID == "" {
		event.ID = listingsubmission.EnsureEventID(event.ID, event.Action, now)
	}
	pkg.SubmissionEvents = listingsubmission.PrependRecentEvents(pkg.SubmissionEvents, event, 30)
}

type SubmissionConfirmRemoteUpdate struct {
	RemoteStatus string
	Record       *sheinproduct.RecordItem
	CheckedAt    time.Time
	Message      string
	Event        *SubmissionEvent
}

func BuildSubmissionAttemptEvent(taskID, action string, record *SubmissionRecord, response *SubmissionResponse, submitErr error, startedAt time.Time) SubmissionEvent {
	finishedAt := time.Now()
	var recordState *listingsubmission.EventRecordState
	var fallbackOutcome *listingsubmission.ResponseOutcome
	if record != nil {
		recordState = &listingsubmission.EventRecordState{
			Status:         record.Status,
			RequestID:      record.RequestID,
			Phase:          record.Phase,
			RemoteRecordID: record.RemoteRecordID,
		}
		fallbackOutcome = SubmissionResponseOutcome(record.Result)
	}
	draft := listingsubmission.BuildAttemptEventDraft(recordState, SubmissionResponseOutcome(response), fallbackOutcome, submitErr, finishedAt)
	event := SubmissionEvent{
		TaskID:          taskID,
		Platform:        "shein",
		Action:          action,
		Status:          draft.Status,
		RequestID:       draft.RequestID,
		Phase:           draft.Phase,
		RemoteRecordID:  draft.RemoteRecordID,
		ErrorMessage:    draft.ErrorMessage,
		ValidationNotes: draft.ValidationNotes,
		StartedAt:       startedAt,
		FinishedAt:      &draft.FinishedAt,
		Response:        response,
	}
	if event.Response == nil && record != nil {
		event.Response = record.Result
	}
	return event
}

func BuildSubmissionPhaseEvent(taskID, action, phase, status, requestID string, startedAt time.Time, detail string, err error) SubmissionEvent {
	finishedAt := time.Now()
	draft := listingsubmission.BuildPhaseEventDraft(status, detail, submissionPhaseDetail(action, phase), err, finishedAt)
	return SubmissionEvent{
		TaskID:       taskID,
		Platform:     "shein",
		Action:       "submit_phase",
		Phase:        phase,
		Status:       draft.Status,
		RequestID:    requestID,
		StartedAt:    startedAt,
		FinishedAt:   &draft.FinishedAt,
		Detail:       draft.Detail,
		ErrorMessage: draft.ErrorMessage,
	}
}

func BuildSubmissionConfirmRemoteEvent(taskID, action, status, requestID string, startedAt time.Time, detail string, err error) SubmissionEvent {
	return BuildSubmissionPhaseEvent(taskID, action, SubmissionPhaseConfirmRemote, status, requestID, startedAt, detail, err)
}

func BuildSubmissionConfirmRemoteEventForRecord(taskID, action, status, requestID string, startedAt time.Time, detail string, err error, remoteRecordID string) SubmissionEvent {
	event := BuildSubmissionConfirmRemoteEvent(taskID, action, status, requestID, startedAt, detail, err)
	event.RemoteRecordID = remoteRecordID
	return event
}

func BuildSubmissionConfirmRemoteUpdateWithEvent(remoteStatus string, record *sheinproduct.RecordItem, checkedAt time.Time, message string, event *SubmissionEvent) (SubmissionConfirmRemoteUpdate, bool) {
	if event == nil {
		return SubmissionConfirmRemoteUpdate{}, false
	}
	recordRemoteRecordID := ""
	if record != nil {
		recordRemoteRecordID = record.RecordID
	}
	state := listingsubmission.BuildConfirmRemoteState(message, event.RemoteRecordID, recordRemoteRecordID, checkedAt)
	copyEvent := *event
	copyEvent.RemoteRecordID = state.EventRemoteRecordID
	return SubmissionConfirmRemoteUpdate{
		RemoteStatus: remoteStatus,
		Record:       record,
		CheckedAt:    state.CheckedAt,
		Message:      state.Message,
		Event:        &copyEvent,
	}, true
}

func BuildSubmissionConfirmRemoteUpdate(taskID, action, status, requestID string, startedAt time.Time, detail string, err error) SubmissionConfirmRemoteUpdate {
	state := listingsubmission.BuildConfirmRemoteState(detail, "", "", time.Now())
	event := BuildSubmissionConfirmRemoteEvent(taskID, action, status, requestID, startedAt, detail, err)
	event.RemoteRecordID = state.EventRemoteRecordID
	return SubmissionConfirmRemoteUpdate{
		RemoteStatus: status,
		CheckedAt:    state.CheckedAt,
		Message:      state.Message,
		Event:        &event,
	}
}

func BuildSubmissionConfirmRemoteUpdateForRecord(taskID, action, status, requestID string, startedAt time.Time, detail string, err error, record *sheinproduct.RecordItem) SubmissionConfirmRemoteUpdate {
	update := BuildSubmissionConfirmRemoteUpdate(taskID, action, status, requestID, startedAt, detail, err)
	update.Record = record
	recordRemoteRecordID := ""
	if record != nil {
		recordRemoteRecordID = record.RecordID
	}
	state := listingsubmission.BuildConfirmRemoteState(update.Message, update.Event.RemoteRecordID, recordRemoteRecordID, update.CheckedAt)
	update.CheckedAt = state.CheckedAt
	update.Message = state.Message
	update.Event.RemoteRecordID = state.EventRemoteRecordID
	return update
}

func BuildSubmissionRefreshConfirmRemoteRunningEvent(taskID, action, requestID string, startedAt time.Time) SubmissionEvent {
	return BuildSubmissionConfirmRemoteEvent(taskID, action, SubmissionStatusRunning, requestID, startedAt, "刷新 SHEIN 远端提交状态", nil)
}

func ApplySubmissionConfirmRemoteWithEvent(pkg *Package, action, requestID, remoteStatus string, record *sheinproduct.RecordItem, checkedAt time.Time, message string, event *SubmissionEvent) bool {
	update, ok := BuildSubmissionConfirmRemoteUpdateWithEvent(remoteStatus, record, checkedAt, message, event)
	if !ok {
		return false
	}
	ApplySubmissionConfirmRemoteUpdate(pkg, action, requestID, update)
	return true
}

func ApplySubmissionConfirmRemoteUpdate(pkg *Package, action, requestID string, update SubmissionConfirmRemoteUpdate) {
	if pkg == nil {
		return
	}
	recordRemoteRecordID := ""
	if update.Record != nil {
		recordRemoteRecordID = update.Record.RecordID
	}
	state := listingsubmission.BuildConfirmRemoteState(update.Message, "", recordRemoteRecordID, update.CheckedAt)
	if update.Event != nil {
		state = listingsubmission.BuildConfirmRemoteState(update.Message, update.Event.RemoteRecordID, recordRemoteRecordID, update.CheckedAt)
		copyEvent := *update.Event
		copyEvent.RemoteRecordID = state.EventRemoteRecordID
		AppendSubmissionEvent(pkg, copyEvent)
	}
	SetSubmissionRemoteRecord(pkg, action, requestID, update.RemoteStatus, update.Record, state.CheckedAt, state.Message)
}

func SubmissionResponseAcceptedForAction(action string, result *SubmissionResponse) bool {
	if SubmissionResponseAccepted(result) {
		return true
	}
	return listingsubmission.SaveDraftSucceeded(action, SubmissionResponseOutcome(result))
}

func SubmissionResponseAcceptedWithSPU(result *SubmissionResponse) bool {
	if !SubmissionResponseAccepted(result) {
		return false
	}
	return strings.TrimSpace(result.SPUName) != ""
}

func ConfirmedSubmissionResponse(response *SubmissionResponse, action string) *SubmissionResponse {
	if response != nil {
		return response
	}
	if strings.TrimSpace(action) == "save_draft" {
		return &SubmissionResponse{Code: "0", Success: true, Message: "save draft confirmed by remote check"}
	}
	return &SubmissionResponse{Code: "0", Success: true, Message: "publish confirmed by remote check"}
}

func SubmissionStartedAt(pkg *Package, action, requestID string, fallback time.Time) time.Time {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.SubmissionState == nil {
		return fallback
	}
	record := SubmissionRecordForAction(pkg.SubmissionState, action)
	if record != nil && record.RequestID == requestID && !record.StartedAt.IsZero() {
		return record.StartedAt
	}
	if pkg.SubmissionState.InFlightStartedAt != nil {
		return *pkg.SubmissionState.InFlightStartedAt
	}
	return fallback
}

func SubmissionResponseForAction(pkg *Package, action string) *SubmissionResponse {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.SubmissionState == nil {
		return nil
	}
	record := SubmissionRecordForAction(pkg.SubmissionState, action)
	if record != nil && record.Result != nil {
		return record.Result
	}
	return pkg.SubmissionState.LastResult
}

func SubmissionStatePackage(pkg *Package) (*Package, bool) {
	pkg = NormalizePackageSemanticFields(pkg)
	return pkg, pkg != nil && pkg.SubmissionState != nil
}

func PreviewPayloadPackage(pkg *Package) (*Package, bool) {
	pkg = NormalizePackageSemanticFields(pkg)
	return pkg, pkg != nil && pkg.PreviewPayload != nil
}

type SubmissionRefreshSelection struct {
	Action       string
	Record       *SubmissionRecord
	SupplierCode string
}

type SubmissionRecoverySelection struct {
	Report    *SubmissionReport
	Record    *SubmissionRecord
	RequestID string
	Response  *SubmissionResponse
}

type SubmissionRemoteRefreshSelection struct {
	StartedAt    time.Time
	Response     *SubmissionResponse
	RemoteStatus string
}

func ResolveSubmissionRefreshSelection(pkg *Package) SubmissionRefreshSelection {
	var ok bool
	pkg, ok = SubmissionStatePackage(pkg)
	if !ok {
		return SubmissionRefreshSelection{}
	}
	action := listingsubmission.ResolveRefreshAction(
		pkg.SubmissionState.LastAction,
		pkg.SubmissionState.Publish != nil,
		pkg.SubmissionState.SaveDraft != nil,
	)
	record := SubmissionRecordForAction(pkg.SubmissionState, action)
	recordSupplierCode := ""
	if record != nil {
		recordSupplierCode = record.SupplierCode
	}
	return SubmissionRefreshSelection{
		Action:       action,
		Record:       record,
		SupplierCode: listingsubmission.ResolveRefreshSupplierCode(recordSupplierCode, submissionPackageSupplierCode(pkg)),
	}
}

func ResolveSubmissionRecoverySelection(pkg *Package, action string) SubmissionRecoverySelection {
	pkg, ok := SubmissionStatePackage(pkg)
	if !ok {
		return SubmissionRecoverySelection{}
	}
	report := pkg.SubmissionState
	record := SubmissionRecordForAction(report, action)
	return SubmissionRecoverySelection{
		Report:    report,
		Record:    record,
		RequestID: report.CurrentRequestID,
		Response:  SubmissionResponseForAction(pkg, action),
	}
}

func ResolveSubmissionRemoteRefreshSelection(pkg *Package, action, requestID string, fallbackStartedAt time.Time) SubmissionRemoteRefreshSelection {
	pkg, ok := SubmissionStatePackage(pkg)
	if !ok {
		return SubmissionRemoteRefreshSelection{StartedAt: fallbackStartedAt}
	}
	return SubmissionRemoteRefreshSelection{
		StartedAt:    SubmissionStartedAt(pkg, action, requestID, fallbackStartedAt),
		Response:     SubmissionResponseForAction(pkg, action),
		RemoteStatus: pkg.SubmissionState.RemoteStatus,
	}
}

func SubmissionRefreshActionMatches(pkg *Package, requestedAction string) bool {
	return listingsubmission.RefreshActionMatches(ResolveSubmissionRefreshSelection(pkg).Action, requestedAction)
}

func SubmissionRefreshRequestMatches(pkg *Package, action, requestedRequestID string) bool {
	var ok bool
	pkg, ok = SubmissionStatePackage(pkg)
	if !ok {
		return false
	}
	record := SubmissionRecordForAction(pkg.SubmissionState, action)
	if record == nil {
		return false
	}
	return listingsubmission.RefreshRequestMatches(record.RequestID, requestedRequestID)
}

func SubmissionRecordResult(record *SubmissionRecord) *SubmissionResponse {
	if record == nil {
		return nil
	}
	return record.Result
}

func RemoteLookupSPUName(pkg *Package, action string) string {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.SubmissionState == nil {
		return ""
	}
	record := RemoteRecordForAction(pkg.SubmissionState, action)
	if record != nil && record.Result != nil {
		if value := strings.TrimSpace(record.Result.SPUName); value != "" {
			return value
		}
	}
	if pkg.SubmissionState.LastResult != nil {
		if value := strings.TrimSpace(pkg.SubmissionState.LastResult.SPUName); value != "" {
			return value
		}
	}
	return ""
}

func RemotePublishAccepted(pkg *Package, action string) bool {
	pkg = NormalizePackageSemanticFields(pkg)
	if action != "publish" || pkg == nil || pkg.SubmissionState == nil {
		return false
	}
	record := RemoteRecordForAction(pkg.SubmissionState, action)
	if SubmissionResponseAcceptedWithSPU(SubmissionRecordResult(record)) {
		return true
	}
	return SubmissionResponseAcceptedWithSPU(pkg.SubmissionState.LastResult)
}

func CollectRemoteLookupCodes(pkg *Package, supplierCode string) []string {
	pkg = NormalizePackageSemanticFields(pkg)
	seen := make(map[string]struct{})
	codes := make([]string, 0, 8)
	appendCode := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		codes = append(codes, value)
	}

	appendCode(supplierCode)
	if pkg == nil || pkg.PreviewPayload == nil {
		return codes
	}
	appendCode(pkg.PreviewPayload.SupplierCode)
	for _, skc := range pkg.PreviewPayload.SKCList {
		if skc.SupplierCode != nil {
			appendCode(*skc.SupplierCode)
		}
		for _, sku := range skc.SKUS {
			appendCode(sku.SupplierSKU)
		}
	}
	return codes
}

func submissionPackageSupplierCode(pkg *Package) string {
	if pkg == nil {
		return ""
	}
	for _, skc := range pkg.SkcList {
		if value := strings.TrimSpace(skc.SupplierCode); value != "" {
			return value
		}
	}
	return ""
}

func EnsureSubmissionReport(pkg *Package) *SubmissionReport {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg.SubmissionState == nil {
		pkg.SubmissionState = &SubmissionReport{}
	}
	return pkg.SubmissionState
}

func SubmissionRecordForAction(report *SubmissionReport, action string) *SubmissionRecord {
	if report == nil {
		return nil
	}
	switch strings.TrimSpace(action) {
	case "save_draft":
		return report.SaveDraft
	case "publish":
		return report.Publish
	default:
		return nil
	}
}

func SubmissionInFlightState(report *SubmissionReport) listingsubmission.InFlightState {
	if report == nil {
		return listingsubmission.InFlightState{}
	}
	return listingsubmission.InFlightState{
		AttemptCount:      report.AttemptCount,
		CurrentAction:     report.CurrentAction,
		CurrentPhase:      report.CurrentPhase,
		CurrentRequestID:  report.CurrentRequestID,
		InFlightStartedAt: report.InFlightStartedAt,
		LeaseExpiresAt:    report.LeaseExpiresAt,
	}
}

func ApplySubmissionInFlightState(report *SubmissionReport, state listingsubmission.InFlightState) {
	if report == nil {
		return
	}
	report.AttemptCount = state.AttemptCount
	report.CurrentAction = state.CurrentAction
	report.CurrentPhase = state.CurrentPhase
	report.CurrentRequestID = state.CurrentRequestID
	report.InFlightStartedAt = state.InFlightStartedAt
	report.LeaseExpiresAt = state.LeaseExpiresAt
}

func SetSubmissionRecordPhase(report *SubmissionReport, action, requestID, phase string) bool {
	if report == nil {
		return false
	}
	return listingsubmission.MutateMatchingRecord(
		submissionActionRecordSlots(report),
		action,
		requestID,
		submissionActionRecordView,
		func(record *SubmissionRecord) {
			record.Phase = phase
		},
	)
}

func RemoteRecordForAction(report *SubmissionReport, action string) *SubmissionRecord {
	return SubmissionRecordForAction(report, action)
}

func FindCompletedSubmissionRecordByRequestID(pkg *Package, action, requestID string) *SubmissionRecord {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.SubmissionState == nil || strings.TrimSpace(requestID) == "" {
		return nil
	}
	record := SubmissionRecordForAction(pkg.SubmissionState, action)
	if record == nil {
		return nil
	}
	if strings.TrimSpace(record.RequestID) != strings.TrimSpace(requestID) {
		return nil
	}
	if record.FinishedAt == nil {
		return nil
	}
	return record
}

func SubmissionRemoteResponsePersisted(pkg *Package, action, requestID string) bool {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.SubmissionState == nil {
		return false
	}
	record := SubmissionRecordForAction(pkg.SubmissionState, action)
	if record == nil || record.RequestID != requestID {
		return false
	}
	return record.Result != nil
}

func SubmissionSucceeded(pkg *Package, action string) bool {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.SubmissionState == nil {
		return false
	}
	record := SubmissionRecordForAction(pkg.SubmissionState, action)
	if record == nil || record.FinishedAt == nil {
		return false
	}
	return record.Status == SubmissionStatusSuccess
}

func ClearSubmissionInFlight(report *SubmissionReport, action, requestID string) {
	if report == nil {
		return
	}
	if !listingsubmission.ShouldClearInFlight(report.CurrentAction, report.CurrentRequestID, action, requestID) {
		return
	}
	report.CurrentAction = ""
	report.CurrentPhase = ""
	report.CurrentRequestID = ""
	report.InFlightStartedAt = nil
	report.LeaseExpiresAt = nil
}

func ApplySubmissionRecord(pkg *Package, record *SubmissionRecord) {
	if pkg == nil || record == nil {
		return
	}
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg.SubmissionState == nil {
		pkg.SubmissionState = &SubmissionReport{}
	}
	state := listingsubmission.ReportState[SubmissionRecord, SubmissionResponse]{
		LastAction:  pkg.SubmissionState.LastAction,
		LastStatus:  pkg.SubmissionState.LastStatus,
		LastError:   pkg.SubmissionState.LastError,
		SubmittedAt: pkg.SubmissionState.SubmittedAt,
		LastResult:  pkg.SubmissionState.LastResult,
		Slots: listingsubmission.ActionRecordSlots[SubmissionRecord]{
			SaveDraft: pkg.SubmissionState.SaveDraft,
			Publish:   pkg.SubmissionState.Publish,
		},
	}
	listingsubmission.ApplyRecordState(&state, record, listingsubmission.ReportRecordState[SubmissionResponse]{
		Action:      record.Action,
		Status:      record.Status,
		Error:       record.Error,
		SubmittedAt: record.SubmittedAt,
		Result:      record.Result,
	})
	pkg.SubmissionState.LastAction = state.LastAction
	pkg.SubmissionState.LastStatus = state.LastStatus
	pkg.SubmissionState.LastError = state.LastError
	pkg.SubmissionState.SubmittedAt = state.SubmittedAt
	pkg.SubmissionState.LastResult = state.LastResult
	pkg.SubmissionState.SaveDraft = state.Slots.SaveDraft
	pkg.SubmissionState.Publish = state.Slots.Publish
}

func ResolveSubmissionAttemptRecord(report *SubmissionReport, action, requestID string, seedState listingsubmission.AttemptSeedState, finishedAt time.Time) *SubmissionRecord {
	if report == nil {
		return nil
	}
	return listingsubmission.ResolveAttemptRecordForRequest(
		submissionActionRecordSlots(report),
		action,
		requestID,
		submissionActionRecordView,
		BuildSubmissionAttemptRecordFromSeed,
		seedState,
		finishedAt,
	)
}

func BuildSubmissionRunningRecord(action, requestID, phase string, startedAt time.Time, attempt int) *SubmissionRecord {
	return &SubmissionRecord{
		Action:      action,
		Status:      SubmissionStatusRunning,
		SubmittedAt: startedAt,
		RequestID:   requestID,
		Phase:       phase,
		StartedAt:   startedAt,
		Attempt:     attempt,
	}
}

func BuildSubmissionAttemptRecordFromSeed(seed listingsubmission.AttemptRecordSeed) *SubmissionRecord {
	return &SubmissionRecord{
		Action:      seed.Action,
		SubmittedAt: seed.SubmittedAt,
		RequestID:   seed.RequestID,
		StartedAt:   seed.StartedAt,
		Attempt:     seed.Attempt,
	}
}

func ApplySubmissionAttemptFinalizeState(record *SubmissionRecord, response *SubmissionResponse, state listingsubmission.AttemptFinalizeState) {
	if record == nil {
		return
	}
	record.Result = response
	record.FinishedAt = &state.FinishedAt
	record.Status = state.Status
	record.Error = state.ErrorMessage
}

func ApplySubmissionAttemptFailureState(record *SubmissionRecord, phase string, state listingsubmission.AttemptFinalizeState) {
	if record == nil {
		return
	}
	record.Status = state.Status
	record.Phase = phase
	record.FinishedAt = &state.FinishedAt
	record.Error = state.ErrorMessage
}

func ApplySubmissionStartFailure(pkg *Package, action, requestID string, startErr error, finishedAt time.Time) *SubmissionRecord {
	pkg, ok := SubmissionStatePackage(pkg)
	if !ok {
		return nil
	}
	record := SubmissionRecordForAction(pkg.SubmissionState, action)
	if record == nil || record.RequestID != requestID || record.Status != SubmissionStatusRunning {
		return nil
	}
	record.Status = SubmissionStatusFailed
	record.Phase = SubmissionPhaseValidate
	record.FinishedAt = &finishedAt
	if startErr != nil {
		record.Error = startErr.Error()
	}
	ApplySubmissionRecord(pkg, record)
	return record
}

func ApplySubmissionPersistenceInput(pkg *Package, action, requestID, supplierCode string, response *SubmissionResponse, snapshot *SubmitSnapshot) {
	if snapshot != nil {
		SetSubmissionSnapshot(pkg, action, requestID, snapshot)
	}
	if supplierCode != "" {
		SetSubmissionSupplierCode(pkg, action, requestID, supplierCode)
	}
	if response != nil {
		SetSubmissionRemoteResponse(pkg, action, requestID, supplierCode, response)
	}
}

func SetSubmissionSupplierCode(pkg *Package, action, requestID, supplierCode string) {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.SubmissionState == nil || supplierCode == "" {
		return
	}
	listingsubmission.MutateMatchingRecord(
		submissionActionRecordSlots(pkg.SubmissionState),
		action,
		requestID,
		submissionActionRecordView,
		func(record *SubmissionRecord) {
			record.SupplierCode = supplierCode
		},
	)
}

func SetSubmissionRemoteResponse(pkg *Package, action, requestID, supplierCode string, response *SubmissionResponse) {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.SubmissionState == nil {
		return
	}
	if listingsubmission.MutateMatchingRecord(
		submissionActionRecordSlots(pkg.SubmissionState),
		action,
		requestID,
		submissionActionRecordView,
		func(record *SubmissionRecord) {
			if supplierCode != "" {
				record.SupplierCode = supplierCode
			}
			record.Result = response
		},
	) {
		pkg.SubmissionState.LastResult = response
	}
}

func SetSubmissionSnapshot(pkg *Package, action, requestID string, snapshot *SubmitSnapshot) {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.SubmissionState == nil || snapshot == nil {
		return
	}
	listingsubmission.MutateMatchingRecord(
		submissionActionRecordSlots(pkg.SubmissionState),
		action,
		requestID,
		submissionActionRecordView,
		func(record *SubmissionRecord) {
			record.SubmitSnapshot = snapshot
		},
	)
}

func SetSubmissionRemoteRecord(pkg *Package, action, requestID, remoteStatus string, item *sheinproduct.RecordItem, checkedAt time.Time, message string) {
	if pkg == nil {
		return
	}
	report := EnsureSubmissionReport(pkg)
	listingsubmission.ApplyRemoteSync(
		submissionActionRecordSlots(report),
		action,
		requestID,
		submissionActionRecordView,
		listingsubmission.ActionRemoteSyncState{
			RemoteStatus: remoteStatus,
			CheckedAt:    checkedAt,
		},
		func(state listingsubmission.ActionRemoteSyncState) {
			report.RemoteStatus = state.RemoteStatus
			report.RemoteCheckedAt = &state.CheckedAt
		},
		func(record *SubmissionRecord, state listingsubmission.ActionRemoteSyncState) {
			record.RemoteCheckedAt = &state.CheckedAt
			record.RemoteMessage = message
			if item != nil {
				record.RemoteRecordID = item.RecordID
				record.RemoteState = item.State
				record.RemoteAuditState = item.AuditState
				if record.SupplierCode == "" {
					record.SupplierCode = item.SupplierCode
				}
			}
		},
	)
}

var remoteRecoveryPhases = map[string]struct{}{
	SubmissionPhaseSubmitRemote:  {},
	SubmissionPhasePersistResult: {},
	SubmissionPhaseConfirmRemote: {},
}

func submissionActionRecordSlots(report *SubmissionReport) listingsubmission.ActionRecordSlots[SubmissionRecord] {
	if report == nil {
		return listingsubmission.ActionRecordSlots[SubmissionRecord]{}
	}
	return listingsubmission.ActionRecordSlots[SubmissionRecord]{
		SaveDraft: report.SaveDraft,
		Publish:   report.Publish,
	}
}

func submissionActionRecordView(record *SubmissionRecord) listingsubmission.ActionRecordView {
	return listingsubmission.ActionRecordView{RequestID: record.RequestID}
}

func FindActiveSubmissionAttempt(pkg *Package, action string, now time.Time, ttl time.Duration) *SubmissionReport {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.SubmissionState == nil {
		return nil
	}
	report := pkg.SubmissionState
	if !listingsubmission.IsActiveAttempt(listingsubmission.RecoveryLeaseState{
		CurrentAction:     report.CurrentAction,
		CurrentRequestID:  report.CurrentRequestID,
		CurrentPhase:      report.CurrentPhase,
		InFlightStartedAt: report.InFlightStartedAt,
		LeaseExpiresAt:    report.LeaseExpiresAt,
	}, action, now, ttl) {
		return nil
	}
	return report
}

func SubmissionLeaseNeedsRemoteRecovery(pkg *Package, action, requestID string, now time.Time, ttl time.Duration) bool {
	pkg, ok := SubmissionStatePackage(pkg)
	if !ok {
		return false
	}
	report := pkg.SubmissionState
	sameRequestNeedsRecovery := report.CurrentRequestID == requestID &&
		(report.CurrentPhase != SubmissionPhaseSubmitRemote ||
			SubmissionRemoteResponsePersisted(pkg, action, requestID))
	return sameRequestNeedsRecovery || SubmissionNeedsRemoteRecovery(report, action, now, ttl)
}

func SubmissionNeedsRemoteRecovery(report *SubmissionReport, action string, now time.Time, ttl time.Duration) bool {
	if report == nil {
		return false
	}
	return listingsubmission.NeedsRemoteRecovery(listingsubmission.RecoveryLeaseState{
		CurrentAction:     report.CurrentAction,
		CurrentRequestID:  report.CurrentRequestID,
		CurrentPhase:      report.CurrentPhase,
		InFlightStartedAt: report.InFlightStartedAt,
		LeaseExpiresAt:    report.LeaseExpiresAt,
	}, action, now, ttl, remoteRecoveryPhases)
}

func SubmissionResponseOutcome(result *SubmissionResponse) *listingsubmission.ResponseOutcome {
	if result == nil {
		return nil
	}
	return &listingsubmission.ResponseOutcome{
		Success:         result.Success,
		Code:            result.Code,
		Message:         result.Message,
		ValidationNotes: append([]string(nil), result.ValidationNotes...),
	}
}

func submissionPhaseDetail(action, phase string) string {
	return listingsubmission.PhaseDetail(action, phase, submissionPhaseDetailLabels)
}

var submissionPhaseDetailLabels = listingsubmission.PhaseDetailLabels{
	Validate:        "检查 SHEIN 提交前状态",
	PrepareProduct:  "准备 SHEIN 商品载荷",
	UploadImages:    "上传 SHEIN 商品图片",
	PreValidate:     "执行 SHEIN 提交前校验",
	SubmitRemote:    "提交 SHEIN 发布请求",
	SaveDraftRemote: "提交 SHEIN 草稿",
	PersistResult:   "保存本地提交结果",
	ConfirmRemote:   "刷新 SHEIN 远端诊断状态",
}

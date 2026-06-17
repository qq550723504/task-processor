package shein

import (
	"strings"
	"time"

	listingsubmission "task-processor/internal/listing/submission"
	sheinmarketpub "task-processor/internal/marketplace/shein/publishing"
	sheinother "task-processor/internal/shein/api/other"
	sheinproduct "task-processor/internal/shein/api/product"
)

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
	draft := listingsubmission.BuildPhaseEventDraft(status, detail, sheinmarketpub.SubmissionPhaseDetail(action, phase), err, finishedAt)
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

func applySubmissionConfirmRemoteState(pkg *Package, action, requestID string, update SubmissionConfirmRemoteUpdate) (*SubmissionEvent, bool) {
	if pkg == nil {
		return nil, false
	}
	recordRemoteRecordID := ""
	if update.Record != nil {
		recordRemoteRecordID = update.Record.RecordID
	}
	state := listingsubmission.BuildConfirmRemoteState(update.Message, "", recordRemoteRecordID, update.CheckedAt)
	if update.Event != nil {
		state = listingsubmission.BuildConfirmRemoteState(update.Message, update.Event.RemoteRecordID, recordRemoteRecordID, update.CheckedAt)
	}
	SetSubmissionRemoteRecord(pkg, action, requestID, update.RemoteStatus, update.Record, state.CheckedAt, state.Message)
	if update.Event == nil {
		return nil, true
	}
	copyEvent := *update.Event
	copyEvent.RemoteRecordID = state.EventRemoteRecordID
	return &copyEvent, true
}

func ApplySubmissionConfirmRemoteState(pkg *Package, action, requestID string, update SubmissionConfirmRemoteUpdate) {
	_, _ = applySubmissionConfirmRemoteState(pkg, action, requestID, update)
}

func ApplySubmissionConfirmRemoteUpdate(pkg *Package, action, requestID string, update SubmissionConfirmRemoteUpdate) {
	event, ok := applySubmissionConfirmRemoteState(pkg, action, requestID, update)
	if !ok || event == nil {
		return
	}
	AppendSubmissionEvent(pkg, *event)
}

func SubmissionStartedAt(pkg *Package, action, requestID string, fallback time.Time) time.Time {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.SubmissionState == nil {
		return fallback
	}
	return listingsubmission.ResolveRecordStartedAt(
		submissionActionRecordSlots(pkg.SubmissionState),
		action,
		requestID,
		submissionActionRecordView,
		func(record *SubmissionRecord) time.Time {
			return record.StartedAt
		},
		pkg.SubmissionState.InFlightStartedAt,
		fallback,
	)
}

func SubmissionResponseForAction(pkg *Package, action string) *SubmissionResponse {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.SubmissionState == nil {
		return nil
	}
	return listingsubmission.ResolveActionResult(
		&listingsubmission.ReportState[SubmissionRecord, SubmissionResponse]{
			LastResult: pkg.SubmissionState.LastResult,
			Slots:      submissionActionRecordSlots(pkg.SubmissionState),
		},
		action,
		func(record *SubmissionRecord) *SubmissionResponse {
			return record.Result
		},
	)
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

type SubmissionRemoteLookupInputs struct {
	LookupCodes      []string
	SPUName          string
	DefaultConfirmed bool
	FallbackMessage  string
}

type SubmissionRefreshRequest struct {
	Action       string
	RequestID    string
	RemoteInputs SubmissionRemoteLookupInputs
}

type SubmissionRemoteResolution struct {
	OnWayDocument      *sheinmarketpub.OnWayDocument
	Record             *sheinproduct.RecordItem
	RecordErr          error
	InventoryConfirmed bool
	SPUName            string
	DefaultConfirmed   bool
	FallbackMessage    string
}

type SubmissionBatchCheckOnWayLogger func(expectedSPUName string, resp *sheinother.BatchCheckOnWayResponse, err error)

type SubmissionRecoverySelection struct {
	Report       *SubmissionReport
	Record       *SubmissionRecord
	SupplierCode string
	RequestID    string
	Response     *SubmissionResponse
	StartedAt    time.Time
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
	startedAt := time.Time{}
	supplierCode := ""
	if record != nil {
		startedAt = record.StartedAt
		supplierCode = record.SupplierCode
	}
	return SubmissionRecoverySelection{
		Report:       report,
		Record:       record,
		SupplierCode: supplierCode,
		RequestID:    report.CurrentRequestID,
		Response:     SubmissionResponseForAction(pkg, action),
		StartedAt:    startedAt,
	}
}

func BuildSubmissionRemoteLookupInputs(pkg *Package, action, supplierCode string, defaultConfirmed bool, fallbackMessage string) SubmissionRemoteLookupInputs {
	return SubmissionRemoteLookupInputs{
		LookupCodes:      CollectRemoteLookupCodes(pkg, supplierCode),
		SPUName:          RemoteLookupSPUName(pkg, action),
		DefaultConfirmed: defaultConfirmed,
		FallbackMessage:  fallbackMessage,
	}
}

func ProbeSubmissionRemoteResolution(
	productAPI sheinproduct.ProductAPI,
	otherAPI sheinother.OtherAPI,
	action string,
	lookupCodes []string,
	spuName string,
	defaultConfirmed bool,
	fallbackMessage string,
	onWayLogger SubmissionBatchCheckOnWayLogger,
) SubmissionRemoteResolution {
	resolution := SubmissionRemoteResolution{
		DefaultConfirmed: defaultConfirmed,
		FallbackMessage:  fallbackMessage,
		SPUName:          spuName,
	}
	if action == "publish" {
		if onWay, onWayErr := lookupSubmissionRemoteOnWayDocument(otherAPI, spuName, onWayLogger); onWayErr == nil && onWay != nil {
			resolution.OnWayDocument = onWay
			return resolution
		}
	}
	record, recordErr := lookupSubmissionRemoteRecord(productAPI, lookupCodes, spuName)
	resolution.Record = record
	resolution.RecordErr = recordErr
	if action == "publish" && strings.TrimSpace(spuName) != "" {
		if inventoryConfirmed, inventoryErr := lookupSubmissionRemoteInventory(productAPI, spuName); inventoryErr == nil && inventoryConfirmed {
			resolution.InventoryConfirmed = true
		}
	}
	return resolution
}

func BuildSubmissionMissingSupplierCodeRemoteUpdate(taskID, action, requestID string, startedAt time.Time, defaultConfirmed bool) SubmissionConfirmRemoteUpdate {
	decision := sheinmarketpub.BuildMissingSupplierCodeDecision(action, defaultConfirmed)
	return BuildSubmissionConfirmRemoteUpdate(taskID, action, decision.Status, requestID, startedAt, decision.Detail, nil)
}

func ApplySubmissionMissingSupplierCodeRemoteUpdate(pkg *Package, taskID, action, requestID string, startedAt time.Time, defaultConfirmed bool) *SubmissionEvent {
	if pkg == nil {
		return nil
	}
	update := BuildSubmissionMissingSupplierCodeRemoteUpdate(taskID, action, requestID, startedAt, defaultConfirmed)
	applySubmissionConfirmRemoteState(pkg, action, requestID, update)
	return update.Event
}

func lookupSubmissionRemoteOnWayDocument(otherAPI sheinother.OtherAPI, expectedSPUName string, logger SubmissionBatchCheckOnWayLogger) (*sheinmarketpub.OnWayDocument, error) {
	if otherAPI == nil {
		return nil, nil
	}
	expectedSPUName = strings.TrimSpace(expectedSPUName)
	if expectedSPUName == "" {
		return nil, nil
	}
	resp, err := otherAPI.BatchCheckOnWay([]string{expectedSPUName})
	if logger != nil {
		logger(expectedSPUName, resp, err)
	}
	if err != nil {
		return nil, err
	}
	return sheinmarketpub.SelectOnWayDocumentFromResponse(resp, expectedSPUName), nil
}

func lookupSubmissionRemoteRecord(productAPI sheinproduct.ProductAPI, codes []string, expectedSPUName string) (*sheinproduct.RecordItem, error) {
	if productAPI == nil || len(codes) == 0 {
		return nil, nil
	}
	resp, err := productAPI.Record(&sheinproduct.ProductRecordRequest{
		Language:                  "en",
		OnlyCurrentMonthRecommend: false,
		OnlySpmbCopyProduct:       false,
		QueryTimeOut:              false,
		SearchDiyCustom:           false,
		SupplierCodeList:          &codes,
		SupplierCodeSearchType:    1,
	})
	if err != nil {
		return nil, err
	}
	return sheinmarketpub.SelectRemoteRecordFromResponse(resp, expectedSPUName)
}

func lookupSubmissionRemoteInventory(productAPI sheinproduct.ProductAPI, spuName string) (bool, error) {
	if productAPI == nil {
		return false, nil
	}
	spuName = strings.TrimSpace(spuName)
	if spuName == "" {
		return false, nil
	}
	resp, err := productAPI.QueryInventory(spuName)
	if err != nil {
		return false, err
	}
	return sheinmarketpub.InventoryConfirmed(resp), nil
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

func RemoteLookupSPUName(pkg *Package, action string) string {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.SubmissionState == nil {
		return ""
	}
	recordSPUName := ""
	record := RemoteRecordForAction(pkg.SubmissionState, action)
	if record != nil && record.Result != nil {
		recordSPUName = record.Result.SPUName
	}
	lastSPUName := ""
	if pkg.SubmissionState.LastResult != nil {
		lastSPUName = pkg.SubmissionState.LastResult.SPUName
	}
	return sheinmarketpub.ResolveRemoteLookupSPUName(recordSPUName, lastSPUName)
}

func RemotePublishAccepted(pkg *Package, action string) bool {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.SubmissionState == nil {
		return false
	}
	record := RemoteRecordForAction(pkg.SubmissionState, action)
	recordAccepted := false
	if record != nil && record.Result != nil {
		result := record.Result
		recordAccepted = sheinmarketpub.ResponseAcceptedWithSPU(result.Success, result.SPUName)
	}
	lastAccepted := false
	if result := pkg.SubmissionState.LastResult; result != nil {
		lastAccepted = sheinmarketpub.ResponseAcceptedWithSPU(result.Success, result.SPUName)
	}
	return sheinmarketpub.RemotePublishAccepted(action, recordAccepted, lastAccepted)
}

func CollectRemoteLookupCodes(pkg *Package, supplierCode string) []string {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.PreviewPayload == nil {
		return sheinmarketpub.CollectRemoteLookupCodes(supplierCode, "", nil, nil)
	}
	skcSupplierCodes := make([]string, 0, len(pkg.PreviewPayload.SKCList))
	skuSupplierCodes := make([]string, 0, len(pkg.PreviewPayload.SKCList)*2)
	for _, skc := range pkg.PreviewPayload.SKCList {
		if skc.SupplierCode != nil {
			skcSupplierCodes = append(skcSupplierCodes, *skc.SupplierCode)
		}
		for _, sku := range skc.SKUS {
			skuSupplierCodes = append(skuSupplierCodes, sku.SupplierSKU)
		}
	}
	return sheinmarketpub.CollectRemoteLookupCodes(supplierCode, pkg.PreviewPayload.SupplierCode, skcSupplierCodes, skuSupplierCodes)
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
	return listingsubmission.RecordForAction(submissionActionRecordSlots(report), action)
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
	if pkg == nil || pkg.SubmissionState == nil {
		return nil
	}
	return listingsubmission.FindCompletedRecordByRequestID(
		submissionActionRecordSlots(pkg.SubmissionState),
		action,
		requestID,
		submissionActionRecordView,
	)
}

func SubmissionRemoteResponsePersisted(pkg *Package, action, requestID string) bool {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.SubmissionState == nil {
		return false
	}
	record := listingsubmission.FindRecordByRequestID(
		submissionActionRecordSlots(pkg.SubmissionState),
		action,
		requestID,
		submissionActionRecordView,
	)
	if record == nil {
		return false
	}
	return record.Result != nil
}

func SubmissionSucceeded(pkg *Package, action string) bool {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.SubmissionState == nil {
		return false
	}
	return listingsubmission.ActionSucceeded(
		submissionActionRecordSlots(pkg.SubmissionState),
		action,
		submissionActionRecordView,
		SubmissionStatusSuccess,
	)
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
	record := listingsubmission.FindRecordByRequestIDAndStatus(
		submissionActionRecordSlots(pkg.SubmissionState),
		action,
		requestID,
		submissionActionRecordView,
		SubmissionStatusRunning,
	)
	if record == nil {
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
	if record == nil {
		return listingsubmission.ActionRecordView{}
	}
	return listingsubmission.ActionRecordView{
		RequestID:  record.RequestID,
		Status:     record.Status,
		FinishedAt: record.FinishedAt,
	}
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
	sameRequestNeedsRecovery := listingsubmission.NeedsRequestScopedRemoteRecovery(
		report.CurrentRequestID,
		report.CurrentPhase,
		requestID,
		SubmissionPhaseSubmitRemote,
		SubmissionRemoteResponsePersisted(pkg, action, requestID),
	)
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

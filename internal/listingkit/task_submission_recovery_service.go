package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"task-processor/internal/listingkit/core"
	"task-processor/internal/listingkit/submission"
	sheinpub "task-processor/internal/publishing/shein"
	sheinother "task-processor/internal/shein/api/other"
	sheinproduct "task-processor/internal/shein/api/product"
)

type taskSubmissionRecoveryServiceConfig struct {
	repo                        Repository
	buildTaskPreview            func(context.Context, *Task, string) (*ListingKitPreview, error)
	buildSheinSubmitProductAPI  func(context.Context, *Task) (sheinproduct.ProductAPI, error)
	buildSheinSubmitOtherAPI    func(context.Context, *Task) (sheinother.OtherAPI, error)
	rememberSheinSubmitted      func(*Task, string)
	persistSuccessfulSubmission func(context.Context, string, *Task, string) error
	resolveRemoteStatusCallback func(sheinproduct.ProductAPI, sheinother.OtherAPI, string, string, []string, string, bool, string, time.Time, string) (*sheinRemoteConfirmation, error)
}

type taskSubmissionRecoveryService struct {
	repo                        Repository
	buildTaskPreview            func(context.Context, *Task, string) (*ListingKitPreview, error)
	buildSheinSubmitProductAPI  func(context.Context, *Task) (sheinproduct.ProductAPI, error)
	buildSheinSubmitOtherAPI    func(context.Context, *Task) (sheinother.OtherAPI, error)
	rememberSheinSubmitted      func(*Task, string)
	persistSuccessfulSubmission func(context.Context, string, *Task, string) error
	resolveRemoteStatusCallback func(sheinproduct.ProductAPI, sheinother.OtherAPI, string, string, []string, string, bool, string, time.Time, string) (*sheinRemoteConfirmation, error)
}

type sheinRecoveredRemoteState struct {
	report    *sheinpub.SubmissionReport
	record    *sheinpub.SubmissionRecord
	requestID string
	now       time.Time
	response  *sheinpub.SubmissionResponse
}

func newTaskSubmissionRecoveryService(config taskSubmissionRecoveryServiceConfig) *taskSubmissionRecoveryService {
	return &taskSubmissionRecoveryService{
		repo:                        config.repo,
		buildTaskPreview:            config.buildTaskPreview,
		buildSheinSubmitProductAPI:  config.buildSheinSubmitProductAPI,
		buildSheinSubmitOtherAPI:    config.buildSheinSubmitOtherAPI,
		rememberSheinSubmitted:      config.rememberSheinSubmitted,
		persistSuccessfulSubmission: config.persistSuccessfulSubmission,
		resolveRemoteStatusCallback: config.resolveRemoteStatusCallback,
	}
}

func (s *taskSubmissionRecoveryService) beginSheinSubmitLease(ctx context.Context, taskID, action, requestID string, startedAt time.Time) (*Task, error) {
	return s.mutateTaskResult(ctx, taskID, func(task *Task) error {
		pkg, err := loadSheinSubmitLeasePackage(task)
		if err != nil {
			return err
		}
		if shouldReplayExistingSheinSubmitLease(pkg, action, requestID) {
			return errSheinSubmitReplayExisting
		}
		if shouldRecoverSheinSubmitLeaseWithSupplierCode(pkg, action, requestID, startedAt) {
			appendSheinSubmissionEvent(pkg, buildRecoverRemoteLeaseEvent(taskID, action, pkg.SubmissionState.CurrentPhase, requestID, startedAt))
			return errSheinSubmitRecoverRemote
		}
		if err := validateActiveSheinSubmitLease(pkg, action, requestID, startedAt); err != nil {
			return err
		}
		beginNewSheinSubmitLease(task, pkg, taskID, action, requestID, startedAt)
		return nil
	})
}

func loadSheinSubmitLeasePackage(task *Task) (*SheinPackage, error) {
	if task == nil || task.Result == nil {
		return nil, ErrTaskResultUnavailable
	}
	pkg := sheinpub.NormalizePackageSemanticFields(task.Result.Shein)
	if pkg == nil || pkg.PreviewPayload == nil {
		return nil, errSheinSubmitMissingPackage
	}
	return pkg, nil
}

func shouldReplayExistingSheinSubmitLease(pkg *SheinPackage, action, requestID string) bool {
	return findSheinSubmissionRecordByRequestID(pkg, action, requestID) != nil
}

func shouldRecoverSheinSubmitLeaseWithSupplierCode(pkg *SheinPackage, action, requestID string, startedAt time.Time) bool {
	if !shouldRecoverSheinSubmitLeaseRemote(pkg, action, requestID, startedAt) {
		return false
	}
	record := sheinSubmissionRecordForAction(pkg.SubmissionState, action)
	return record != nil && strings.TrimSpace(record.SupplierCode) != ""
}

func shouldRecoverSheinSubmitLeaseRemote(pkg *SheinPackage, action, requestID string, startedAt time.Time) bool {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.SubmissionState == nil {
		return false
	}
	sameRequestNeedsRecovery := pkg.SubmissionState.CurrentRequestID == requestID &&
		(pkg.SubmissionState.CurrentPhase != sheinpub.SubmissionPhaseSubmitRemote ||
			sheinSubmitRemoteResponsePersisted(pkg, action, requestID))
	return sameRequestNeedsRecovery || sheinSubmitAttemptNeedsRemoteRecovery(pkg.SubmissionState, action, startedAt)
}

func buildRecoverRemoteLeaseEvent(taskID, action, phase, requestID string, startedAt time.Time) sheinpub.SubmissionEvent {
	return submission.BuildPhaseEvent(taskID, action, phase, sheinpub.SubmissionStatusRunning, requestID, startedAt, "远端可能已收到，正在刷新诊断状态", nil)
}

func validateActiveSheinSubmitLease(pkg *SheinPackage, action, requestID string, startedAt time.Time) error {
	active := findActiveSheinSubmitAttempt(pkg, action, startedAt)
	if active == nil {
		return nil
	}
	if active.CurrentRequestID == requestID {
		return errSheinSubmitReplayExisting
	}
	return buildSheinSubmitInProgressError(action, active)
}

func buildSheinSubmitInProgressError(action string, active *sheinpub.SubmissionReport) error {
	if active == nil {
		return nil
	}
	return &submission.SubmitInProgressError{
		Platform:       "shein",
		Action:         action,
		Phase:          active.CurrentPhase,
		RequestID:      active.CurrentRequestID,
		LeaseExpiresAt: active.LeaseExpiresAt,
	}
}

func beginNewSheinSubmitLease(task *Task, pkg *SheinPackage, taskID, action, requestID string, startedAt time.Time) {
	if task == nil || task.Result == nil || pkg == nil {
		return
	}
	_, event := submission.BeginAttemptAndBuildEvent(pkg, taskID, action, requestID, sheinpub.SubmissionPhaseValidate, startedAt, sheinSubmitInFlightTTL)
	appendSheinSubmissionEvent(pkg, event)
	task.Result.UpdatedAt = startedAt
}

func sheinSubmitRemoteResponsePersisted(pkg *SheinPackage, action, requestID string) bool {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.SubmissionState == nil {
		return false
	}
	record := sheinSubmissionRecordForAction(pkg.SubmissionState, action)
	if record == nil || record.RequestID != requestID {
		return false
	}
	return record.Result != nil
}

func (s *taskSubmissionRecoveryService) mutateTaskResult(ctx context.Context, taskID string, mutate TaskResultMutation) (*Task, error) {
	if txRepo, ok := s.repo.(TaskResultTransactionRepository); ok {
		return txRepo.MutateTaskResult(ctx, taskID, mutate)
	}
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if mutate != nil {
		if err := mutate(task); err != nil {
			return task, err
		}
	}
	if task.Result != nil {
		if err := s.repo.SaveTaskResult(ctx, taskID, task.Result); err != nil {
			return nil, err
		}
	}
	return task, nil
}

func (s *taskSubmissionRecoveryService) recoverSheinSubmitRemote(ctx context.Context, task *Task, action string) (*ListingKitPreview, error) {
	pkg, recoveredState, err := s.loadRecoveredRemoteState(task, action)
	if err != nil {
		return nil, err
	}
	if s.shouldRecoverSheinSubmitLocally(action, recoveredState.response) {
		return s.recoverSheinSubmitLocally(ctx, task, pkg, action, recoveredState)
	}
	return s.recoverSheinSubmitViaRemoteConfirmation(ctx, task, pkg, action, recoveredState)
}

func (s *taskSubmissionRecoveryService) refreshSheinSubmitRemoteStatus(ctx context.Context, task *Task, taskID string, pkg *SheinPackage, productAPI sheinproduct.ProductAPI, action, requestID, supplierCode string, startedAt time.Time) (*sheinpub.SubmissionEvent, error) {
	inputs := buildSheinRemoteRefreshState(pkg, action, supplierCode)
	if shouldSkipSheinRemoteRefreshLookup(inputs.lookupCodes) {
		return applyMissingSupplierCodeRemoteConfirmation(pkg, taskID, action, requestID, startedAt, inputs.defaultConfirmed), nil
	}
	confirmation, err := s.resolveSheinRemoteRefreshConfirmation(ctx, task, pkg, productAPI, action, requestID, startedAt, taskID, inputs)
	applySheinRemoteRefreshConfirmation(pkg, action, requestID, confirmation)
	return confirmation.event, err
}

type sheinRemoteRefreshState struct {
	lookupCodes      []string
	spuName          string
	defaultConfirmed bool
	fallbackMessage  string
}

func buildSheinRemoteRefreshState(pkg *SheinPackage, action, supplierCode string) sheinRemoteRefreshState {
	defaultConfirmed := action == "publish" && sheinRemotePublishAccepted(pkg, action)
	fallbackMessage := "refreshing SHEIN remote record"
	if defaultConfirmed {
		fallbackMessage = "SHEIN accepted publish request; remote record not yet visible"
	}
	return sheinRemoteRefreshState{
		lookupCodes:      collectSheinRemoteLookupCodes(pkg, supplierCode),
		spuName:          sheinRemoteLookupSPUName(pkg, action),
		defaultConfirmed: defaultConfirmed,
		fallbackMessage:  fallbackMessage,
	}
}

func shouldSkipSheinRemoteRefreshLookup(lookupCodes []string) bool {
	return len(lookupCodes) == 0
}

func (s *taskSubmissionRecoveryService) resolveSheinRemoteRefreshConfirmation(ctx context.Context, task *Task, pkg *SheinPackage, productAPI sheinproduct.ProductAPI, action, requestID string, startedAt time.Time, taskID string, inputs sheinRemoteRefreshState) (*sheinRemoteConfirmation, error) {
	otherAPI := s.buildSheinSubmitOtherAPIForRecovery(ctx, task)
	return s.resolveRemoteStatus(productAPI, otherAPI, action, requestID, inputs.lookupCodes, inputs.spuName, inputs.defaultConfirmed, inputs.fallbackMessage, startedAt, taskID)
}

func applySheinRemoteRefreshConfirmation(pkg *SheinPackage, action, requestID string, confirmation *sheinRemoteConfirmation) {
	if pkg == nil || confirmation == nil {
		return
	}
	setSheinSubmitRemoteRecord(pkg, action, requestID, confirmation.remoteStatus, confirmation.record, confirmation.checkedAt, confirmation.message)
}

func applyMissingSupplierCodeRemoteConfirmation(pkg *SheinPackage, taskID, action, requestID string, startedAt time.Time, defaultConfirmed bool) *sheinpub.SubmissionEvent {
	now := time.Now()
	remoteStatus := sheinpub.SubmissionRemoteStatusPending
	detail := "SHEIN submit succeeded, but supplier code is unavailable for remote confirmation"
	if defaultConfirmed {
		remoteStatus = sheinpub.SubmissionRemoteStatusConfirmed
		detail = "SHEIN accepted publish request, but supplier code is unavailable for remote confirmation"
	}
	setSheinSubmitRemoteRecord(pkg, action, requestID, remoteStatus, nil, now, detail)
	event := submission.BuildConfirmRemoteEvent(taskID, action, remoteStatus, requestID, startedAt, detail, nil)
	return &event
}

func (s *taskSubmissionRecoveryService) buildSheinSubmitOtherAPIForRecovery(ctx context.Context, task *Task) sheinother.OtherAPI {
	if s.buildSheinSubmitOtherAPI == nil || task == nil {
		return nil
	}
	otherAPI, _ := s.buildSheinSubmitOtherAPI(ctx, task)
	return otherAPI
}

func (s *taskSubmissionRecoveryService) loadRecoveredRemoteState(task *Task, action string) (*SheinPackage, *sheinRecoveredRemoteState, error) {
	pkg, report, err := loadRecoveredSheinSubmissionReport(task)
	if err != nil {
		return nil, nil, err
	}
	return pkg, buildRecoveredSheinRemoteState(report, action), nil
}

func (s *taskSubmissionRecoveryService) shouldRecoverSheinSubmitLocally(action string, response *sheinpub.SubmissionResponse) bool {
	return sheinSubmissionResponseAccepted(response) || (action == "save_draft" && submission.SaveDraftSucceeded(action, response))
}

func (s *taskSubmissionRecoveryService) recoverSheinSubmitLocally(ctx context.Context, task *Task, pkg *SheinPackage, action string, state *sheinRecoveredRemoteState) (*ListingKitPreview, error) {
	if task == nil || pkg == nil || state == nil {
		return nil, ErrTaskResultUnavailable
	}
	s.completeRecoveredSheinLocalState(pkg, task.ID, action, state)
	return s.finalizeRecoveredSheinSubmission(ctx, task, action)
}

func (s *taskSubmissionRecoveryService) completeRecoveredSheinLocalState(pkg *SheinPackage, taskID, action string, state *sheinRecoveredRemoteState) {
	appendRecoveredSheinLocalCompletionEvents(pkg, taskID, action, state)
}

func loadRecoveredSheinSubmissionReport(task *Task) (*SheinPackage, *sheinpub.SubmissionReport, error) {
	if task == nil || task.Result == nil {
		return nil, nil, ErrTaskResultUnavailable
	}
	pkg := sheinpub.NormalizePackageSemanticFields(task.Result.Shein)
	if pkg == nil || pkg.SubmissionState == nil {
		return nil, nil, ErrTaskResultUnavailable
	}
	return pkg, pkg.SubmissionState, nil
}

func buildRecoveredSheinRemoteState(report *sheinpub.SubmissionReport, action string) *sheinRecoveredRemoteState {
	if report == nil {
		return nil
	}
	record := sheinSubmissionRecordForAction(report, action)
	return &sheinRecoveredRemoteState{
		report:    report,
		record:    record,
		requestID: report.CurrentRequestID,
		now:       time.Now(),
		response:  resolveRecoveredSheinRemoteResponse(report, record),
	}
}

func resolveRecoveredSheinRemoteResponse(report *sheinpub.SubmissionReport, record *sheinpub.SubmissionRecord) *sheinpub.SubmissionResponse {
	if response := recordResult(record); response != nil {
		return response
	}
	if report != nil {
		return report.LastResult
	}
	return nil
}

func appendRecoveredSheinLocalCompletionEvents(pkg *SheinPackage, taskID, action string, state *sheinRecoveredRemoteState) {
	if pkg == nil || state == nil {
		return
	}
	appendSheinSubmissionEvent(pkg, buildRecoveredSheinLocalPersistEvent(taskID, action, state))
	appendRecoveredSheinLocalCompletionEvent(pkg, taskID, action, state)
}

func buildRecoveredSheinLocalPersistEvent(taskID, action string, state *sheinRecoveredRemoteState) sheinpub.SubmissionEvent {
	return submission.BuildPhaseEvent(taskID, action, sheinpub.SubmissionPhasePersistResult, sheinpub.SubmissionStatusRunning, state.requestID, state.now, "恢复本地提交完成状态", nil)
}

func appendRecoveredSheinLocalCompletionEvent(pkg *SheinPackage, taskID, action string, state *sheinRecoveredRemoteState) {
	if pkg == nil || state == nil || state.record == nil {
		return
	}
	_, completionEvent := submission.CompleteAttemptAndBuildEvent(pkg, taskID, action, state.requestID, state.response, nil, state.record.StartedAt, time.Now())
	appendSheinSubmissionEvent(pkg, completionEvent)
}

func (s *taskSubmissionRecoveryService) recoverSheinSubmitViaRemoteConfirmation(ctx context.Context, task *Task, pkg *SheinPackage, action string, state *sheinRecoveredRemoteState) (*ListingKitPreview, error) {
	if task == nil || pkg == nil || state == nil {
		return nil, ErrTaskResultUnavailable
	}
	productAPI, err := s.buildRecoveredSheinRemoteProductAPI(ctx, task, state)
	if err != nil {
		return nil, err
	}
	event, remoteErr := s.resolveRecoveredSheinRemoteConfirmation(ctx, task, pkg, productAPI, action, state)
	appendRecoveredSheinRemoteConfirmationEvent(pkg, event)
	if remoteErr != nil {
		return nil, s.persistSheinRecoveredRemoteFailure(ctx, task, pkg, action, state, remoteErr)
	}
	return s.completeSheinRecoveredRemoteSuccess(ctx, task, pkg, action, state)
}

func (s *taskSubmissionRecoveryService) resolveRecoveredSheinRemoteConfirmation(ctx context.Context, task *Task, pkg *SheinPackage, productAPI sheinproduct.ProductAPI, action string, state *sheinRecoveredRemoteState) (*sheinpub.SubmissionEvent, error) {
	appendRecoveredSheinRemoteConfirmationPhase(pkg, task.ID, action, state)
	return s.refreshRecoveredSheinRemoteStatus(ctx, task, pkg, productAPI, action, state)
}

func (s *taskSubmissionRecoveryService) buildRecoveredSheinRemoteProductAPI(ctx context.Context, task *Task, state *sheinRecoveredRemoteState) (sheinproduct.ProductAPI, error) {
	if state == nil || state.record == nil || strings.TrimSpace(state.record.SupplierCode) == "" {
		return nil, fmt.Errorf("%w: stale SHEIN submit has no supplier code", core.ErrSubmitInProgress)
	}
	return s.buildSheinSubmitProductAPI(ctx, task)
}

func (s *taskSubmissionRecoveryService) advanceRecoveredSheinRemoteConfirmation(pkg *SheinPackage, taskID, action string, state *sheinRecoveredRemoteState) {
	if pkg == nil || state == nil {
		return
	}
	appendSheinSubmissionEvent(pkg, submission.AdvancePhaseAndBuildEvent(pkg, taskID, action, state.requestID, sheinpub.SubmissionPhaseConfirmRemote, state.now, sheinSubmitInFlightTTL))
}

func appendRecoveredSheinRemoteConfirmationPhase(pkg *SheinPackage, taskID, action string, state *sheinRecoveredRemoteState) {
	sAdvance := submission.AdvancePhaseAndBuildEvent(pkg, taskID, action, state.requestID, sheinpub.SubmissionPhaseConfirmRemote, state.now, sheinSubmitInFlightTTL)
	appendSheinSubmissionEvent(pkg, sAdvance)
}

func appendRecoveredSheinRemoteConfirmationEvent(pkg *SheinPackage, event *sheinpub.SubmissionEvent) {
	if pkg == nil || event == nil {
		return
	}
	appendSheinSubmissionEvent(pkg, *event)
}

func (s *taskSubmissionRecoveryService) refreshRecoveredSheinRemoteStatus(ctx context.Context, task *Task, pkg *SheinPackage, productAPI sheinproduct.ProductAPI, action string, state *sheinRecoveredRemoteState) (*sheinpub.SubmissionEvent, error) {
	if state == nil || state.record == nil {
		return nil, ErrTaskResultUnavailable
	}
	return s.refreshSheinSubmitRemoteStatus(ctx, task, task.ID, pkg, productAPI, action, state.requestID, state.record.SupplierCode, state.now)
}

func (s *taskSubmissionRecoveryService) persistSheinRecoveredRemoteFailure(ctx context.Context, task *Task, pkg *SheinPackage, action string, state *sheinRecoveredRemoteState, remoteErr error) error {
	if task == nil || pkg == nil || state == nil {
		return remoteErr
	}
	s.appendRecoveredSheinRemoteFailureEvent(pkg, task.ID, action, state, remoteErr)
	if err := s.saveRecoveredSheinRemoteFailure(ctx, task); err != nil {
		return err
	}
	return remoteErr
}

func (s *taskSubmissionRecoveryService) appendRecoveredSheinRemoteFailureEvent(pkg *SheinPackage, taskID, action string, state *sheinRecoveredRemoteState, remoteErr error) {
	_, failureEvent := submission.FailAttemptAndBuildEvent(pkg, taskID, action, state.requestID, sheinpub.SubmissionPhaseConfirmRemote, remoteErr, time.Now())
	appendSheinSubmissionEvent(pkg, failureEvent)
}

func (s *taskSubmissionRecoveryService) saveRecoveredSheinRemoteFailure(ctx context.Context, task *Task) error {
	if task == nil || task.Result == nil {
		return nil
	}
	task.Result.UpdatedAt = time.Now()
	return s.repo.SaveTaskResult(ctx, task.ID, task.Result)
}

func (s *taskSubmissionRecoveryService) completeSheinRecoveredRemoteSuccess(ctx context.Context, task *Task, pkg *SheinPackage, action string, state *sheinRecoveredRemoteState) (*ListingKitPreview, error) {
	if task == nil || pkg == nil || state == nil {
		return nil, ErrTaskResultUnavailable
	}
	appendRecoveredSheinRemoteSuccessEvent(pkg, task.ID, action, state)
	return s.finalizeRecoveredSheinSubmission(ctx, task, action)
}

func resolveRecoveredSheinSubmissionResponse(state *sheinRecoveredRemoteState) *sheinpub.SubmissionResponse {
	if state == nil {
		return nil
	}
	if state.record != nil && state.record.Result != nil {
		return state.record.Result
	}
	if state.report != nil {
		return state.report.LastResult
	}
	return nil
}

func appendRecoveredSheinRemoteSuccessEvent(pkg *SheinPackage, taskID, action string, state *sheinRecoveredRemoteState) {
	if pkg == nil || state == nil || state.record == nil {
		return
	}
	record, completionEvent := buildRecoveredSheinRemoteSuccessEvent(pkg, taskID, action, state)
	if record != nil && record.Result == nil {
		record.Status = sheinpub.SubmissionStatusSuccess
	}
	appendSheinSubmissionEvent(pkg, completionEvent)
}

func buildRecoveredSheinRemoteSuccessEvent(pkg *SheinPackage, taskID, action string, state *sheinRecoveredRemoteState) (*sheinpub.SubmissionRecord, sheinpub.SubmissionEvent) {
	return submission.CompleteAttemptAndBuildEvent(pkg, taskID, action, state.requestID, resolveRecoveredSheinSubmissionResponse(state), nil, state.record.StartedAt, time.Now())
}

func (s *taskSubmissionRecoveryService) finalizeRecoveredSheinSubmission(ctx context.Context, task *Task, action string) (*ListingKitPreview, error) {
	if task == nil {
		return nil, ErrTaskResultUnavailable
	}
	s.rememberRecoveredSheinSubmission(task, action)
	if err := s.persistRecoveredSheinSubmission(ctx, task, action); err != nil {
		return nil, err
	}
	return s.buildRecoveredSheinPreview(ctx, task)
}

func (s *taskSubmissionRecoveryService) rememberRecoveredSheinSubmission(task *Task, action string) {
	if s.rememberSheinSubmitted != nil {
		s.rememberSheinSubmitted(task, action)
	}
}

func (s *taskSubmissionRecoveryService) persistRecoveredSheinSubmission(ctx context.Context, task *Task, action string) error {
	return s.persistSuccessfulSubmission(ctx, task.ID, task, action)
}

func (s *taskSubmissionRecoveryService) buildRecoveredSheinPreview(ctx context.Context, task *Task) (*ListingKitPreview, error) {
	return s.buildTaskPreview(ctx, task, "shein")
}

func (s *taskSubmissionRecoveryService) clearSheinSubmitLease(ctx context.Context, taskID, action, requestID string) error {
	return s.mutateSheinSubmitLease(ctx, taskID, func(task *Task, pkg *SheinPackage) {
		clearSheinSubmitLeaseState(task, pkg, action, requestID)
	})
}

func (s *taskSubmissionRecoveryService) clearSheinSubmitLeaseAfterStartFailure(ctx context.Context, taskID, action, requestID string, startErr error) error {
	return s.mutateSheinSubmitLease(ctx, taskID, func(task *Task, pkg *SheinPackage) {
		markSheinSubmitStartFailure(pkg, taskID, action, requestID, startErr)
		clearSheinSubmitLeaseState(task, pkg, action, requestID)
	})
}

func (s *taskSubmissionRecoveryService) mutateSheinSubmitLease(ctx context.Context, taskID string, mutate func(*Task, *SheinPackage)) error {
	_, err := s.mutateTaskResult(ctx, taskID, func(task *Task) error {
		pkg := loadSheinSubmitLeaseState(task)
		if pkg == nil {
			return nil
		}
		if mutate != nil {
			mutate(task, pkg)
		}
		return nil
	})
	return err
}

func loadSheinSubmitLeaseState(task *Task) *SheinPackage {
	if task == nil || task.Result == nil {
		return nil
	}
	pkg := sheinpub.NormalizePackageSemanticFields(task.Result.Shein)
	if pkg == nil || pkg.SubmissionState == nil {
		return nil
	}
	return pkg
}

func clearSheinSubmitLeaseState(task *Task, pkg *SheinPackage, action, requestID string) {
	if task == nil || pkg == nil || pkg.SubmissionState == nil {
		return
	}
	clearSheinSubmitInFlight(pkg.SubmissionState, action, requestID)
	task.Result.UpdatedAt = time.Now()
}

func markSheinSubmitStartFailure(pkg *SheinPackage, taskID, action, requestID string, startErr error) {
	if pkg == nil || pkg.SubmissionState == nil {
		return
	}
	record := sheinSubmissionRecordForAction(pkg.SubmissionState, action)
	if record == nil || record.RequestID != requestID || record.Status != sheinpub.SubmissionStatusRunning {
		return
	}
	finishedAt := time.Now()
	record.Status = sheinpub.SubmissionStatusFailed
	record.Phase = sheinpub.SubmissionPhaseValidate
	record.FinishedAt = &finishedAt
	if startErr != nil {
		record.Error = startErr.Error()
	}
	submission.ApplyRecord(pkg, record)
	appendSheinSubmissionEvent(pkg, submission.BuildEvent(taskID, action, record, nil, startErr, record.StartedAt))
}

func (s *taskSubmissionRecoveryService) resolveRemoteStatus(productAPI sheinproduct.ProductAPI, otherAPI sheinother.OtherAPI, action, requestID string, lookupCodes []string, spuName string, defaultConfirmed bool, fallbackMessage string, startedAt time.Time, taskID string) (*sheinRemoteConfirmation, error) {
	if s.resolveRemoteStatusCallback == nil {
		return nil, errors.New("submit remote status resolution is not configured")
	}
	return s.resolveRemoteStatusCallback(productAPI, otherAPI, action, requestID, lookupCodes, spuName, defaultConfirmed, fallbackMessage, startedAt, taskID)
}

type sheinRemoteConfirmation struct {
	remoteStatus string
	record       *sheinproduct.RecordItem
	checkedAt    time.Time
	message      string
	event        *sheinpub.SubmissionEvent
}

func newSheinRemoteConfirmation(parts submission.ConfirmRemoteParts) *sheinRemoteConfirmation {
	return &sheinRemoteConfirmation{
		remoteStatus: parts.RemoteStatus,
		record:       parts.Record,
		checkedAt:    parts.CheckedAt,
		message:      parts.Message,
		event:        &parts.Event,
	}
}

func (s *taskSubmissionRecoveryService) resolveSheinSubmitRemoteStatus(productAPI sheinproduct.ProductAPI, otherAPI sheinother.OtherAPI, action, requestID string, lookupCodes []string, spuName string, defaultConfirmed bool, fallbackMessage string, startedAt time.Time, taskID string) (*sheinRemoteConfirmation, error) {
	return s.resolveRemoteStatus(productAPI, otherAPI, action, requestID, lookupCodes, spuName, defaultConfirmed, fallbackMessage, startedAt, taskID)
}

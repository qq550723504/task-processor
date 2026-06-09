package listingkit

import (
	"context"
	"strings"
	"time"

	apperrors "task-processor/internal/core/errors"
	sheinpub "task-processor/internal/publishing/shein"
	sheinother "task-processor/internal/shein/api/other"
	sheinproduct "task-processor/internal/shein/api/product"
)

type sheinSubmissionRefreshState struct {
	task             *Task
	action           string
	requestID        string
	startedAt        time.Time
	lookupCodes      []string
	defaultConfirmed bool
	fallbackMessage  string
	productAPI       sheinproduct.ProductAPI
	otherAPI         sheinother.OtherAPI
	spuName          string
}

type sheinSubmissionRefreshRemoteInputs struct {
	lookupCodes      []string
	spuName          string
	defaultConfirmed bool
	fallbackMessage  string
}

type sheinSubmissionRefreshRequest struct {
	action       string
	requestID    string
	remoteInputs sheinSubmissionRefreshRemoteInputs
}

type sheinSubmissionRefreshConfirmationRequest struct {
	productAPI       sheinproduct.ProductAPI
	otherAPI         sheinother.OtherAPI
	action           string
	requestID        string
	lookupCodes      []string
	spuName          string
	defaultConfirmed bool
	fallbackMessage  string
	startedAt        time.Time
	taskID           string
}

type sheinSubmissionRefreshSelection struct {
	action       string
	record       *sheinpub.SubmissionRecord
	supplierCode string
}

func (s *taskSubmissionService) RefreshSubmissionStatus(ctx context.Context, taskID string) (*ListingKitPreview, error) {
	if s.lockSubmit != nil {
		unlockSubmit := s.lockSubmit(taskID + ":refresh_submission_status")
		defer unlockSubmit()
	}
	refreshState, err := s.loadSheinSubmissionRefreshState(ctx, taskID)
	if err != nil {
		return nil, err
	}
	confirmation, remoteErr := s.resolveSubmissionRefreshConfirmation(taskID, refreshState)
	if remoteErr != nil && confirmation == nil {
		return s.finishSubmissionRefresh(ctx, taskID, refreshState, nil, remoteErr)
	}
	return s.finishSubmissionRefresh(ctx, taskID, refreshState, confirmation, remoteErr)
}

func (s *taskSubmissionService) resolveSubmissionRefreshConfirmation(taskID string, refreshState *sheinSubmissionRefreshState) (*sheinRemoteConfirmation, error) {
	request, err := buildSubmissionRefreshConfirmationRequest(taskID, refreshState)
	if err != nil {
		return nil, err
	}
	return s.resolveSubmissionRefreshRemoteConfirmation(request)
}

func (s *taskSubmissionService) finishSubmissionRefresh(ctx context.Context, taskID string, refreshState *sheinSubmissionRefreshState, confirmation *sheinRemoteConfirmation, remoteErr error) (*ListingKitPreview, error) {
	task, err := s.persistSheinSubmissionRefreshResult(ctx, taskID, refreshState, confirmation)
	if err != nil {
		return nil, err
	}
	return s.completeSubmissionRefresh(ctx, task, remoteErr)
}

func (s *taskSubmissionService) loadSheinSubmissionRefreshState(ctx context.Context, taskID string) (*sheinSubmissionRefreshState, error) {
	task, pkg, err := s.loadSheinSubmissionRefreshTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	selection, productAPI, err := s.loadSubmissionRefreshInputs(ctx, taskID, task, pkg)
	if err != nil {
		return nil, err
	}
	return s.buildSheinSubmissionRefreshState(ctx, task, pkg, selection, productAPI), nil
}

func (s *taskSubmissionService) loadSheinSubmissionRefreshTask(ctx context.Context, taskID string) (*Task, *SheinPackage, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, nil, apperrors.Wrapf(err, apperrors.ErrCodeTaskNotFound, "failed to get task %s", taskID)
	}
	if task.Result == nil {
		return nil, nil, apperrors.New(apperrors.ErrCodeTaskProcessing, "task result is not available yet")
	}
	pkg, err := loadSubmissionRefreshTaskPackage(task)
	if err != nil {
		return task, nil, err
	}
	return task, pkg, nil
}

func (s *taskSubmissionService) completeSubmissionRefresh(ctx context.Context, task *Task, remoteErr error) (*ListingKitPreview, error) {
	if remoteErr != nil {
		return nil, remoteErr
	}
	return s.buildTaskPreview(ctx, task, "shein")
}

func buildSubmissionRefreshConfirmationRequest(taskID string, refreshState *sheinSubmissionRefreshState) (*sheinSubmissionRefreshConfirmationRequest, error) {
	if refreshState == nil {
		return nil, apperrors.New(apperrors.ErrCodeSystem, "submission refresh state is not available")
	}
	return &sheinSubmissionRefreshConfirmationRequest{
		productAPI:       refreshState.productAPI,
		otherAPI:         refreshState.otherAPI,
		action:           refreshState.action,
		requestID:        refreshState.requestID,
		lookupCodes:      refreshState.lookupCodes,
		spuName:          refreshState.spuName,
		defaultConfirmed: refreshState.defaultConfirmed,
		fallbackMessage:  refreshState.fallbackMessage,
		startedAt:        refreshState.startedAt,
		taskID:           taskID,
	}, nil
}

func (s *taskSubmissionService) resolveSubmissionRefreshRemoteConfirmation(request *sheinSubmissionRefreshConfirmationRequest) (*sheinRemoteConfirmation, error) {
	if request == nil {
		return nil, apperrors.New(apperrors.ErrCodeSystem, "submission refresh confirmation request is not available")
	}
	return s.resolveSheinSubmitRemoteStatus(
		request.productAPI,
		request.otherAPI,
		request.action,
		request.requestID,
		request.lookupCodes,
		request.spuName,
		request.defaultConfirmed,
		request.fallbackMessage,
		request.startedAt,
		request.taskID,
	)
}

func (s *taskSubmissionService) loadSubmissionRefreshInputs(ctx context.Context, taskID string, task *Task, pkg *SheinPackage) (*sheinSubmissionRefreshSelection, sheinproduct.ProductAPI, error) {
	selection, err := loadSubmissionRefreshSelection(pkg)
	if err != nil {
		return nil, nil, err
	}
	productAPI, err := s.buildSubmissionRefreshProductAPI(ctx, task, taskID)
	if err != nil {
		return nil, nil, err
	}
	return selection, productAPI, nil
}

func (s *taskSubmissionService) buildSubmissionRefreshProductAPI(ctx context.Context, task *Task, taskID string) (sheinproduct.ProductAPI, error) {
	productAPI, err := s.buildSheinSubmitProductAPI(ctx, task)
	if err != nil {
		return nil, apperrors.Wrapf(err, apperrors.ErrCodePlatformError, "failed to build shein product API for task %s", taskID)
	}
	return productAPI, nil
}

func resolveSubmissionRefreshAction(report *sheinpub.SubmissionReport) string {
	if report == nil {
		return ""
	}
	action := strings.TrimSpace(report.LastAction)
	if action != "" {
		return action
	}
	if report.Publish != nil {
		return "publish"
	}
	if report.SaveDraft != nil {
		return "save_draft"
	}
	return ""
}

func loadSubmissionRefreshReport(pkg *SheinPackage) (*sheinpub.SubmissionReport, error) {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.SubmissionState == nil {
		return nil, apperrors.Wrap(ErrSubmitBlocked, apperrors.ErrCodeValidation, "shein submission is not available")
	}
	return pkg.SubmissionState, nil
}

func resolveSubmissionRefreshRecord(report *sheinpub.SubmissionReport, action string) (*sheinpub.SubmissionRecord, error) {
	record := sheinSubmissionRecordForAction(report, action)
	if record == nil {
		return nil, apperrors.Wrap(ErrSubmitBlocked, apperrors.ErrCodeValidation, "shein submission record is not available")
	}
	return record, nil
}

func resolveSubmissionRefreshSupplierCode(record *sheinpub.SubmissionRecord, pkg *SheinPackage) (string, error) {
	supplierCode := ""
	if record != nil {
		supplierCode = strings.TrimSpace(record.SupplierCode)
	}
	if supplierCode == "" {
		supplierCode = sheinSubmitSupplierCode(nil, pkg)
	}
	if supplierCode == "" {
		return "", apperrors.Wrap(ErrSubmitBlocked, apperrors.ErrCodeValidation, "shein supplier code is not available")
	}
	return supplierCode, nil
}

func loadSubmissionRefreshSelection(pkg *SheinPackage) (*sheinSubmissionRefreshSelection, error) {
	report, err := loadSubmissionRefreshReport(pkg)
	if err != nil {
		return nil, err
	}
	action := resolveSubmissionRefreshAction(report)
	record, err := resolveSubmissionRefreshRecord(report, action)
	if err != nil {
		return nil, err
	}
	supplierCode, err := resolveSubmissionRefreshSupplierCode(record, pkg)
	if err != nil {
		return nil, err
	}
	return &sheinSubmissionRefreshSelection{
		action:       action,
		record:       record,
		supplierCode: supplierCode,
	}, nil
}

func (s *taskSubmissionService) buildSheinSubmissionRefreshState(ctx context.Context, task *Task, pkg *SheinPackage, selection *sheinSubmissionRefreshSelection, productAPI sheinproduct.ProductAPI) *sheinSubmissionRefreshState {
	startedAt := time.Now()
	request := buildSubmissionRefreshRequest(pkg, selection)
	var otherAPI sheinother.OtherAPI
	if s.buildSheinSubmitOtherAPI != nil {
		otherAPI, _ = s.buildSheinSubmitOtherAPI(ctx, task)
	}
	return newSubmissionRefreshState(task, request.action, request.requestID, startedAt, productAPI, otherAPI, request.remoteInputs)
}

func buildSubmissionRefreshRequestID(record *sheinpub.SubmissionRecord) string {
	if record == nil {
		return ""
	}
	return strings.TrimSpace(record.RequestID)
}

func buildSubmissionRefreshRequest(pkg *SheinPackage, selection *sheinSubmissionRefreshSelection) sheinSubmissionRefreshRequest {
	action := ""
	record := (*sheinpub.SubmissionRecord)(nil)
	supplierCode := ""
	if selection != nil {
		action = selection.action
		record = selection.record
		supplierCode = selection.supplierCode
	}
	return sheinSubmissionRefreshRequest{
		action:       action,
		requestID:    buildSubmissionRefreshRequestID(record),
		remoteInputs: buildSubmissionRefreshRemoteInputs(pkg, action, supplierCode),
	}
}

func newSubmissionRefreshState(task *Task, action, requestID string, startedAt time.Time, productAPI sheinproduct.ProductAPI, otherAPI sheinother.OtherAPI, remoteInputs sheinSubmissionRefreshRemoteInputs) *sheinSubmissionRefreshState {
	return &sheinSubmissionRefreshState{
		task:             task,
		action:           action,
		requestID:        requestID,
		startedAt:        startedAt,
		lookupCodes:      remoteInputs.lookupCodes,
		defaultConfirmed: remoteInputs.defaultConfirmed,
		fallbackMessage:  remoteInputs.fallbackMessage,
		productAPI:       productAPI,
		otherAPI:         otherAPI,
		spuName:          remoteInputs.spuName,
	}
}

func buildSubmissionRefreshRemoteInputs(pkg *SheinPackage, action, supplierCode string) sheinSubmissionRefreshRemoteInputs {
	return sheinSubmissionRefreshRemoteInputs{
		lookupCodes:      collectSheinRemoteLookupCodes(pkg, supplierCode),
		spuName:          sheinRemoteLookupSPUName(pkg, action),
		defaultConfirmed: action == "publish" && sheinRemotePublishAccepted(pkg, action),
		// Preserve current submission-service behavior; resolveSheinSubmitRemoteStatus supplies
		// the publish fallback when defaultConfirmed is true.
		fallbackMessage: "",
	}
}

func (s *taskSubmissionService) resolveSheinSubmitRemoteStatus(productAPI sheinproduct.ProductAPI, otherAPI sheinother.OtherAPI, action, requestID string, lookupCodes []string, spuName string, defaultConfirmed bool, fallbackMessage string, startedAt time.Time, taskID string) (*sheinRemoteConfirmation, error) {
	if s.resolveRemoteStatus == nil {
		return nil, apperrors.New(apperrors.ErrCodeSystem, "submit remote status resolution is not configured")
	}
	if defaultConfirmed {
		fallbackMessage = "SHEIN accepted publish request; remote record not yet visible"
	}
	return s.resolveRemoteStatus(productAPI, otherAPI, action, requestID, lookupCodes, spuName, defaultConfirmed, fallbackMessage, startedAt, taskID)
}

package listingkit

import (
	"context"
	"strings"
	"time"

	apperrors "task-processor/internal/core/errors"
	"task-processor/internal/listingkit/core"
	"task-processor/internal/listingkit/submission"
	sheinpub "task-processor/internal/publishing/shein"
	sheinother "task-processor/internal/shein/api/other"
	sheinproduct "task-processor/internal/shein/api/product"

	"github.com/sirupsen/logrus"
)

type taskSubmissionServiceConfig struct {
	repo                            Repository
	lockSubmit                      func(key string) func()
	resolveDefaultSheinSubmitAction func(context.Context, string) (string, error)
	acquireSheinSubmitTask          func(context.Context, string, string, string, time.Time) (*Task, *ListingKitPreview, error)
	shouldStartSheinPublishWorkflow func(platform, action string) bool
	submitSheinTaskWithWorkflow     func(context.Context, string, *Task, *SubmitTaskRequest, sheinWorkflowSubmitOptions) (*ListingKitPreview, error)
	submitSheinTaskDirect           func(context.Context, string, *Task, *SubmitTaskRequest, sheinDirectSubmitOptions) (*ListingKitPreview, error)
	buildTaskPreview                func(context.Context, *Task, string) (*ListingKitPreview, error)
	buildSheinSubmitProductAPI      func(context.Context, *Task) (sheinproduct.ProductAPI, error)
	buildSheinSubmitOtherAPI        func(context.Context, *Task) (sheinother.OtherAPI, error)
	mutateTaskResult                func(context.Context, string, TaskResultMutation) (*Task, error)
	resolveRemoteStatus             func(sheinproduct.ProductAPI, sheinother.OtherAPI, string, string, []string, string, bool, string, time.Time, string) (*sheinRemoteConfirmation, error)
}

type taskSubmissionService struct {
	repo                            Repository
	lockSubmit                      func(key string) func()
	resolveDefaultSheinSubmitAction func(context.Context, string) (string, error)
	acquireSheinSubmitTask          func(context.Context, string, string, string, time.Time) (*Task, *ListingKitPreview, error)
	shouldStartSheinPublishWorkflow func(platform, action string) bool
	submitSheinTaskWithWorkflow     func(context.Context, string, *Task, *SubmitTaskRequest, sheinWorkflowSubmitOptions) (*ListingKitPreview, error)
	submitSheinTaskDirect           func(context.Context, string, *Task, *SubmitTaskRequest, sheinDirectSubmitOptions) (*ListingKitPreview, error)
	buildTaskPreview                func(context.Context, *Task, string) (*ListingKitPreview, error)
	buildSheinSubmitProductAPI      func(context.Context, *Task) (sheinproduct.ProductAPI, error)
	buildSheinSubmitOtherAPI        func(context.Context, *Task) (sheinother.OtherAPI, error)
	mutateTaskResult                func(context.Context, string, TaskResultMutation) (*Task, error)
	resolveRemoteStatus             func(sheinproduct.ProductAPI, sheinother.OtherAPI, string, string, []string, string, bool, string, time.Time, string) (*sheinRemoteConfirmation, error)
}

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

type sheinSubmissionRefreshSelection struct {
	action       string
	record       *sheinpub.SubmissionRecord
	supplierCode string
}

type sheinSubmissionAttemptState struct {
	task        *Task
	platform    string
	action      string
	requestID   string
	startedAt   time.Time
	useWorkflow bool
}

func newTaskSubmissionService(config taskSubmissionServiceConfig) *taskSubmissionService {
	return &taskSubmissionService{
		repo:                            config.repo,
		lockSubmit:                      config.lockSubmit,
		resolveDefaultSheinSubmitAction: config.resolveDefaultSheinSubmitAction,
		acquireSheinSubmitTask:          config.acquireSheinSubmitTask,
		shouldStartSheinPublishWorkflow: config.shouldStartSheinPublishWorkflow,
		submitSheinTaskWithWorkflow:     config.submitSheinTaskWithWorkflow,
		submitSheinTaskDirect:           config.submitSheinTaskDirect,
		buildTaskPreview:                config.buildTaskPreview,
		buildSheinSubmitProductAPI:      config.buildSheinSubmitProductAPI,
		buildSheinSubmitOtherAPI:        config.buildSheinSubmitOtherAPI,
		mutateTaskResult:                config.mutateTaskResult,
		resolveRemoteStatus:             config.resolveRemoteStatus,
	}
}

func (s *taskSubmissionService) SubmitTask(ctx context.Context, taskID string, req *SubmitTaskRequest) (*ListingKitPreview, error) {
	attempt, preview, err := s.buildSheinSubmissionAttemptState(ctx, taskID, req)
	if preview != nil || err != nil {
		return preview, err
	}
	if s.lockSubmit != nil {
		unlockSubmit := s.lockSubmit(taskID + ":" + attempt.action)
		defer unlockSubmit()
	}
	return s.executeSheinSubmissionAttempt(ctx, taskID, req, attempt)
}

func (s *taskSubmissionService) buildSheinSubmissionAttemptState(ctx context.Context, taskID string, req *SubmitTaskRequest) (*sheinSubmissionAttemptState, *ListingKitPreview, error) {
	platform, action, err := s.normalizeSubmitTarget(ctx, taskID, req)
	if err != nil {
		return nil, nil, err
	}

	attempt := s.buildSubmissionAttemptState(taskID, platform, action, req)
	logSheinSubmissionRequested(taskID, attempt.platform, attempt.action, attempt.requestID, attempt.useWorkflow, req)

	task, preview, err := s.acquireSubmitTask(ctx, taskID, attempt.action, attempt.requestID, attempt.startedAt)
	if preview != nil || err != nil {
		return nil, preview, err
	}
	attempt.task = task
	return attempt, nil, nil
}

func (s *taskSubmissionService) buildSubmissionAttemptState(taskID, platform, action string, req *SubmitTaskRequest) *sheinSubmissionAttemptState {
	startedAt := time.Now()
	return buildSubmissionAttemptState(taskID, platform, action, req, startedAt, s.shouldStartSheinPublishWorkflow)
}

func buildSubmissionAttemptState(taskID, platform, action string, req *SubmitTaskRequest, startedAt time.Time, shouldStartWorkflow func(string, string) bool) *sheinSubmissionAttemptState {
	requestID := normalizedSubmitIdempotencyKey(req)
	useWorkflow := shouldStartWorkflow != nil && shouldStartWorkflow(platform, action)
	if useWorkflow && requestID == "" {
		requestID = derivedSheinSubmitRequestID(taskID, action, startedAt)
	}
	return &sheinSubmissionAttemptState{
		platform:    platform,
		action:      action,
		requestID:   requestID,
		startedAt:   startedAt,
		useWorkflow: useWorkflow,
	}
}

func logSheinSubmissionRequested(taskID, platform, action, requestID string, useWorkflow bool, req *SubmitTaskRequest) {
	logrus.WithFields(logrus.Fields{
		"task_id":         strings.TrimSpace(taskID),
		"platform":        platform,
		"action":          action,
		"request_id":      requestID,
		"use_workflow":    useWorkflow,
		"confirmed_final": req != nil && req.ConfirmedFinal,
	}).Info("listingkit shein submit requested")
}

func (s *taskSubmissionService) executeSheinSubmissionAttempt(ctx context.Context, taskID string, req *SubmitTaskRequest, attempt *sheinSubmissionAttemptState) (*ListingKitPreview, error) {
	if attempt == nil {
		return nil, apperrors.New(apperrors.ErrCodeSystem, "submit attempt state is not available")
	}
	if attempt.useWorkflow {
		return s.submitSheinTaskWithWorkflow(ctx, taskID, attempt.task, req, buildWorkflowSubmitOptions(attempt))
	}
	return s.submitSheinTaskDirect(ctx, taskID, attempt.task, req, buildDirectSubmitOptions(attempt))
}

func buildWorkflowSubmitOptions(attempt *sheinSubmissionAttemptState) sheinWorkflowSubmitOptions {
	if attempt == nil {
		return sheinWorkflowSubmitOptions{}
	}
	return sheinWorkflowSubmitOptions{
		platform:  attempt.platform,
		action:    attempt.action,
		requestID: attempt.requestID,
		startedAt: attempt.startedAt,
	}
}

func buildDirectSubmitOptions(attempt *sheinSubmissionAttemptState) sheinDirectSubmitOptions {
	if attempt == nil {
		return sheinDirectSubmitOptions{}
	}
	return sheinDirectSubmitOptions{
		action:    attempt.action,
		requestID: attempt.requestID,
		startedAt: attempt.startedAt,
	}
}

func (s *taskSubmissionService) normalizeSubmitTarget(ctx context.Context, taskID string, req *SubmitTaskRequest) (platform string, action string, err error) {
	defaultAction := ""
	if req == nil || strings.TrimSpace(req.Action) == "" {
		if s.resolveDefaultSheinSubmitAction != nil {
			defaultAction, err = s.resolveDefaultSheinSubmitAction(ctx, taskID)
			if err != nil {
				return "", "", err
			}
		}
	}
	return normalizeSubmitTargetWithDefault(req, defaultAction)
}

func (s *taskSubmissionService) acquireSubmitTask(ctx context.Context, taskID, action, requestID string, startedAt time.Time) (*Task, *ListingKitPreview, error) {
	if s.acquireSheinSubmitTask == nil {
		return nil, nil, apperrors.New(apperrors.ErrCodeSystem, "submit task acquisition is not configured")
	}
	return s.acquireSheinSubmitTask(ctx, taskID, action, requestID, startedAt)
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
	if refreshState == nil {
		return nil, apperrors.New(apperrors.ErrCodeSystem, "submission refresh state is not available")
	}
	return s.resolveSheinSubmitRemoteStatus(
		refreshState.productAPI,
		refreshState.otherAPI,
		refreshState.action,
		refreshState.requestID,
		refreshState.lookupCodes,
		refreshState.spuName,
		refreshState.defaultConfirmed,
		refreshState.fallbackMessage,
		refreshState.startedAt,
		taskID,
	)
}

func (s *taskSubmissionService) finishSubmissionRefresh(ctx context.Context, taskID string, refreshState *sheinSubmissionRefreshState, confirmation *sheinRemoteConfirmation, remoteErr error) (*ListingKitPreview, error) {
	task, err := s.persistSheinSubmissionRefreshResult(ctx, taskID, refreshState, confirmation)
	if err != nil {
		return nil, err
	}
	if remoteErr != nil {
		return nil, remoteErr
	}
	return s.buildTaskPreview(ctx, task, "shein")
}

func (s *taskSubmissionService) loadSheinSubmissionRefreshState(ctx context.Context, taskID string) (*sheinSubmissionRefreshState, error) {
	task, pkg, err := s.loadSheinSubmissionRefreshTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	selection, err := loadSubmissionRefreshSelection(pkg)
	if err != nil {
		return nil, err
	}
	productAPI, err := s.buildSheinSubmitProductAPI(ctx, task)
	if err != nil {
		return nil, apperrors.Wrapf(err, apperrors.ErrCodePlatformError, "failed to build shein product API for task %s", taskID)
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

func (s *taskSubmissionService) buildSheinSubmitOtherAPIForRefresh(ctx context.Context, task *Task) sheinother.OtherAPI {
	if s.buildSheinSubmitOtherAPI == nil {
		return nil
	}
	otherAPI, _ := s.buildSheinSubmitOtherAPI(ctx, task)
	return otherAPI
}

func (s *taskSubmissionService) buildSheinSubmissionRefreshState(ctx context.Context, task *Task, pkg *SheinPackage, selection *sheinSubmissionRefreshSelection, productAPI sheinproduct.ProductAPI) *sheinSubmissionRefreshState {
	startedAt := time.Now()
	action := submissionRefreshSelectionAction(selection)
	requestID := buildSubmissionRefreshRequestID(submissionRefreshSelectionRecord(selection))
	remoteInputs := buildSubmissionRefreshRemoteInputs(pkg, action, submissionRefreshSelectionSupplierCode(selection))
	otherAPI := s.buildSheinSubmitOtherAPIForRefresh(ctx, task)
	return newSubmissionRefreshState(task, action, requestID, startedAt, productAPI, otherAPI, remoteInputs)
}

func submissionRefreshSelectionAction(selection *sheinSubmissionRefreshSelection) string {
	if selection == nil {
		return ""
	}
	return selection.action
}

func submissionRefreshSelectionRecord(selection *sheinSubmissionRefreshSelection) *sheinpub.SubmissionRecord {
	if selection == nil {
		return nil
	}
	return selection.record
}

func submissionRefreshSelectionSupplierCode(selection *sheinSubmissionRefreshSelection) string {
	if selection == nil {
		return ""
	}
	return selection.supplierCode
}

func buildSubmissionRefreshRequestID(record *sheinpub.SubmissionRecord) string {
	if record == nil {
		return ""
	}
	return strings.TrimSpace(record.RequestID)
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

func (s *taskSubmissionService) persistSheinSubmissionRefreshResult(ctx context.Context, taskID string, refreshState *sheinSubmissionRefreshState, confirmation *sheinRemoteConfirmation) (*Task, error) {
	if refreshState == nil {
		return nil, apperrors.New(apperrors.ErrCodeSystem, "submission refresh state is not available")
	}
	return s.mutateTaskResult(ctx, taskID, func(task *Task) error {
		return applySubmissionRefreshMutation(task, taskID, refreshState, confirmation)
	})
}

func applySubmissionRefreshMutation(task *Task, taskID string, refreshState *sheinSubmissionRefreshState, confirmation *sheinRemoteConfirmation) error {
	if refreshState == nil {
		return apperrors.New(apperrors.ErrCodeSystem, "submission refresh state is not available")
	}
	pkg, err := validateSubmissionRefreshMutation(task, refreshState.action, refreshState.requestID)
	if err != nil {
		return err
	}
	appendSubmissionRefreshMutationEvents(pkg, taskID, refreshState, confirmation)
	task.Result.UpdatedAt = time.Now()
	return nil
}

func appendSubmissionRefreshMutationEvents(pkg *SheinPackage, taskID string, refreshState *sheinSubmissionRefreshState, confirmation *sheinRemoteConfirmation) {
	if pkg == nil || refreshState == nil {
		return
	}
	appendSubmissionRefreshRunningEvent(pkg, taskID, refreshState)
	applySubmissionRefreshConfirmation(pkg, refreshState.action, refreshState.requestID, confirmation)
}

func appendSubmissionRefreshRunningEvent(pkg *SheinPackage, taskID string, refreshState *sheinSubmissionRefreshState) {
	if pkg == nil || refreshState == nil {
		return
	}
	appendSheinSubmissionEvent(pkg, submission.BuildRefreshConfirmRemoteRunningEvent(taskID, refreshState.action, refreshState.requestID, refreshState.startedAt))
}

func validateSubmissionRefreshMutation(task *Task, action, requestID string) (*SheinPackage, error) {
	pkg, err := loadSubmissionRefreshMutationPackage(task)
	if err != nil {
		return nil, err
	}
	if err := validateSubmissionRefreshAction(pkg, action); err != nil {
		return nil, err
	}
	if err := validateSubmissionRefreshRequest(pkg, action, requestID); err != nil {
		return nil, err
	}
	return pkg, nil
}

func loadSubmissionRefreshMutationPackage(task *Task) (*SheinPackage, error) {
	return loadSubmissionRefreshTaskPackage(task)
}

func loadSubmissionRefreshTaskPackage(task *Task) (*SheinPackage, error) {
	if task == nil || task.Result == nil {
		return nil, apperrors.Wrap(ErrSubmitBlocked, apperrors.ErrCodeValidation, "shein submission is not available")
	}
	pkg := sheinpub.NormalizePackageSemanticFields(task.Result.Shein)
	if pkg == nil || pkg.SubmissionState == nil {
		return nil, apperrors.Wrap(ErrSubmitBlocked, apperrors.ErrCodeValidation, "shein submission is not available")
	}
	return pkg, nil
}

func validateSubmissionRefreshAction(pkg *SheinPackage, action string) error {
	if pkg == nil || pkg.SubmissionState == nil {
		return apperrors.Wrap(ErrSubmitBlocked, apperrors.ErrCodeValidation, "shein submission is not available")
	}
	currentAction := resolveSubmissionRefreshAction(pkg.SubmissionState)
	if currentAction == "" {
		currentAction = action
	}
	if currentAction != action {
		return buildSubmissionRefreshChangedError()
	}
	return nil
}

func validateSubmissionRefreshRequest(pkg *SheinPackage, action, requestID string) error {
	if pkg == nil || pkg.SubmissionState == nil {
		return apperrors.Wrap(ErrSubmitBlocked, apperrors.ErrCodeValidation, "shein submission is not available")
	}
	currentRecord := sheinSubmissionRecordForAction(pkg.SubmissionState, action)
	if currentRecord == nil || strings.TrimSpace(currentRecord.RequestID) != requestID {
		return buildSubmissionRefreshChangedError()
	}
	return nil
}

func buildSubmissionRefreshChangedError() error {
	return apperrors.Wrap(core.ErrSubmitInProgress, apperrors.ErrCodeTaskProcessing, "shein submission changed during refresh")
}

func applySubmissionRefreshConfirmation(pkg *SheinPackage, action, requestID string, confirmation *sheinRemoteConfirmation) {
	if pkg == nil || confirmation == nil {
		return
	}
	if parts, ok := buildSubmissionRefreshConfirmRemoteParts(confirmation); ok {
		submission.ApplyConfirmRemoteParts(pkg, action, requestID, parts)
		return
	}
	applySubmissionRefreshRemoteRecord(pkg, action, requestID, confirmation)
}

func buildSubmissionRefreshConfirmRemoteParts(confirmation *sheinRemoteConfirmation) (submission.ConfirmRemoteParts, bool) {
	if confirmation == nil || confirmation.event == nil {
		return submission.ConfirmRemoteParts{}, false
	}
	return submission.ConfirmRemoteParts{
		RemoteStatus: confirmation.remoteStatus,
		Record:       confirmation.record,
		CheckedAt:    confirmation.checkedAt,
		Message:      confirmation.message,
		Event:        *confirmation.event,
	}, true
}

func applySubmissionRefreshRemoteRecord(pkg *SheinPackage, action, requestID string, confirmation *sheinRemoteConfirmation) {
	if pkg == nil || confirmation == nil {
		return
	}
	setSheinSubmitRemoteRecord(pkg, action, requestID, confirmation.remoteStatus, confirmation.record, confirmation.checkedAt, confirmation.message)
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

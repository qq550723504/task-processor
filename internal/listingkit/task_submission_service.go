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

	startedAt := time.Now()
	requestID := normalizedSubmitIdempotencyKey(req)
	useWorkflow := s.shouldStartSheinPublishWorkflow != nil && s.shouldStartSheinPublishWorkflow(platform, action)
	if useWorkflow && requestID == "" {
		requestID = derivedSheinSubmitRequestID(taskID, action, startedAt)
	}
	logSheinSubmissionRequested(taskID, platform, action, requestID, useWorkflow, req)

	task, preview, err := s.acquireSubmitTask(ctx, taskID, action, requestID, startedAt)
	if preview != nil || err != nil {
		return nil, preview, err
	}
	return &sheinSubmissionAttemptState{
		task:        task,
		platform:    platform,
		action:      action,
		requestID:   requestID,
		startedAt:   startedAt,
		useWorkflow: useWorkflow,
	}, nil, nil
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
		return s.submitSheinTaskWithWorkflow(ctx, taskID, attempt.task, req, sheinWorkflowSubmitOptions{
			platform:  attempt.platform,
			action:    attempt.action,
			requestID: attempt.requestID,
			startedAt: attempt.startedAt,
		})
	}
	return s.submitSheinTaskDirect(ctx, taskID, attempt.task, req, sheinDirectSubmitOptions{
		action:    attempt.action,
		requestID: attempt.requestID,
		startedAt: attempt.startedAt,
	})
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
	confirmation, remoteErr := s.resolveSheinSubmitRemoteStatus(
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
	action := resolveSubmissionRefreshAction(pkg.SubmissionState)
	record, err := resolveSubmissionRefreshRecord(pkg, action)
	if err != nil {
		return nil, err
	}
	supplierCode, err := resolveSubmissionRefreshSupplierCode(record, pkg)
	if err != nil {
		return nil, err
	}
	productAPI, err := s.buildSheinSubmitProductAPI(ctx, task)
	if err != nil {
		return nil, apperrors.Wrapf(err, apperrors.ErrCodePlatformError, "failed to build shein product API for task %s", taskID)
	}
	return &sheinSubmissionRefreshState{
		task:             task,
		action:           action,
		requestID:        strings.TrimSpace(record.RequestID),
		startedAt:        time.Now(),
		lookupCodes:      collectSheinRemoteLookupCodes(pkg, supplierCode),
		defaultConfirmed: action == "publish" && sheinRemotePublishAccepted(pkg, action),
		productAPI:       productAPI,
		otherAPI:         s.buildSheinSubmitOtherAPIForRefresh(ctx, task),
		spuName:          sheinRemoteLookupSPUName(pkg, action),
	}, nil
}

func (s *taskSubmissionService) loadSheinSubmissionRefreshTask(ctx context.Context, taskID string) (*Task, *SheinPackage, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, nil, apperrors.Wrapf(err, apperrors.ErrCodeTaskNotFound, "failed to get task %s", taskID)
	}
	if task.Result == nil {
		return nil, nil, apperrors.New(apperrors.ErrCodeTaskProcessing, "task result is not available yet")
	}
	pkg := sheinpub.NormalizePackageSemanticFields(task.Result.Shein)
	if pkg == nil || pkg.SubmissionState == nil {
		return nil, nil, apperrors.Wrap(ErrSubmitBlocked, apperrors.ErrCodeValidation, "shein submission is not available")
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

func resolveSubmissionRefreshRecord(pkg *SheinPackage, action string) (*sheinpub.SubmissionRecord, error) {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.SubmissionState == nil {
		return nil, apperrors.Wrap(ErrSubmitBlocked, apperrors.ErrCodeValidation, "shein submission is not available")
	}
	record := sheinSubmissionRecordForAction(pkg.SubmissionState, action)
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

func (s *taskSubmissionService) buildSheinSubmitOtherAPIForRefresh(ctx context.Context, task *Task) sheinother.OtherAPI {
	if s.buildSheinSubmitOtherAPI == nil {
		return nil
	}
	otherAPI, _ := s.buildSheinSubmitOtherAPI(ctx, task)
	return otherAPI
}

func (s *taskSubmissionService) persistSheinSubmissionRefreshResult(ctx context.Context, taskID string, refreshState *sheinSubmissionRefreshState, confirmation *sheinRemoteConfirmation) (*Task, error) {
	if refreshState == nil {
		return nil, apperrors.New(apperrors.ErrCodeSystem, "submission refresh state is not available")
	}
	return s.mutateTaskResult(ctx, taskID, func(task *Task) error {
		pkg, err := validateSubmissionRefreshMutation(task, refreshState.action, refreshState.requestID)
		if err != nil {
			return err
		}
		appendSheinSubmissionEvent(pkg, submission.BuildRefreshConfirmRemoteRunningEvent(taskID, refreshState.action, refreshState.requestID, refreshState.startedAt))
		applySubmissionRefreshConfirmation(pkg, refreshState.action, refreshState.requestID, confirmation)
		task.Result.UpdatedAt = time.Now()
		return nil
	})
}

func validateSubmissionRefreshMutation(task *Task, action, requestID string) (*SheinPackage, error) {
	if task == nil || task.Result == nil {
		return nil, apperrors.Wrap(ErrSubmitBlocked, apperrors.ErrCodeValidation, "shein submission is not available")
	}
	pkg := sheinpub.NormalizePackageSemanticFields(task.Result.Shein)
	if pkg == nil || pkg.SubmissionState == nil {
		return nil, apperrors.Wrap(ErrSubmitBlocked, apperrors.ErrCodeValidation, "shein submission is not available")
	}
	currentAction := resolveSubmissionRefreshAction(pkg.SubmissionState)
	if currentAction == "" {
		currentAction = action
	}
	if currentAction != action {
		return nil, apperrors.Wrap(core.ErrSubmitInProgress, apperrors.ErrCodeTaskProcessing, "shein submission changed during refresh")
	}
	currentRecord := sheinSubmissionRecordForAction(pkg.SubmissionState, action)
	if currentRecord == nil || strings.TrimSpace(currentRecord.RequestID) != requestID {
		return nil, apperrors.Wrap(core.ErrSubmitInProgress, apperrors.ErrCodeTaskProcessing, "shein submission changed during refresh")
	}
	return pkg, nil
}

func applySubmissionRefreshConfirmation(pkg *SheinPackage, action, requestID string, confirmation *sheinRemoteConfirmation) {
	if pkg == nil || confirmation == nil {
		return
	}
	if confirmation.event != nil {
		submission.ApplyConfirmRemoteParts(pkg, action, requestID, submission.ConfirmRemoteParts{
			RemoteStatus: confirmation.remoteStatus,
			Record:       confirmation.record,
			CheckedAt:    confirmation.checkedAt,
			Message:      confirmation.message,
			Event:        *confirmation.event,
		})
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

package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	listingsubmission "task-processor/internal/listingkit/submission"
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
	platform, action, err := s.normalizeSubmitTarget(ctx, taskID, req)
	if err != nil {
		return nil, err
	}

	startedAt := time.Now()
	requestID := normalizedSubmitIdempotencyKey(req)
	useWorkflow := s.shouldStartSheinPublishWorkflow != nil && s.shouldStartSheinPublishWorkflow(platform, action)
	if useWorkflow && requestID == "" {
		requestID = derivedSheinSubmitRequestID(taskID, action, startedAt)
	}
	logrus.WithFields(logrus.Fields{
		"task_id":         strings.TrimSpace(taskID),
		"platform":        platform,
		"action":          action,
		"request_id":      requestID,
		"use_workflow":    useWorkflow,
		"confirmed_final": req != nil && req.ConfirmedFinal,
	}).Info("listingkit shein submit requested")
	if s.lockSubmit != nil {
		unlockSubmit := s.lockSubmit(taskID + ":" + action)
		defer unlockSubmit()
	}
	task, preview, err := s.acquireSheinSubmitTask(ctx, taskID, action, requestID, startedAt)
	if preview != nil || err != nil {
		return preview, err
	}
	if useWorkflow {
		return s.submitSheinTaskWithWorkflow(ctx, taskID, task, req, sheinWorkflowSubmitOptions{
			platform:  platform,
			action:    action,
			requestID: requestID,
			startedAt: startedAt,
		})
	}
	return s.submitSheinTaskDirect(ctx, taskID, task, req, sheinDirectSubmitOptions{
		action:    action,
		requestID: requestID,
		startedAt: startedAt,
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
		return nil, nil, fmt.Errorf("submit task acquisition is not configured")
	}
	return s.acquireSheinSubmitTask(ctx, taskID, action, requestID, startedAt)
}

func (s *taskSubmissionService) RefreshSubmissionStatus(ctx context.Context, taskID string) (*ListingKitPreview, error) {
	if s.lockSubmit != nil {
		unlockSubmit := s.lockSubmit(taskID + ":refresh_submission_status")
		defer unlockSubmit()
	}
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task.Result == nil {
		return nil, ErrTaskResultUnavailable
	}
	pkg := sheinpub.NormalizePackageSemanticFields(task.Result.Shein)
	if pkg == nil || pkg.SubmissionState == nil {
		return nil, fmt.Errorf("%w: shein submission is not available", ErrSubmitBlocked)
	}
	report := pkg.SubmissionState
	action := strings.TrimSpace(report.LastAction)
	if action == "" {
		if report.Publish != nil {
			action = "publish"
		} else if report.SaveDraft != nil {
			action = "save_draft"
		}
	}
	record := sheinSubmissionRecordForAction(report, action)
	if record == nil {
		return nil, fmt.Errorf("%w: shein submission record is not available", ErrSubmitBlocked)
	}
	supplierCode := strings.TrimSpace(record.SupplierCode)
	if supplierCode == "" {
		supplierCode = sheinSubmitSupplierCode(nil, pkg)
	}
	if supplierCode == "" {
		return nil, fmt.Errorf("%w: shein supplier code is not available", ErrSubmitBlocked)
	}
	productAPI, err := s.buildSheinSubmitProductAPI(ctx, task)
	if err != nil {
		return nil, err
	}
	var otherAPI sheinother.OtherAPI
	if s.buildSheinSubmitOtherAPI != nil {
		otherAPI, _ = s.buildSheinSubmitOtherAPI(ctx, task)
	}
	requestID := strings.TrimSpace(record.RequestID)
	startedAt := time.Now()
	lookupCodes := collectSheinRemoteLookupCodes(pkg, supplierCode)
	defaultConfirmed := action == "publish" && sheinRemotePublishAccepted(pkg, action)
	fallbackMessage := "refreshing SHEIN remote record"
	if defaultConfirmed {
		fallbackMessage = "SHEIN accepted publish request; remote record not yet visible"
	}
	confirmation, remoteErr := s.resolveSheinSubmitRemoteStatus(productAPI, otherAPI, action, requestID, lookupCodes, sheinRemoteLookupSPUName(pkg, action), defaultConfirmed, fallbackMessage, startedAt, taskID)
	task, err = s.mutateTaskResult(ctx, taskID, func(task *Task) error {
		if task.Result == nil {
			return fmt.Errorf("%w: shein submission is not available", ErrSubmitBlocked)
		}
		pkg := sheinpub.NormalizePackageSemanticFields(task.Result.Shein)
		if pkg == nil || pkg.SubmissionState == nil {
			return fmt.Errorf("%w: shein submission is not available", ErrSubmitBlocked)
		}
		currentAction := strings.TrimSpace(pkg.SubmissionState.LastAction)
		if currentAction == "" {
			currentAction = action
		}
		if currentAction != action {
			return fmt.Errorf("%w: shein submission changed during refresh", ErrSubmitInProgress)
		}
		currentRecord := sheinSubmissionRecordForAction(pkg.SubmissionState, action)
		if currentRecord == nil || strings.TrimSpace(currentRecord.RequestID) != requestID {
			return fmt.Errorf("%w: shein submission changed during refresh", ErrSubmitInProgress)
		}
		appendSheinSubmissionEvent(pkg, listingsubmission.BuildRefreshConfirmRemoteRunningEvent(taskID, action, requestID, startedAt))
		if confirmation.event != nil {
			listingsubmission.ApplyConfirmRemoteParts(pkg, action, requestID, listingsubmission.ConfirmRemoteParts{
				RemoteStatus: confirmation.remoteStatus,
				Record:       confirmation.record,
				CheckedAt:    confirmation.checkedAt,
				Message:      confirmation.message,
				Event:        *confirmation.event,
			})
		} else {
			setSheinSubmitRemoteRecord(pkg, action, requestID, confirmation.remoteStatus, confirmation.record, confirmation.checkedAt, confirmation.message)
		}
		task.Result.UpdatedAt = time.Now()
		return nil
	})
	if err != nil {
		return nil, err
	}
	if remoteErr != nil {
		return nil, remoteErr
	}
	return s.buildTaskPreview(ctx, task, "shein")
}

func (s *taskSubmissionService) resolveSheinSubmitRemoteStatus(productAPI sheinproduct.ProductAPI, otherAPI sheinother.OtherAPI, action, requestID string, lookupCodes []string, spuName string, defaultConfirmed bool, fallbackMessage string, startedAt time.Time, taskID string) (*sheinRemoteConfirmation, error) {
	if s.resolveRemoteStatus == nil {
		return nil, errors.New("submit remote status resolution is not configured")
	}
	return s.resolveRemoteStatus(productAPI, otherAPI, action, requestID, lookupCodes, spuName, defaultConfirmed, fallbackMessage, startedAt, taskID)
}

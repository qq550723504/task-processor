package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	sdktemporal "go.temporal.io/sdk/temporal"
	submissiondomain "task-processor/internal/listing/submission"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

type taskTemporalSubmissionAdapterConfig struct {
	startSheinPublishWorkflow            func(context.Context, SheinPublishWorkflowStartInput) error
	beginSheinSubmitLease                func(context.Context, string, string, string, time.Time) (*Task, error)
	loadSheinPublishTask                 func(context.Context, string) (*Task, *SheinPackage, error)
	normalizeSheinSubmitPackage          func(*Task, *SheinPackage, *SubmitTaskRequest, string)
	validateSheinPublishFreshness        func(context.Context, *Task, *SheinPackage, string) (*SheinSubmitReadiness, error)
	saveTaskResult                       func(context.Context, string, *ListingKitResult) error
	persistSheinSubmitPhase              func(context.Context, string, *ListingKitResult, *SheinPackage, string, string, string) error
	prepareSheinSubmitProduct            func(context.Context, *Task, *SheinPackage, string) (*sheinproduct.Product, error)
	uploadSheinSubmitImages              func(context.Context, *Task, *SheinPackage, *sheinproduct.Product) error
	resolveSubmitSettings                func(context.Context, *Task) SheinSettings
	buildSheinSubmitProductAPI           func(context.Context, *Task) (sheinproduct.ProductAPI, error)
	preValidateSheinSubmitProduct        func(*SheinPackage, *sheinproduct.Product) error
	executeSheinSubmitRemote             func(sheinproduct.ProductAPI, string, *sheinproduct.Product) (*sheinpub.SubmissionResponse, error)
	retrySheinSensitiveWordSubmit        func(context.Context, string, *SheinPackage, string, string, sheinproduct.ProductAPI, *sheinproduct.Product, *sheinpub.SubmissionResponse, error) (*sheinpub.SubmissionResponse, error, bool)
	persistSuccessfulSheinSubmission     func(context.Context, string, *Task, string) error
	recordSheinSubmissionFailureForState func(context.Context, string, *ListingKitResult, *SheinPackage, string, string, string, error) error
	refreshSheinSubmitRemoteStatus       func(context.Context, *Task, string, *SheinPackage, sheinproduct.ProductAPI, string, string, string, time.Time) (*sheinpub.SubmissionEvent, error)
	handleWorkflowStartFailure           func(context.Context, string, *Task, sheinWorkflowSubmitOptions, error) error
	rememberSheinSubmitted               func(*Task, string)
	getTaskPreview                       func(context.Context, string, string) (*ListingKitPreview, error)
	payloadStages                        *submissiondomain.PayloadStageService[*Task, *SheinPackage, *sheinproduct.Product, *sheinpub.SubmitSnapshot]
	remoteSubmitter                      *submissiondomain.RemoteSubmitService[*SheinPackage, sheinproduct.ProductAPI, *sheinproduct.Product, *sheinpub.SubmissionResponse, *sheinpub.SubmitSnapshot]
	successRunner                        *submissiondomain.SuccessPersistenceService[*Task, *SheinPackage, *sheinpub.SubmissionResponse]
	failureRunner                        *submissiondomain.FailurePersistenceService[*ListingKitResult, *SheinPackage]
}

type taskTemporalSubmissionAdapter struct {
	startSheinPublishWorkflow            func(context.Context, SheinPublishWorkflowStartInput) error
	beginSheinSubmitLease                func(context.Context, string, string, string, time.Time) (*Task, error)
	loadSheinPublishTask                 func(context.Context, string) (*Task, *SheinPackage, error)
	normalizeSheinSubmitPackage          func(*Task, *SheinPackage, *SubmitTaskRequest, string)
	validateSheinPublishFreshness        func(context.Context, *Task, *SheinPackage, string) (*SheinSubmitReadiness, error)
	saveTaskResult                       func(context.Context, string, *ListingKitResult) error
	persistSheinSubmitPhase              func(context.Context, string, *ListingKitResult, *SheinPackage, string, string, string) error
	prepareSheinSubmitProduct            func(context.Context, *Task, *SheinPackage, string) (*sheinproduct.Product, error)
	uploadSheinSubmitImages              func(context.Context, *Task, *SheinPackage, *sheinproduct.Product) error
	resolveSubmitSettings                func(context.Context, *Task) SheinSettings
	buildSheinSubmitProductAPI           func(context.Context, *Task) (sheinproduct.ProductAPI, error)
	preValidateSheinSubmitProduct        func(*SheinPackage, *sheinproduct.Product) error
	executeSheinSubmitRemote             func(sheinproduct.ProductAPI, string, *sheinproduct.Product) (*sheinpub.SubmissionResponse, error)
	retrySheinSensitiveWordSubmit        func(context.Context, string, *SheinPackage, string, string, sheinproduct.ProductAPI, *sheinproduct.Product, *sheinpub.SubmissionResponse, error) (*sheinpub.SubmissionResponse, error, bool)
	persistSuccessfulSheinSubmission     func(context.Context, string, *Task, string) error
	recordSheinSubmissionFailureForState func(context.Context, string, *ListingKitResult, *SheinPackage, string, string, string, error) error
	refreshSheinSubmitRemoteStatus       func(context.Context, *Task, string, *SheinPackage, sheinproduct.ProductAPI, string, string, string, time.Time) (*sheinpub.SubmissionEvent, error)
	handleWorkflowStartFailure           func(context.Context, string, *Task, sheinWorkflowSubmitOptions, error) error
	rememberSheinSubmitted               func(*Task, string)
	getTaskPreview                       func(context.Context, string, string) (*ListingKitPreview, error)
	payloadStages                        *submissiondomain.PayloadStageService[*Task, *SheinPackage, *sheinproduct.Product, *sheinpub.SubmitSnapshot]
	remoteSubmitter                      *submissiondomain.RemoteSubmitService[*SheinPackage, sheinproduct.ProductAPI, *sheinproduct.Product, *sheinpub.SubmissionResponse, *sheinpub.SubmitSnapshot]
	successRunner                        *submissiondomain.SuccessPersistenceService[*Task, *SheinPackage, *sheinpub.SubmissionResponse]
	failureRunner                        *submissiondomain.FailurePersistenceService[*ListingKitResult, *SheinPackage]
}

func newTaskTemporalSubmissionAdapter(config taskTemporalSubmissionAdapterConfig) *taskTemporalSubmissionAdapter {
	adapter := &taskTemporalSubmissionAdapter{
		startSheinPublishWorkflow:            config.startSheinPublishWorkflow,
		beginSheinSubmitLease:                config.beginSheinSubmitLease,
		loadSheinPublishTask:                 config.loadSheinPublishTask,
		normalizeSheinSubmitPackage:          config.normalizeSheinSubmitPackage,
		validateSheinPublishFreshness:        config.validateSheinPublishFreshness,
		saveTaskResult:                       config.saveTaskResult,
		persistSheinSubmitPhase:              config.persistSheinSubmitPhase,
		prepareSheinSubmitProduct:            config.prepareSheinSubmitProduct,
		uploadSheinSubmitImages:              config.uploadSheinSubmitImages,
		resolveSubmitSettings:                config.resolveSubmitSettings,
		buildSheinSubmitProductAPI:           config.buildSheinSubmitProductAPI,
		preValidateSheinSubmitProduct:        config.preValidateSheinSubmitProduct,
		executeSheinSubmitRemote:             config.executeSheinSubmitRemote,
		retrySheinSensitiveWordSubmit:        config.retrySheinSensitiveWordSubmit,
		persistSuccessfulSheinSubmission:     config.persistSuccessfulSheinSubmission,
		recordSheinSubmissionFailureForState: config.recordSheinSubmissionFailureForState,
		refreshSheinSubmitRemoteStatus:       config.refreshSheinSubmitRemoteStatus,
		handleWorkflowStartFailure:           config.handleWorkflowStartFailure,
		rememberSheinSubmitted:               config.rememberSheinSubmitted,
		getTaskPreview:                       config.getTaskPreview,
		payloadStages:                        config.payloadStages,
		remoteSubmitter:                      config.remoteSubmitter,
		successRunner:                        config.successRunner,
		failureRunner:                        config.failureRunner,
	}
	if adapter.payloadStages == nil {
		adapter.payloadStages = newSheinTemporalSubmitPayloadStages(adapter)
	}
	if adapter.remoteSubmitter == nil {
		adapter.remoteSubmitter = newSheinRemoteSubmitService(adapter.executeTemporalRemoteSubmitAttempt)
	}
	if adapter.successRunner == nil {
		adapter.successRunner = newSheinSubmissionSuccessPersistenceService(
			adapter.completeTemporalSubmitAttempt,
			adapter.persistTemporalSuccessResultAndPhase,
			adapter.rememberSheinSubmitted,
			adapter.persistSuccessfulSheinSubmission,
		)
	}
	if adapter.failureRunner == nil {
		adapter.failureRunner = newSheinSubmissionFailurePersistenceService(adapter.recordTemporalFailureState)
	}
	return adapter
}

func (s *service) loadSheinPublishTaskForTemporal(ctx context.Context, taskID string) (*Task, *SheinPackage, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, nil, err
	}
	if task.Result == nil {
		return nil, nil, ErrTaskResultUnavailable
	}
	pkg := sheinpub.NormalizePackageSemanticFields(task.Result.Shein)
	if pkg == nil || pkg.PreviewPayload == nil {
		return nil, nil, fmt.Errorf("%w: shein preview payload is not available", ErrSubmitBlocked)
	}
	return task, pkg, nil
}

func (s *taskTemporalSubmissionAdapter) startSheinPublishWorkflowAttempt(ctx context.Context, taskID string, task *Task, req *SubmitTaskRequest, opts sheinWorkflowSubmitOptions) (*ListingKitPreview, error) {
	if s.startSheinPublishWorkflow == nil {
		return nil, fmt.Errorf("shein publish workflow start is not configured")
	}
	err := s.startSheinPublishWorkflow(ctx, SheinPublishWorkflowStartInput{
		TaskID:         strings.TrimSpace(taskID),
		Platform:       opts.platform,
		Action:         opts.action,
		RequestID:      opts.requestID,
		ConfirmedFinal: req != nil && req.ConfirmedFinal,
		RequestedAt:    opts.startedAt,
	})
	if err == nil {
		return s.getTaskPreview(ctx, taskID, "shein")
	}
	if shouldReplayStartedTemporalSubmit(err, opts.requestID) {
		return s.buildSheinWorkflowReplayPreview(ctx, task)
	}
	if s.handleWorkflowStartFailure == nil {
		return nil, err
	}
	return nil, s.handleWorkflowStartFailure(ctx, taskID, task, opts, err)
}

func (s *taskTemporalSubmissionAdapter) BeginSheinPublishAttempt(ctx context.Context, in SheinPublishAttemptInput) error {
	if strings.TrimSpace(in.Action) == "" {
		in.Action = "publish"
	}
	_, err := s.beginSheinSubmitLease(ctx, in.TaskID, in.Action, in.RequestID, sheinRequestedAt(in.RequestedAt))
	if errors.Is(err, errSheinSubmitReplayExisting) {
		return nil
	}
	if errors.Is(err, errSheinSubmitMissingPackage) {
		return fmt.Errorf("%w: shein preview payload is not available", ErrSubmitBlocked)
	}
	return err
}

func (s *taskTemporalSubmissionAdapter) ValidateSheinPublishReadiness(ctx context.Context, in SheinPublishAttemptInput) error {
	task, pkg, err := s.loadSheinPublishTaskState(ctx, in.TaskID)
	if err != nil {
		return err
	}
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	prepared := prepareSheinSubmitReadinessForAction(task, pkg, sheinSubmitRequestFromActivity(in), in.Action, s.normalizeSheinSubmitPackage)
	if prepared.stateChanged {
		task.Result.UpdatedAt = time.Now()
		if err := s.saveTaskResult(ctx, in.TaskID, task.Result); err != nil {
			return err
		}
	}
	readiness := prepared.readiness
	if err := validateSheinSubmitReadinessGates(ctx, task, pkg, in.Action, readiness, s.validateSheinPublishFreshness); err != nil {
		return err
	}
	return nil
}

func (s *taskTemporalSubmissionAdapter) BuildSheinTaskPreview(ctx context.Context, taskID string) (*ListingKitPreview, error) {
	return s.getTaskPreview(ctx, taskID, "shein")
}

func buildTaskPreviewFromTask(ctx context.Context, task *Task, platform string, getTaskPreview func(context.Context, string, string) (*ListingKitPreview, error)) (*ListingKitPreview, error) {
	if task == nil {
		return nil, ErrTaskResultUnavailable
	}
	if getTaskPreview != nil {
		return getTaskPreview(ctx, task.ID, platform)
	}
	return nil, ErrTaskResultUnavailable
}

func (s *taskTemporalSubmissionAdapter) buildSheinWorkflowReplayPreview(ctx context.Context, task *Task) (*ListingKitPreview, error) {
	if task == nil {
		return nil, ErrTaskResultUnavailable
	}
	return buildTaskPreviewFromTask(ctx, task, "shein", s.getTaskPreview)
}

func (s *taskTemporalSubmissionAdapter) loadSheinPublishTaskState(ctx context.Context, taskID string) (*Task, *SheinPackage, error) {
	if s.loadSheinPublishTask == nil {
		return nil, nil, fmt.Errorf("shein publish task loader is not configured")
	}
	return s.loadSheinPublishTask(ctx, taskID)
}

func (s *taskTemporalSubmissionAdapter) persistSheinSubmitSnapshot(ctx context.Context, taskID string, result *ListingKitResult, pkg *SheinPackage, action, requestID string, snapshot *sheinpub.SubmitSnapshot) error {
	if result == nil || pkg == nil || snapshot == nil {
		return nil
	}
	setSheinSubmitSnapshot(pkg, action, requestID, snapshot)
	result.UpdatedAt = time.Now()
	return s.saveTaskResult(ctx, taskID, result)
}

func sheinSubmitRequestFromActivity(in SheinPublishAttemptInput) *SubmitTaskRequest {
	return &SubmitTaskRequest{
		Platform:       "shein",
		Action:         in.Action,
		RequestID:      in.RequestID,
		IdempotencyKey: in.RequestID,
		ConfirmedFinal: in.ConfirmedFinal,
	}
}

func sheinRequestedAt(requestedAt time.Time) time.Time {
	if requestedAt.IsZero() {
		return time.Now()
	}
	return requestedAt
}

func sheinSubmitStartedAt(pkg *SheinPackage, action, requestID string, fallback time.Time) time.Time {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.SubmissionState == nil {
		return fallback
	}
	record := sheinSubmissionRecordForAction(pkg.SubmissionState, action)
	if record != nil && record.RequestID == requestID && !record.StartedAt.IsZero() {
		return record.StartedAt
	}
	if pkg.SubmissionState.InFlightStartedAt != nil {
		return *pkg.SubmissionState.InFlightStartedAt
	}
	return fallback
}

func submissionResponseForRecord(pkg *SheinPackage, action string) *sheinpub.SubmissionResponse {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.SubmissionState == nil {
		return nil
	}
	record := sheinSubmissionRecordForAction(pkg.SubmissionState, action)
	if record != nil && record.Result != nil {
		return record.Result
	}
	return pkg.SubmissionState.LastResult
}

func confirmedSubmissionResponse(response *sheinpub.SubmissionResponse, action string) *sheinpub.SubmissionResponse {
	if response != nil {
		return response
	}
	if action == "save_draft" {
		return &sheinpub.SubmissionResponse{Code: "0", Success: true, Message: "save draft confirmed by remote check"}
	}
	return &sheinpub.SubmissionResponse{Code: "0", Success: true, Message: "publish confirmed by remote check"}
}

func newSubmitRemoteActivityError(cause error, supplierCode string, response *sheinpub.SubmissionResponse, snapshot *sheinpub.SubmitSnapshot) error {
	details := SheinSubmitRemoteActivityErrorDetails{
		ErrorMessage: strings.TrimSpace(errorMessage(cause)),
		SupplierCode: supplierCode,
		Response:     response,
		Snapshot:     snapshot,
	}
	if details.ErrorMessage == "" {
		details.ErrorMessage = "shein submit remote failed"
	}
	return sdktemporal.NewNonRetryableApplicationError(
		details.ErrorMessage,
		SheinSubmitRemoteActivityErrorType,
		nil,
		details,
	)
}

func errorMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

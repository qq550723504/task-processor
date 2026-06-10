package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

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
}

func newTaskTemporalSubmissionAdapter(config taskTemporalSubmissionAdapterConfig) *taskTemporalSubmissionAdapter {
	return &taskTemporalSubmissionAdapter{
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
	}
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
	task, pkg, err := s.loadSheinPublishTask(ctx, in.TaskID)
	if err != nil {
		return err
	}
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	finalWasConfirmed := pkg.FinalSubmissionDraft != nil && pkg.FinalSubmissionDraft.Confirmed
	s.normalizeSheinSubmitPackage(task, pkg, sheinSubmitRequestFromActivity(in), in.Action)
	if in.ConfirmedFinal && !finalWasConfirmed {
		task.Result.UpdatedAt = time.Now()
		if err := s.saveTaskResult(ctx, in.TaskID, task.Result); err != nil {
			return err
		}
	}

	changed := ensureTaskPodExecution(task)
	if changed {
		task.Result.UpdatedAt = time.Now()
		if err := s.saveTaskResult(ctx, in.TaskID, task.Result); err != nil {
			return err
		}
	}
	readiness := buildSheinSubmitReadinessWithPodForAction(pkg, task.Result.PodExecution, in.Action)
	if err := validateSheinSubmitReadinessGates(ctx, task, pkg, in.Action, readiness, s.validateSheinPublishFreshness); err != nil {
		return err
	}
	return nil
}

func (s *taskTemporalSubmissionAdapter) BuildSheinTaskPreview(ctx context.Context, taskID string) (*ListingKitPreview, error) {
	return s.getTaskPreview(ctx, taskID, "shein")
}

func (s *taskTemporalSubmissionAdapter) buildSheinWorkflowReplayPreview(ctx context.Context, task *Task) (*ListingKitPreview, error) {
	if task == nil {
		return nil, ErrTaskResultUnavailable
	}
	return buildTaskPreviewFromTask(ctx, task, "shein", s.getTaskPreview)
}

func (s *taskTemporalSubmissionAdapter) persistSheinSubmitSnapshot(ctx context.Context, taskID string, result *ListingKitResult, pkg *SheinPackage, action, requestID string, snapshot *sheinpub.SubmitSnapshot) error {
	if result == nil || pkg == nil || snapshot == nil {
		return nil
	}
	setSheinSubmitSnapshot(pkg, action, requestID, snapshot)
	result.UpdatedAt = time.Now()
	return s.saveTaskResult(ctx, taskID, result)
}

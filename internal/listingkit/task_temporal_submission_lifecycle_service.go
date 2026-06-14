package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	sheinpub "task-processor/internal/publishing/shein"
)

type taskTemporalSubmissionLifecycleServiceConfig struct {
	startSheinPublishWorkflow     func(context.Context, SheinPublishWorkflowStartInput) error
	beginSheinSubmitLease         func(context.Context, string, string, string, time.Time) (*Task, error)
	loadSheinPublishTask          func(context.Context, string) (*Task, *SheinPackage, error)
	normalizeSheinSubmitPackage   func(*Task, *SheinPackage, *SubmitTaskRequest, string)
	validateSheinPublishFreshness func(context.Context, *Task, *SheinPackage, string) (*SheinSubmitReadiness, error)
	saveTaskResult                func(context.Context, string, *ListingKitResult) error
	handleWorkflowStartFailure    func(context.Context, string, *Task, sheinWorkflowSubmitOptions, error) error
	getTaskPreview                func(context.Context, string, string) (*ListingKitPreview, error)
}

type taskTemporalSubmissionLifecycleService struct {
	startSheinPublishWorkflow     func(context.Context, SheinPublishWorkflowStartInput) error
	beginSheinSubmitLease         func(context.Context, string, string, string, time.Time) (*Task, error)
	loadSheinPublishTask          func(context.Context, string) (*Task, *SheinPackage, error)
	normalizeSheinSubmitPackage   func(*Task, *SheinPackage, *SubmitTaskRequest, string)
	validateSheinPublishFreshness func(context.Context, *Task, *SheinPackage, string) (*SheinSubmitReadiness, error)
	saveTaskResult                func(context.Context, string, *ListingKitResult) error
	handleWorkflowStartFailure    func(context.Context, string, *Task, sheinWorkflowSubmitOptions, error) error
	getTaskPreview                func(context.Context, string, string) (*ListingKitPreview, error)
}

func newTaskTemporalSubmissionLifecycleService(config taskTemporalSubmissionLifecycleServiceConfig) *taskTemporalSubmissionLifecycleService {
	return &taskTemporalSubmissionLifecycleService{
		startSheinPublishWorkflow:     config.startSheinPublishWorkflow,
		beginSheinSubmitLease:         config.beginSheinSubmitLease,
		loadSheinPublishTask:          config.loadSheinPublishTask,
		normalizeSheinSubmitPackage:   config.normalizeSheinSubmitPackage,
		validateSheinPublishFreshness: config.validateSheinPublishFreshness,
		saveTaskResult:                config.saveTaskResult,
		handleWorkflowStartFailure:    config.handleWorkflowStartFailure,
		getTaskPreview:                config.getTaskPreview,
	}
}

func (s *service) loadSheinPublishTaskForTemporal(ctx context.Context, taskID string) (*Task, *SheinPackage, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, nil, err
	}
	if task.Result == nil {
		return nil, nil, ErrTaskResultUnavailable
	}
	pkg, ok := sheinpub.PreviewPayloadPackage(task.Result.Shein)
	if !ok {
		return nil, nil, fmt.Errorf("%w: shein preview payload is not available", ErrSubmitBlocked)
	}
	return task, pkg, nil
}

func (s *taskTemporalSubmissionLifecycleService) startSheinPublishWorkflowAttempt(ctx context.Context, taskID string, task *Task, req *SubmitTaskRequest, opts sheinWorkflowSubmitOptions) (*ListingKitPreview, error) {
	taskID = strings.TrimSpace(taskID)
	if s.startSheinPublishWorkflow == nil {
		return nil, fmt.Errorf("shein publish workflow start is not configured")
	}
	err := s.startSheinPublishWorkflow(ctx, SheinPublishWorkflowStartInput{
		TaskID:         taskID,
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

func (s *taskTemporalSubmissionLifecycleService) BeginSheinPublishAttempt(ctx context.Context, in SheinPublishAttemptInput) error {
	if in.Action == "" {
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

func (s *taskTemporalSubmissionLifecycleService) ValidateSheinPublishReadiness(ctx context.Context, in SheinPublishAttemptInput) error {
	state, err := s.loadSheinPreparedPublishState(ctx, in)
	if err != nil {
		return err
	}
	if state.execution == nil {
		return ErrTaskResultUnavailable
	}
	execution := state.execution
	pkg := sheinpub.NormalizePackageSemanticFields(execution.pkg)
	stateChanged := state.finalDraftConfirmedChanged
	if ensureTaskPodExecution(execution.task) {
		stateChanged = true
	}
	if stateChanged {
		execution.task.Result.UpdatedAt = time.Now()
		if s.saveTaskResult != nil {
			if err := s.saveTaskResult(ctx, execution.taskID, execution.task.Result); err != nil {
				return err
			}
		}
	}
	readiness := buildSheinSubmitReadinessWithPodForAction(pkg, taskPodExecution(execution.task), execution.action)
	return validateSheinSubmitReadinessGates(ctx, execution.task, pkg, execution.action, readiness, s.validateSheinPublishFreshness)
}

func (s *taskTemporalSubmissionLifecycleService) BuildSheinTaskPreview(ctx context.Context, taskID string) (*ListingKitPreview, error) {
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

func (s *taskTemporalSubmissionLifecycleService) buildSheinWorkflowReplayPreview(ctx context.Context, task *Task) (*ListingKitPreview, error) {
	if task == nil {
		return nil, ErrTaskResultUnavailable
	}
	return buildTaskPreviewFromTask(ctx, task, "shein", s.getTaskPreview)
}

func (s *taskTemporalSubmissionLifecycleService) loadSheinPublishTaskState(ctx context.Context, taskID string) (*Task, *SheinPackage, error) {
	if s.loadSheinPublishTask == nil {
		return nil, nil, fmt.Errorf("shein publish task loader is not configured")
	}
	return s.loadSheinPublishTask(ctx, taskID)
}

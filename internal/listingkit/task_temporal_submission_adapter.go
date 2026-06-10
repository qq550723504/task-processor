package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"task-processor/internal/listingkit/submission"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

type taskTemporalSubmissionAdapterConfig struct {
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
	rememberSheinSubmitted               func(*Task, string)
	getTaskPreview                       func(context.Context, string, string) (*ListingKitPreview, error)
}

type taskTemporalSubmissionAdapter struct {
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
	rememberSheinSubmitted               func(*Task, string)
	getTaskPreview                       func(context.Context, string, string) (*ListingKitPreview, error)
}

func newTaskTemporalSubmissionAdapter(config taskTemporalSubmissionAdapterConfig) *taskTemporalSubmissionAdapter {
	return &taskTemporalSubmissionAdapter{
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
		rememberSheinSubmitted:               config.rememberSheinSubmitted,
		getTaskPreview:                       config.getTaskPreview,
	}
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

func (s *taskTemporalSubmissionAdapter) PersistSheinPublishSuccess(ctx context.Context, in SheinPersistSubmitSuccessInput) error {
	task, pkg, err := s.loadSheinPublishTask(ctx, in.TaskID)
	if err != nil {
		return err
	}
	if in.Snapshot != nil {
		setSheinSubmitSnapshot(pkg, in.Action, in.RequestID, in.Snapshot)
	}
	setSheinSubmitRemoteResponse(pkg, in.Action, in.RequestID, in.SupplierCode, in.Response)
	task.Result.UpdatedAt = time.Now()
	if err := s.saveTaskResult(ctx, in.TaskID, task.Result); err != nil {
		return err
	}
	if err := s.persistSheinSubmitPhase(ctx, in.TaskID, task.Result, pkg, in.Action, in.RequestID, sheinpub.SubmissionPhasePersistResult); err != nil {
		return err
	}

	startedAt := sheinSubmitStartedAt(pkg, in.Action, in.RequestID, time.Now())
	record := completeSheinSubmitAttempt(pkg, in.Action, in.RequestID, in.Response, nil, time.Now())
	appendSheinSubmissionEvent(pkg, submission.BuildEvent(in.TaskID, in.Action, record, in.Response, nil, startedAt))
	if s.rememberSheinSubmitted != nil {
		s.rememberSheinSubmitted(task, in.Action)
	}
	return s.persistSuccessfulSheinSubmission(ctx, in.TaskID, task, in.Action)
}

func (s *taskTemporalSubmissionAdapter) PersistSheinPublishFailure(ctx context.Context, in SheinPersistSubmitFailureInput) error {
	task, pkg, err := s.loadSheinPublishTask(ctx, in.TaskID)
	if err != nil {
		return err
	}
	if in.Snapshot != nil {
		setSheinSubmitSnapshot(pkg, in.Action, in.RequestID, in.Snapshot)
	}
	if in.SupplierCode != "" {
		setSheinSubmitSupplierCode(pkg, in.Action, in.RequestID, in.SupplierCode)
	}
	if in.Response != nil {
		setSheinSubmitRemoteResponse(pkg, in.Action, in.RequestID, in.SupplierCode, in.Response)
	}
	return s.recordSheinSubmissionFailureForState(
		ctx,
		in.TaskID,
		task.Result,
		pkg,
		in.Action,
		in.RequestID,
		in.Phase,
		errors.New(strings.TrimSpace(in.ErrorMessage)),
	)
}

func (s *taskTemporalSubmissionAdapter) RefreshSheinPublishRemoteStatus(ctx context.Context, in SheinRefreshRemoteStatusInput) (*SheinRefreshRemoteStatusResult, error) {
	task, pkg, err := s.loadSheinPublishTask(ctx, in.TaskID)
	if err != nil {
		return nil, err
	}
	productAPI, err := s.buildSheinSubmitProductAPI(ctx, task)
	if err != nil {
		return nil, err
	}
	if err := s.persistSheinSubmitPhase(ctx, in.TaskID, task.Result, pkg, in.Action, in.RequestID, sheinpub.SubmissionPhaseConfirmRemote); err != nil {
		return nil, err
	}

	startedAt := sheinSubmitStartedAt(pkg, in.Action, in.RequestID, time.Now())
	remoteEvent, remoteErr := s.refreshSheinSubmitRemoteStatus(ctx, task, in.TaskID, pkg, productAPI, in.Action, in.RequestID, in.SupplierCode, startedAt)
	if remoteEvent != nil {
		appendSheinSubmissionEvent(pkg, *remoteEvent)
	}

	record := sheinSubmissionRecordForAction(pkg.SubmissionState, in.Action)
	response := submissionResponseForRecord(pkg, in.Action)
	if remoteErr != nil {
		record = failSheinSubmitAttempt(pkg, in.Action, in.RequestID, sheinpub.SubmissionPhaseConfirmRemote, remoteErr, time.Now())
		appendSheinSubmissionEvent(pkg, submission.BuildEvent(in.TaskID, in.Action, record, response, remoteErr, record.StartedAt))
		task.Result.UpdatedAt = time.Now()
		if err := s.saveTaskResult(ctx, in.TaskID, task.Result); err != nil {
			return nil, err
		}
		return nil, remoteErr
	}

	response = confirmedSubmissionResponse(response, in.Action)
	record = completeSheinSubmitAttempt(pkg, in.Action, in.RequestID, response, nil, time.Now())
	appendSheinSubmissionEvent(pkg, submission.BuildEvent(in.TaskID, in.Action, record, record.Result, nil, record.StartedAt))
	if s.rememberSheinSubmitted != nil {
		s.rememberSheinSubmitted(task, in.Action)
	}
	if err := s.persistSuccessfulSheinSubmission(ctx, in.TaskID, task, in.Action); err != nil {
		return nil, err
	}

	remoteStatus := ""
	if pkg.SubmissionState != nil {
		remoteStatus = pkg.SubmissionState.RemoteStatus
	}
	return &SheinRefreshRemoteStatusResult{
		TaskID:       in.TaskID,
		Action:       in.Action,
		RequestID:    in.RequestID,
		RemoteStatus: remoteStatus,
	}, nil
}

func (s *taskTemporalSubmissionAdapter) BuildSheinTaskPreview(ctx context.Context, taskID string) (*ListingKitPreview, error) {
	return s.getTaskPreview(ctx, taskID, "shein")
}

func (s *taskTemporalSubmissionAdapter) persistSheinSubmitSnapshot(ctx context.Context, taskID string, result *ListingKitResult, pkg *SheinPackage, action, requestID string, snapshot *sheinpub.SubmitSnapshot) error {
	if result == nil || pkg == nil || snapshot == nil {
		return nil
	}
	setSheinSubmitSnapshot(pkg, action, requestID, snapshot)
	result.UpdatedAt = time.Now()
	return s.saveTaskResult(ctx, taskID, result)
}

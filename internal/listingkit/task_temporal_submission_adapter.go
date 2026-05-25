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
	beginSheinSubmitLease                func(context.Context, string, string, string, time.Time) (*Task, error)
	loadSheinPublishTask                 func(context.Context, string) (*Task, *SheinPackage, error)
	normalizeSheinSubmitPackage          func(*Task, *SheinPackage, *SubmitTaskRequest, string)
	saveTaskResult                       func(context.Context, string, *ListingKitResult) error
	persistSheinSubmitPhase              func(context.Context, string, *ListingKitResult, *SheinPackage, string, string, string) error
	prepareSheinSubmitProduct            func(context.Context, *Task, *SheinPackage, string) (*sheinproduct.Product, error)
	uploadSheinSubmitImages              func(context.Context, *Task, *SheinPackage, *sheinproduct.Product) error
	resolveSubmitSettings                func(context.Context, *Task) SheinSettings
	buildSheinSubmitProductAPI           func(context.Context, *Task) (sheinproduct.ProductAPI, error)
	retrySheinSensitiveWordSubmit        func(context.Context, string, *SheinPackage, string, string, sheinproduct.ProductAPI, *sheinproduct.Product, *sheinpub.SubmissionResponse, error) (*sheinpub.SubmissionResponse, error, bool)
	persistSuccessfulSheinSubmission     func(context.Context, string, *Task, string) error
	recordSheinSubmissionFailureForState func(context.Context, string, *ListingKitResult, *SheinPackage, string, string, string, error) error
	refreshSheinSubmitRemoteStatus       func(context.Context, string, *SheinPackage, sheinproduct.ProductAPI, string, string, string, time.Time) (*sheinpub.SubmissionEvent, error)
	rememberSheinSubmitted               func(*Task, string)
	getTaskPreview                       func(context.Context, string, string) (*ListingKitPreview, error)
}

type taskTemporalSubmissionAdapter struct {
	beginSheinSubmitLease                func(context.Context, string, string, string, time.Time) (*Task, error)
	loadSheinPublishTask                 func(context.Context, string) (*Task, *SheinPackage, error)
	normalizeSheinSubmitPackage          func(*Task, *SheinPackage, *SubmitTaskRequest, string)
	saveTaskResult                       func(context.Context, string, *ListingKitResult) error
	persistSheinSubmitPhase              func(context.Context, string, *ListingKitResult, *SheinPackage, string, string, string) error
	prepareSheinSubmitProduct            func(context.Context, *Task, *SheinPackage, string) (*sheinproduct.Product, error)
	uploadSheinSubmitImages              func(context.Context, *Task, *SheinPackage, *sheinproduct.Product) error
	resolveSubmitSettings                func(context.Context, *Task) SheinSettings
	buildSheinSubmitProductAPI           func(context.Context, *Task) (sheinproduct.ProductAPI, error)
	retrySheinSensitiveWordSubmit        func(context.Context, string, *SheinPackage, string, string, sheinproduct.ProductAPI, *sheinproduct.Product, *sheinpub.SubmissionResponse, error) (*sheinpub.SubmissionResponse, error, bool)
	persistSuccessfulSheinSubmission     func(context.Context, string, *Task, string) error
	recordSheinSubmissionFailureForState func(context.Context, string, *ListingKitResult, *SheinPackage, string, string, string, error) error
	refreshSheinSubmitRemoteStatus       func(context.Context, string, *SheinPackage, sheinproduct.ProductAPI, string, string, string, time.Time) (*sheinpub.SubmissionEvent, error)
	rememberSheinSubmitted               func(*Task, string)
	getTaskPreview                       func(context.Context, string, string) (*ListingKitPreview, error)
}

func newTaskTemporalSubmissionAdapter(config taskTemporalSubmissionAdapterConfig) *taskTemporalSubmissionAdapter {
	return &taskTemporalSubmissionAdapter{
		beginSheinSubmitLease:                config.beginSheinSubmitLease,
		loadSheinPublishTask:                 config.loadSheinPublishTask,
		normalizeSheinSubmitPackage:          config.normalizeSheinSubmitPackage,
		saveTaskResult:                       config.saveTaskResult,
		persistSheinSubmitPhase:              config.persistSheinSubmitPhase,
		prepareSheinSubmitProduct:            config.prepareSheinSubmitProduct,
		uploadSheinSubmitImages:              config.uploadSheinSubmitImages,
		resolveSubmitSettings:                config.resolveSubmitSettings,
		buildSheinSubmitProductAPI:           config.buildSheinSubmitProductAPI,
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
		return fmt.Errorf("%w: shein preview_product is not available", ErrSubmitBlocked)
	}
	return err
}

func (s *taskTemporalSubmissionAdapter) ValidateSheinPublishReadiness(ctx context.Context, in SheinPublishAttemptInput) error {
	task, pkg, err := s.loadSheinPublishTask(ctx, in.TaskID)
	if err != nil {
		return err
	}
	finalWasConfirmed := pkg.FinalDraft != nil && pkg.FinalDraft.Confirmed
	s.normalizeSheinSubmitPackage(task, pkg, sheinSubmitRequestFromActivity(in), in.Action)
	if in.ConfirmedFinal && !finalWasConfirmed {
		task.Result.UpdatedAt = time.Now()
		if err := s.saveTaskResult(ctx, in.TaskID, task.Result); err != nil {
			return err
		}
	}

	readiness := buildSheinSubmitReadinessForAction(pkg, in.Action)
	if readiness != nil && readiness.Ready {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrSubmitBlocked, firstSubmitReadinessMessage(readiness))
}

func (s *taskTemporalSubmissionAdapter) PrepareSheinPublishPayload(ctx context.Context, in SheinPublishAttemptInput) (*SheinPreparedSubmitPayload, error) {
	task, pkg, err := s.loadSheinPublishTask(ctx, in.TaskID)
	if err != nil {
		return nil, err
	}
	s.normalizeSheinSubmitPackage(task, pkg, sheinSubmitRequestFromActivity(in), in.Action)
	if err := s.persistSheinSubmitPhase(ctx, in.TaskID, task.Result, pkg, in.Action, in.RequestID, sheinpub.SubmissionPhasePrepareProduct); err != nil {
		return nil, err
	}

	submitProduct, err := s.prepareSheinSubmitProduct(ctx, task, pkg, in.Action)
	if err != nil {
		return nil, err
	}
	dumpSheinSubmitPayloadForDebug(in.TaskID, in.Action, in.RequestID, "prepared", submitProduct)
	snapshot := sheinpub.BuildSubmitSnapshot(submitProduct)
	if err := s.persistSheinSubmitSnapshot(ctx, in.TaskID, task.Result, pkg, in.Action, in.RequestID, snapshot); err != nil {
		return nil, err
	}

	return &SheinPreparedSubmitPayload{
		TaskID:           in.TaskID,
		Action:           in.Action,
		RequestID:        in.RequestID,
		Product:          submitProduct,
		NeedsImageUpload: sheinProductPendingImageUploadCount(submitProduct) > 0,
		Snapshot:         snapshot,
	}, nil
}

func (s *taskTemporalSubmissionAdapter) UploadSheinPublishImages(ctx context.Context, in *SheinPreparedSubmitPayload) (*SheinPreparedSubmitPayload, error) {
	if in == nil || in.Product == nil {
		return nil, fmt.Errorf("shein publish payload is required")
	}
	if !in.NeedsImageUpload {
		return in, nil
	}

	task, pkg, err := s.loadSheinPublishTask(ctx, in.TaskID)
	if err != nil {
		return nil, err
	}
	if err := s.persistSheinSubmitPhase(ctx, in.TaskID, task.Result, pkg, in.Action, in.RequestID, sheinpub.SubmissionPhaseUploadImages); err != nil {
		return nil, err
	}
	if err := s.uploadSheinSubmitImages(ctx, task, pkg, in.Product); err != nil {
		return nil, err
	}
	prepareSheinProductForSubmit(in.Product, s.resolveSubmitSettings(ctx, task))
	dumpSheinSubmitPayloadForDebug(in.TaskID, in.Action, in.RequestID, "uploaded", in.Product)

	snapshot := sheinpub.BuildSubmitSnapshot(in.Product)
	if err := s.persistSheinSubmitSnapshot(ctx, in.TaskID, task.Result, pkg, in.Action, in.RequestID, snapshot); err != nil {
		return nil, err
	}

	out := *in
	out.NeedsImageUpload = false
	out.Snapshot = snapshot
	return &out, nil
}

func (s *taskTemporalSubmissionAdapter) PreValidateSheinPublish(ctx context.Context, in *SheinPreparedSubmitPayload) error {
	if in == nil || in.Product == nil {
		return fmt.Errorf("shein publish payload is required")
	}
	task, pkg, err := s.loadSheinPublishTask(ctx, in.TaskID)
	if err != nil {
		return err
	}
	if err := s.persistSheinSubmitPhase(ctx, in.TaskID, task.Result, pkg, in.Action, in.RequestID, sheinpub.SubmissionPhasePreValidate); err != nil {
		return err
	}
	return preValidateSheinSubmitProduct(in.Product)
}

func (s *taskTemporalSubmissionAdapter) SubmitSheinPublishRemote(ctx context.Context, in *SheinPreparedSubmitPayload) (*SheinRemoteSubmitResult, error) {
	if in == nil || in.Product == nil {
		return nil, fmt.Errorf("shein publish payload is required")
	}
	task, pkg, err := s.loadSheinPublishTask(ctx, in.TaskID)
	if err != nil {
		return nil, err
	}
	productAPI, err := s.buildSheinSubmitProductAPI(ctx, task)
	if err != nil {
		return nil, err
	}

	supplierCode := sheinSubmitSupplierCode(in.Product, pkg)
	snapshot := in.Snapshot
	if snapshot == nil {
		snapshot = sheinpub.BuildSubmitSnapshot(in.Product)
	}
	setSheinSubmitSupplierCode(pkg, in.Action, in.RequestID, supplierCode)
	setSheinSubmitSnapshot(pkg, in.Action, in.RequestID, snapshot)
	if err := s.persistSheinSubmitPhase(ctx, in.TaskID, task.Result, pkg, in.Action, in.RequestID, sheinpub.SubmissionPhaseSubmitRemote); err != nil {
		return nil, err
	}

	response, responseErr := executeSheinSubmitRemote(productAPI, in.Action, in.Product)
	if responseErr == nil {
		responseErr = buildSheinSubmitResponseError(in.Action, response)
	}
	if retryResponse, retryErr, retried := s.retrySheinSensitiveWordSubmit(ctx, in.TaskID, pkg, in.Action, in.RequestID, productAPI, in.Product, response, responseErr); retried {
		response = retryResponse
		responseErr = retryErr
		snapshot = sheinpub.BuildSubmitSnapshot(in.Product)
		setSheinSubmitSnapshot(pkg, in.Action, in.RequestID, snapshot)
	}
	if responseErr != nil {
		return nil, newSubmitRemoteActivityError(responseErr, supplierCode, response, snapshot)
	}

	return &SheinRemoteSubmitResult{
		TaskID:       in.TaskID,
		Action:       in.Action,
		RequestID:    in.RequestID,
		SupplierCode: supplierCode,
		Response:     response,
		Snapshot:     snapshot,
	}, nil
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
	appendSheinSubmissionEvent(pkg, buildSheinSubmissionEvent(in.TaskID, in.Action, record, in.Response, nil, startedAt))
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
	remoteEvent, remoteErr := s.refreshSheinSubmitRemoteStatus(ctx, in.TaskID, pkg, productAPI, in.Action, in.RequestID, in.SupplierCode, startedAt)
	if remoteEvent != nil {
		appendSheinSubmissionEvent(pkg, *remoteEvent)
	}

	record := sheinSubmissionRecordForAction(pkg.Submission, in.Action)
	response := submissionResponseForRecord(pkg, in.Action)
	if remoteErr != nil {
		record = failSheinSubmitAttempt(pkg, in.Action, in.RequestID, sheinpub.SubmissionPhaseConfirmRemote, remoteErr, time.Now())
		appendSheinSubmissionEvent(pkg, buildSheinSubmissionEvent(in.TaskID, in.Action, record, response, remoteErr, record.StartedAt))
		task.Result.UpdatedAt = time.Now()
		if err := s.saveTaskResult(ctx, in.TaskID, task.Result); err != nil {
			return nil, err
		}
		return nil, remoteErr
	}

	response = confirmedSubmissionResponse(response, in.Action)
	record = completeSheinSubmitAttempt(pkg, in.Action, in.RequestID, response, nil, time.Now())
	appendSheinSubmissionEvent(pkg, buildSheinSubmissionEvent(in.TaskID, in.Action, record, record.Result, nil, record.StartedAt))
	if s.rememberSheinSubmitted != nil {
		s.rememberSheinSubmitted(task, in.Action)
	}
	if err := s.persistSuccessfulSheinSubmission(ctx, in.TaskID, task, in.Action); err != nil {
		return nil, err
	}

	remoteStatus := ""
	if pkg.Submission != nil {
		remoteStatus = pkg.Submission.RemoteStatus
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

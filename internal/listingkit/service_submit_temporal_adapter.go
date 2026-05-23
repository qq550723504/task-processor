package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	sdktemporal "go.temporal.io/sdk/temporal"
	sheinpub "task-processor/internal/publishing/shein"
)

func (s *service) BeginSheinPublishAttempt(ctx context.Context, in SheinPublishAttemptInput) error {
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

func (s *service) ValidateSheinPublishReadiness(ctx context.Context, in SheinPublishAttemptInput) error {
	task, pkg, err := s.loadSheinPublishTask(ctx, in.TaskID)
	if err != nil {
		return err
	}
	finalWasConfirmed := pkg.FinalDraft != nil && pkg.FinalDraft.Confirmed
	s.normalizeSheinSubmitPackage(task, pkg, sheinSubmitRequestFromActivity(in), in.Action)
	if in.ConfirmedFinal && !finalWasConfirmed {
		task.Result.UpdatedAt = time.Now()
		if err := s.repo.SaveTaskResult(ctx, in.TaskID, task.Result); err != nil {
			return err
		}
	}

	readiness := buildSheinSubmitReadinessForAction(pkg, in.Action)
	if readiness != nil && readiness.Ready {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrSubmitBlocked, firstSubmitReadinessMessage(readiness))
}

func (s *service) PrepareSheinPublishPayload(ctx context.Context, in SheinPublishAttemptInput) (*SheinPreparedSubmitPayload, error) {
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

func (s *service) UploadSheinPublishImages(ctx context.Context, in *SheinPreparedSubmitPayload) (*SheinPreparedSubmitPayload, error) {
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
	prepareSheinProductForSubmit(in.Product, s.resolveSheinSubmitSettings(ctx, task))
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

func (s *service) PreValidateSheinPublish(ctx context.Context, in *SheinPreparedSubmitPayload) error {
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

func (s *service) SubmitSheinPublishRemote(ctx context.Context, in *SheinPreparedSubmitPayload) (*SheinRemoteSubmitResult, error) {
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

func (s *service) PersistSheinPublishSuccess(ctx context.Context, in SheinPersistSubmitSuccessInput) error {
	task, pkg, err := s.loadSheinPublishTask(ctx, in.TaskID)
	if err != nil {
		return err
	}
	if in.Snapshot != nil {
		setSheinSubmitSnapshot(pkg, in.Action, in.RequestID, in.Snapshot)
	}
	setSheinSubmitRemoteResponse(pkg, in.Action, in.RequestID, in.SupplierCode, in.Response)
	task.Result.UpdatedAt = time.Now()
	if err := s.repo.SaveTaskResult(ctx, in.TaskID, task.Result); err != nil {
		return err
	}
	if err := s.persistSheinSubmitPhase(ctx, in.TaskID, task.Result, pkg, in.Action, in.RequestID, sheinpub.SubmissionPhasePersistResult); err != nil {
		return err
	}

	startedAt := sheinSubmitStartedAt(pkg, in.Action, in.RequestID, time.Now())
	record := completeSheinSubmitAttempt(pkg, in.Action, in.RequestID, in.Response, nil, time.Now())
	appendSheinSubmissionEvent(pkg, buildSheinSubmissionEvent(in.TaskID, in.Action, record, in.Response, nil, startedAt))
	s.rememberSheinSubmittedResolution(task, in.Action)
	return s.persistSuccessfulSheinSubmission(ctx, in.TaskID, task, in.Action)
}

func (s *service) PersistSheinPublishFailure(ctx context.Context, in SheinPersistSubmitFailureInput) error {
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

func (s *service) RefreshSheinPublishRemoteStatus(ctx context.Context, in SheinRefreshRemoteStatusInput) (*SheinRefreshRemoteStatusResult, error) {
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
		if err := s.repo.SaveTaskResult(ctx, in.TaskID, task.Result); err != nil {
			return nil, err
		}
		return nil, remoteErr
	}

	response = confirmedSubmissionResponse(response, in.Action)
	record = completeSheinSubmitAttempt(pkg, in.Action, in.RequestID, response, nil, time.Now())
	appendSheinSubmissionEvent(pkg, buildSheinSubmissionEvent(in.TaskID, in.Action, record, record.Result, nil, record.StartedAt))
	s.rememberSheinSubmittedResolution(task, in.Action)
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

func (s *service) BuildSheinTaskPreview(ctx context.Context, taskID string) (*ListingKitPreview, error) {
	return s.GetTaskPreview(ctx, taskID, "shein")
}

func (s *service) loadSheinPublishTask(ctx context.Context, taskID string) (*Task, *SheinPackage, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, nil, err
	}
	if task.Result == nil {
		return nil, nil, ErrTaskResultUnavailable
	}
	pkg := task.Result.Shein
	if pkg == nil || pkg.PreviewProduct == nil {
		return nil, nil, fmt.Errorf("%w: shein preview_product is not available", ErrSubmitBlocked)
	}
	return task, pkg, nil
}

func (s *service) persistSheinSubmitSnapshot(ctx context.Context, taskID string, result *ListingKitResult, pkg *SheinPackage, action, requestID string, snapshot *sheinpub.SubmitSnapshot) error {
	if result == nil || pkg == nil || snapshot == nil {
		return nil
	}
	setSheinSubmitSnapshot(pkg, action, requestID, snapshot)
	result.UpdatedAt = time.Now()
	return s.repo.SaveTaskResult(ctx, taskID, result)
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
	if pkg == nil || pkg.Submission == nil {
		return fallback
	}
	record := sheinSubmissionRecordForAction(pkg.Submission, action)
	if record != nil && record.RequestID == requestID && !record.StartedAt.IsZero() {
		return record.StartedAt
	}
	if pkg.Submission.InFlightStartedAt != nil {
		return *pkg.Submission.InFlightStartedAt
	}
	return fallback
}

func submissionResponseForRecord(pkg *SheinPackage, action string) *sheinpub.SubmissionResponse {
	if pkg == nil || pkg.Submission == nil {
		return nil
	}
	record := sheinSubmissionRecordForAction(pkg.Submission, action)
	if record != nil && record.Result != nil {
		return record.Result
	}
	return pkg.Submission.LastResult
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

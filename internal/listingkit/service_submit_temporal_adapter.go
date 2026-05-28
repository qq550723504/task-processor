package listingkit

import (
	"context"
	"fmt"
	"strings"
	"time"

	sdktemporal "go.temporal.io/sdk/temporal"
	sheinpub "task-processor/internal/publishing/shein"
)

func (s *service) BeginSheinPublishAttempt(ctx context.Context, in SheinPublishAttemptInput) error {
	return s.taskTemporalSubmissionAdapterOrDefault().BeginSheinPublishAttempt(ctx, in)
}

func (s *service) ValidateSheinPublishReadiness(ctx context.Context, in SheinPublishAttemptInput) error {
	return s.taskTemporalSubmissionAdapterOrDefault().ValidateSheinPublishReadiness(ctx, in)
}

func (s *service) PrepareSheinPublishPayload(ctx context.Context, in SheinPublishAttemptInput) (*SheinPreparedSubmitPayload, error) {
	return s.taskTemporalSubmissionAdapterOrDefault().PrepareSheinPublishPayload(ctx, in)
}

func (s *service) UploadSheinPublishImages(ctx context.Context, in *SheinPreparedSubmitPayload) (*SheinPreparedSubmitPayload, error) {
	return s.taskTemporalSubmissionAdapterOrDefault().UploadSheinPublishImages(ctx, in)
}

func (s *service) PreValidateSheinPublish(ctx context.Context, in *SheinPreparedSubmitPayload) error {
	return s.taskTemporalSubmissionAdapterOrDefault().PreValidateSheinPublish(ctx, in)
}

func (s *service) SubmitSheinPublishRemote(ctx context.Context, in *SheinPreparedSubmitPayload) (*SheinRemoteSubmitResult, error) {
	return s.taskTemporalSubmissionAdapterOrDefault().SubmitSheinPublishRemote(ctx, in)
}

func (s *service) PersistSheinPublishSuccess(ctx context.Context, in SheinPersistSubmitSuccessInput) error {
	return s.taskTemporalSubmissionAdapterOrDefault().PersistSheinPublishSuccess(ctx, in)
}

func (s *service) PersistSheinPublishFailure(ctx context.Context, in SheinPersistSubmitFailureInput) error {
	return s.taskTemporalSubmissionAdapterOrDefault().PersistSheinPublishFailure(ctx, in)
}

func (s *service) RefreshSheinPublishRemoteStatus(ctx context.Context, in SheinRefreshRemoteStatusInput) (*SheinRefreshRemoteStatusResult, error) {
	return s.taskTemporalSubmissionAdapterOrDefault().RefreshSheinPublishRemoteStatus(ctx, in)
}

func (s *service) BuildSheinTaskPreview(ctx context.Context, taskID string) (*ListingKitPreview, error) {
	return s.taskTemporalSubmissionAdapterOrDefault().BuildSheinTaskPreview(ctx, taskID)
}

func (s *service) taskTemporalSubmissionAdapterOrDefault() *taskTemporalSubmissionAdapter {
	if s.taskTemporalSubmissionAdapter != nil {
		return s.taskTemporalSubmissionAdapter
	}
	s.taskTemporalSubmissionAdapter = newTaskTemporalSubmissionAdapter(taskTemporalSubmissionAdapterConfig{
		beginSheinSubmitLease:                s.beginSheinSubmitLease,
		loadSheinPublishTask:                 s.loadSheinPublishTask,
		normalizeSheinSubmitPackage:          s.normalizeSheinSubmitPackage,
		validateSheinPublishFreshness:        s.validateSheinPublishFreshness,
		saveTaskResult:                       s.repo.SaveTaskResult,
		persistSheinSubmitPhase:              s.persistSheinSubmitPhase,
		prepareSheinSubmitProduct:            s.prepareSheinSubmitProduct,
		uploadSheinSubmitImages:              s.uploadSheinSubmitImages,
		resolveSubmitSettings:                s.resolveSheinSubmitSettings,
		buildSheinSubmitProductAPI:           s.buildSheinSubmitProductAPI,
		retrySheinSensitiveWordSubmit:        s.retrySheinSensitiveWordSubmit,
		persistSuccessfulSheinSubmission:     s.persistSuccessfulSheinSubmission,
		recordSheinSubmissionFailureForState: s.recordSheinSubmissionFailureForState,
		refreshSheinSubmitRemoteStatus:       s.refreshSheinSubmitRemoteStatus,
		rememberSheinSubmitted:               s.rememberSheinSubmittedResolution,
		getTaskPreview:                       s.GetTaskPreview,
	})
	return s.taskTemporalSubmissionAdapter
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
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.PreviewPayload == nil {
		return nil, nil, fmt.Errorf("%w: shein preview payload is not available", ErrSubmitBlocked)
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

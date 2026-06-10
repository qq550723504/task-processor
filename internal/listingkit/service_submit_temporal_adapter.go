package listingkit

import (
	"context"
	"fmt"

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
	if s.submission.taskTemporalSubmissionAdapter != nil {
		return s.submission.taskTemporalSubmissionAdapter
	}
	s.submission.taskTemporalSubmissionAdapter = newTaskTemporalSubmissionAdapter(buildTaskTemporalSubmissionAdapterConfig(s))
	return s.submission.taskTemporalSubmissionAdapter
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

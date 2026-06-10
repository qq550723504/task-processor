package listingkit

import "context"

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

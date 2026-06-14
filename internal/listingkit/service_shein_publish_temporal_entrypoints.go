package listingkit

import "context"

func (s *service) BeginSheinPublishAttempt(ctx context.Context, in SheinPublishAttemptInput) error {
	temporal := s.taskTemporalSubmissionOrDefault()
	if temporal == nil {
		return ErrTaskResultUnavailable
	}
	return temporal.BeginSheinPublishAttempt(ctx, in)
}

func (s *service) ValidateSheinPublishReadiness(ctx context.Context, in SheinPublishAttemptInput) error {
	temporal := s.taskTemporalSubmissionOrDefault()
	if temporal == nil {
		return ErrTaskResultUnavailable
	}
	return temporal.ValidateSheinPublishReadiness(ctx, in)
}

func (s *service) PrepareSheinPublishPayload(ctx context.Context, in SheinPublishAttemptInput) (*SheinPreparedSubmitPayload, error) {
	temporal := s.taskTemporalSubmissionOrDefault()
	if temporal == nil {
		return nil, ErrTaskResultUnavailable
	}
	return temporal.PrepareSheinPublishPayload(ctx, in)
}

func (s *service) UploadSheinPublishImages(ctx context.Context, in *SheinPreparedSubmitPayload) (*SheinPreparedSubmitPayload, error) {
	temporal := s.taskTemporalSubmissionOrDefault()
	if temporal == nil {
		return nil, ErrTaskResultUnavailable
	}
	return temporal.UploadSheinPublishImages(ctx, in)
}

func (s *service) PreValidateSheinPublish(ctx context.Context, in *SheinPreparedSubmitPayload) error {
	temporal := s.taskTemporalSubmissionOrDefault()
	if temporal == nil {
		return ErrTaskResultUnavailable
	}
	return temporal.PreValidateSheinPublish(ctx, in)
}

func (s *service) SubmitSheinPublishRemote(ctx context.Context, in *SheinPreparedSubmitPayload) (*SheinRemoteSubmitResult, error) {
	temporal := s.taskTemporalSubmissionOrDefault()
	if temporal == nil {
		return nil, ErrTaskResultUnavailable
	}
	return temporal.SubmitSheinPublishRemote(ctx, in)
}

func (s *service) PersistSheinPublishSuccess(ctx context.Context, in SheinPersistSubmitSuccessInput) error {
	temporal := s.taskTemporalSubmissionOrDefault()
	if temporal == nil {
		return nil
	}
	return temporal.PersistSheinPublishSuccess(ctx, in)
}

func (s *service) PersistSheinPublishFailure(ctx context.Context, in SheinPersistSubmitFailureInput) error {
	temporal := s.taskTemporalSubmissionOrDefault()
	if temporal == nil {
		return nil
	}
	return temporal.PersistSheinPublishFailure(ctx, in)
}

func (s *service) RefreshSheinPublishRemoteStatus(ctx context.Context, in SheinRefreshRemoteStatusInput) (*SheinRefreshRemoteStatusResult, error) {
	temporal := s.taskTemporalSubmissionOrDefault()
	if temporal == nil {
		return nil, ErrTaskResultUnavailable
	}
	return temporal.RefreshSheinPublishRemoteStatus(ctx, in)
}

func (s *service) BuildSheinTaskPreview(ctx context.Context, taskID string) (*ListingKitPreview, error) {
	temporal := s.taskTemporalSubmissionOrDefault()
	if temporal == nil {
		return nil, ErrTaskResultUnavailable
	}
	return temporal.BuildSheinTaskPreview(ctx, taskID)
}

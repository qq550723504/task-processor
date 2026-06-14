package listingkit

import "context"

func (s *service) BeginSheinPublishAttempt(ctx context.Context, in SheinPublishAttemptInput) error {
	lifecycle := s.taskTemporalSubmissionLifecycleOrDefault()
	if lifecycle == nil {
		return ErrTaskResultUnavailable
	}
	return lifecycle.BeginSheinPublishAttempt(ctx, in)
}

func (s *service) ValidateSheinPublishReadiness(ctx context.Context, in SheinPublishAttemptInput) error {
	lifecycle := s.taskTemporalSubmissionLifecycleOrDefault()
	if lifecycle == nil {
		return ErrTaskResultUnavailable
	}
	return lifecycle.ValidateSheinPublishReadiness(ctx, in)
}

func (s *service) PrepareSheinPublishPayload(ctx context.Context, in SheinPublishAttemptInput) (*SheinPreparedSubmitPayload, error) {
	flow := s.taskTemporalSubmissionFlowOrDefault()
	if flow == nil {
		return nil, ErrTaskResultUnavailable
	}
	return flow.PrepareSheinPublishPayload(ctx, in)
}

func (s *service) UploadSheinPublishImages(ctx context.Context, in *SheinPreparedSubmitPayload) (*SheinPreparedSubmitPayload, error) {
	flow := s.taskTemporalSubmissionFlowOrDefault()
	if flow == nil {
		return nil, ErrTaskResultUnavailable
	}
	return flow.UploadSheinPublishImages(ctx, in)
}

func (s *service) PreValidateSheinPublish(ctx context.Context, in *SheinPreparedSubmitPayload) error {
	flow := s.taskTemporalSubmissionFlowOrDefault()
	if flow == nil {
		return ErrTaskResultUnavailable
	}
	return flow.PreValidateSheinPublish(ctx, in)
}

func (s *service) SubmitSheinPublishRemote(ctx context.Context, in *SheinPreparedSubmitPayload) (*SheinRemoteSubmitResult, error) {
	flow := s.taskTemporalSubmissionFlowOrDefault()
	if flow == nil {
		return nil, ErrTaskResultUnavailable
	}
	return flow.SubmitSheinPublishRemote(ctx, in)
}

func (s *service) PersistSheinPublishSuccess(ctx context.Context, in SheinPersistSubmitSuccessInput) error {
	persistence := s.taskTemporalSubmissionPersistenceOrDefault()
	if persistence == nil {
		return nil
	}
	return persistence.PersistSheinPublishSuccess(ctx, in)
}

func (s *service) PersistSheinPublishFailure(ctx context.Context, in SheinPersistSubmitFailureInput) error {
	persistence := s.taskTemporalSubmissionPersistenceOrDefault()
	if persistence == nil {
		return nil
	}
	return persistence.PersistSheinPublishFailure(ctx, in)
}

func (s *service) RefreshSheinPublishRemoteStatus(ctx context.Context, in SheinRefreshRemoteStatusInput) (*SheinRefreshRemoteStatusResult, error) {
	refresh := s.taskTemporalSubmissionRefreshOrDefault()
	if refresh == nil {
		return nil, ErrTaskResultUnavailable
	}
	return refresh.RefreshSheinPublishRemoteStatus(ctx, in)
}

func (s *service) BuildSheinTaskPreview(ctx context.Context, taskID string) (*ListingKitPreview, error) {
	lifecycle := s.taskTemporalSubmissionLifecycleOrDefault()
	if lifecycle == nil {
		return nil, ErrTaskResultUnavailable
	}
	return lifecycle.BuildSheinTaskPreview(ctx, taskID)
}

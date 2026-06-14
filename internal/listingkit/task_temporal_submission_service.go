package listingkit

import "context"

type taskTemporalSubmissionServiceConfig struct {
	lifecycle   *taskTemporalSubmissionLifecycleService
	flow        *taskTemporalSubmissionFlowService
	persistence *taskTemporalSubmissionPersistenceService
	refresh     *taskTemporalSubmissionRefreshService
}

type taskTemporalSubmissionService struct {
	lifecycle   *taskTemporalSubmissionLifecycleService
	flow        *taskTemporalSubmissionFlowService
	persistence *taskTemporalSubmissionPersistenceService
	refresh     *taskTemporalSubmissionRefreshService
}

func newTaskTemporalSubmissionService(config taskTemporalSubmissionServiceConfig) *taskTemporalSubmissionService {
	return &taskTemporalSubmissionService{
		lifecycle:   config.lifecycle,
		flow:        config.flow,
		persistence: config.persistence,
		refresh:     config.refresh,
	}
}

func (s *taskTemporalSubmissionService) BeginSheinPublishAttempt(ctx context.Context, in SheinPublishAttemptInput) error {
	if s == nil || s.lifecycle == nil {
		return ErrTaskResultUnavailable
	}
	return s.lifecycle.BeginSheinPublishAttempt(ctx, in)
}

func (s *taskTemporalSubmissionService) StartSheinPublishWorkflowAttempt(ctx context.Context, taskID string, task *Task, req *SubmitTaskRequest, opts sheinWorkflowSubmitOptions) (*ListingKitPreview, error) {
	if s == nil || s.lifecycle == nil {
		return nil, ErrTaskResultUnavailable
	}
	return s.lifecycle.startSheinPublishWorkflowAttempt(ctx, taskID, task, req, opts)
}

func (s *taskTemporalSubmissionService) ValidateSheinPublishReadiness(ctx context.Context, in SheinPublishAttemptInput) error {
	if s == nil || s.lifecycle == nil {
		return ErrTaskResultUnavailable
	}
	return s.lifecycle.ValidateSheinPublishReadiness(ctx, in)
}

func (s *taskTemporalSubmissionService) PrepareSheinPublishPayload(ctx context.Context, in SheinPublishAttemptInput) (*SheinPreparedSubmitPayload, error) {
	if s == nil || s.flow == nil {
		return nil, ErrTaskResultUnavailable
	}
	return s.flow.PrepareSheinPublishPayload(ctx, in)
}

func (s *taskTemporalSubmissionService) UploadSheinPublishImages(ctx context.Context, in *SheinPreparedSubmitPayload) (*SheinPreparedSubmitPayload, error) {
	if s == nil || s.flow == nil {
		return nil, ErrTaskResultUnavailable
	}
	return s.flow.UploadSheinPublishImages(ctx, in)
}

func (s *taskTemporalSubmissionService) PreValidateSheinPublish(ctx context.Context, in *SheinPreparedSubmitPayload) error {
	if s == nil || s.flow == nil {
		return ErrTaskResultUnavailable
	}
	return s.flow.PreValidateSheinPublish(ctx, in)
}

func (s *taskTemporalSubmissionService) SubmitSheinPublishRemote(ctx context.Context, in *SheinPreparedSubmitPayload) (*SheinRemoteSubmitResult, error) {
	if s == nil || s.flow == nil {
		return nil, ErrTaskResultUnavailable
	}
	return s.flow.SubmitSheinPublishRemote(ctx, in)
}

func (s *taskTemporalSubmissionService) PersistSheinPublishSuccess(ctx context.Context, in SheinPersistSubmitSuccessInput) error {
	if s == nil || s.persistence == nil {
		return nil
	}
	return s.persistence.PersistSheinPublishSuccess(ctx, in)
}

func (s *taskTemporalSubmissionService) PersistSheinPublishFailure(ctx context.Context, in SheinPersistSubmitFailureInput) error {
	if s == nil || s.persistence == nil {
		return nil
	}
	return s.persistence.PersistSheinPublishFailure(ctx, in)
}

func (s *taskTemporalSubmissionService) RefreshSheinPublishRemoteStatus(ctx context.Context, in SheinRefreshRemoteStatusInput) (*SheinRefreshRemoteStatusResult, error) {
	if s == nil || s.refresh == nil {
		return nil, ErrTaskResultUnavailable
	}
	return s.refresh.RefreshSheinPublishRemoteStatus(ctx, in)
}

func (s *taskTemporalSubmissionService) BuildSheinTaskPreview(ctx context.Context, taskID string) (*ListingKitPreview, error) {
	if s == nil || s.lifecycle == nil {
		return nil, ErrTaskResultUnavailable
	}
	return s.lifecycle.BuildSheinTaskPreview(ctx, taskID)
}

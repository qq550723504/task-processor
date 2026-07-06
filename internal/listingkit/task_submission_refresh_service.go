package listingkit

import (
	"context"

	apperrors "task-processor/internal/core/errors"
	submissiondomain "task-processor/internal/listing/submission"
	sheinpub "task-processor/internal/publishing/shein"
	sheinother "task-processor/internal/shein/api/other"
	sheinproduct "task-processor/internal/shein/api/product"
)

type sheinSubmissionRefreshState struct {
	task          *Task
	remoteRequest *sheinRemoteStatusRequest
}

type taskSubmissionRefreshServiceConfig struct {
	repo                       Repository
	lockSubmit                 func(key string) func()
	buildTaskPreview           func(context.Context, *Task, string) (*ListingKitPreview, error)
	buildSheinSubmitProductAPI func(context.Context, *Task) (sheinproduct.ProductAPI, error)
	buildSheinSubmitOtherAPI   func(context.Context, *Task) (sheinother.OtherAPI, error)
	recovery                   *taskSubmissionRecoveryService
	resolveRemoteStatus        func(*sheinRemoteStatusRequest) (*sheinpub.SubmissionConfirmRemoteUpdate, error)
}

type taskSubmissionRefreshService struct {
	repo                       Repository
	lockSubmit                 func(key string) func()
	buildTaskPreview           func(context.Context, *Task, string) (*ListingKitPreview, error)
	buildSheinSubmitProductAPI func(context.Context, *Task) (sheinproduct.ProductAPI, error)
	buildSheinSubmitOtherAPI   func(context.Context, *Task) (sheinother.OtherAPI, error)
	recovery                   *taskSubmissionRecoveryService
	resolveRemoteStatus        func(*sheinRemoteStatusRequest) (*sheinpub.SubmissionConfirmRemoteUpdate, error)
	refreshRunner              *submissiondomain.StatusRefreshService[sheinSubmissionRefreshState, sheinpub.SubmissionConfirmRemoteUpdate, ListingKitPreview]
}

func newTaskSubmissionRefreshService(config taskSubmissionRefreshServiceConfig) *taskSubmissionRefreshService {
	svc := &taskSubmissionRefreshService{
		repo:                       config.repo,
		lockSubmit:                 config.lockSubmit,
		buildTaskPreview:           config.buildTaskPreview,
		buildSheinSubmitProductAPI: config.buildSheinSubmitProductAPI,
		buildSheinSubmitOtherAPI:   config.buildSheinSubmitOtherAPI,
		recovery:                   config.recovery,
		resolveRemoteStatus:        config.resolveRemoteStatus,
	}
	svc.refreshRunner = submissiondomain.NewStatusRefreshService(
		submissiondomain.StatusRefreshServiceConfig[sheinSubmissionRefreshState, sheinpub.SubmissionConfirmRemoteUpdate, ListingKitPreview]{
			LockKeySuffix:       "refresh_submission_status",
			LockSubmit:          svc.lockSubmit,
			LoadState:           svc.loadSheinSubmissionRefreshState,
			ResolveConfirmation: svc.resolveSubmissionRefreshConfirmation,
			Finish:              svc.finishSubmissionRefresh,
		},
	)
	return svc
}

func (s *taskSubmissionRefreshService) RefreshSubmissionStatus(ctx context.Context, taskID string) (*ListingKitPreview, error) {
	if s == nil || s.refreshRunner == nil {
		return nil, apperrors.New(apperrors.ErrCodeSystem, "submission refresh service is not configured")
	}
	return s.refreshRunner.RefreshStatus(ctx, taskID)
}

func (s *taskSubmissionRefreshService) resolveSubmissionRefreshConfirmation(taskID string, refreshState *sheinSubmissionRefreshState) (*sheinpub.SubmissionConfirmRemoteUpdate, error) {
	request, err := buildSheinRemoteStatusRequest(taskID, refreshState)
	if err != nil {
		return nil, err
	}
	return s.resolveSheinSubmitRemoteStatus(request)
}

func (s *taskSubmissionRefreshService) finishSubmissionRefresh(ctx context.Context, taskID string, refreshState *sheinSubmissionRefreshState, confirmation *sheinpub.SubmissionConfirmRemoteUpdate, remoteErr error) (*ListingKitPreview, error) {
	task, err := s.persistSheinSubmissionRefreshResult(ctx, taskID, refreshState, confirmation, remoteErr)
	if err != nil {
		return nil, err
	}
	return s.completeSubmissionRefresh(ctx, task, remoteErr)
}

func (s *taskSubmissionRefreshService) loadSheinSubmissionRefreshState(ctx context.Context, taskID string) (*sheinSubmissionRefreshState, error) {
	task, pkg, err := s.loadSheinSubmissionRefreshTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	selection, productAPI, err := s.loadSubmissionRefreshInputs(ctx, taskID, task, pkg)
	if err != nil {
		return nil, err
	}
	return s.buildSheinSubmissionRefreshState(ctx, task, pkg, selection, productAPI), nil
}

func (s *taskSubmissionRefreshService) loadSheinSubmissionRefreshTask(ctx context.Context, taskID string) (*Task, *SheinPackage, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, nil, apperrors.Wrapf(err, apperrors.ErrCodeTaskNotFound, "failed to get task %s", taskID)
	}
	if task.Result == nil {
		return nil, nil, apperrors.New(apperrors.ErrCodeTaskProcessing, "task result is not available yet")
	}
	pkg, err := loadSubmissionRefreshTaskPackage(task)
	if err != nil {
		return task, nil, err
	}
	return task, pkg, nil
}

func (s *taskSubmissionRefreshService) completeSubmissionRefresh(ctx context.Context, task *Task, remoteErr error) (*ListingKitPreview, error) {
	if remoteErr != nil {
		return nil, remoteErr
	}
	return s.buildTaskPreview(ctx, task, "shein")
}

func buildSheinRemoteStatusRequest(taskID string, refreshState *sheinSubmissionRefreshState) (*sheinRemoteStatusRequest, error) {
	if refreshState == nil || refreshState.remoteRequest == nil {
		return nil, apperrors.New(apperrors.ErrCodeSystem, "submission refresh state is not available")
	}
	request := *refreshState.remoteRequest
	request.taskID = taskID
	return &request, nil
}

func (s *taskSubmissionRefreshService) resolveSheinSubmitRemoteStatus(request *sheinRemoteStatusRequest) (*sheinpub.SubmissionConfirmRemoteUpdate, error) {
	if request == nil {
		return nil, apperrors.New(apperrors.ErrCodeSystem, "submission refresh confirmation request is not available")
	}
	if s.resolveRemoteStatus == nil {
		return nil, apperrors.New(apperrors.ErrCodeSystem, "submit remote status resolution is not configured")
	}
	copyRequest := *request
	copyRequest.fallbackMessage = sheinpub.ResolveSubmissionRemoteRefreshFallbackMessage(copyRequest.action, copyRequest.defaultConfirmed, copyRequest.fallbackMessage)
	return s.resolveRemoteStatus(&copyRequest)
}

func (s *taskSubmissionRefreshService) mutateSubmissionRefreshTask(ctx context.Context, taskID string, mutate TaskResultMutation) (*Task, error) {
	if s.recovery == nil {
		return nil, apperrors.New(apperrors.ErrCodeSystem, "submission refresh recovery is not configured")
	}
	return s.recovery.mutateTaskResult(ctx, taskID, mutate)
}

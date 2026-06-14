package listingkit

import (
	"context"
	"time"

	apperrors "task-processor/internal/core/errors"
	submissiondomain "task-processor/internal/listing/submission"
	sheinmarketpub "task-processor/internal/marketplace/shein/publishing"
	sheinother "task-processor/internal/shein/api/other"
	sheinproduct "task-processor/internal/shein/api/product"
)

type sheinSubmissionRefreshState struct {
	task             *Task
	action           string
	requestID        string
	startedAt        time.Time
	lookupCodes      []string
	defaultConfirmed bool
	fallbackMessage  string
	productAPI       sheinproduct.ProductAPI
	otherAPI         sheinother.OtherAPI
	spuName          string
}

type sheinSubmissionRefreshConfirmationRequest struct {
	productAPI       sheinproduct.ProductAPI
	otherAPI         sheinother.OtherAPI
	action           string
	requestID        string
	lookupCodes      []string
	spuName          string
	defaultConfirmed bool
	fallbackMessage  string
	startedAt        time.Time
	taskID           string
}

type taskSubmissionRefreshServiceConfig struct {
	repo                       Repository
	lockSubmit                 func(key string) func()
	buildTaskPreview           func(context.Context, *Task, string) (*ListingKitPreview, error)
	buildSheinSubmitProductAPI func(context.Context, *Task) (sheinproduct.ProductAPI, error)
	buildSheinSubmitOtherAPI   func(context.Context, *Task) (sheinother.OtherAPI, error)
	recovery                   *taskSubmissionRecoveryService
	resolveRemoteStatus        func(sheinproduct.ProductAPI, sheinother.OtherAPI, string, string, []string, string, bool, string, time.Time, string) (*sheinRemoteConfirmation, error)
}

type taskSubmissionRefreshService struct {
	repo                       Repository
	lockSubmit                 func(key string) func()
	buildTaskPreview           func(context.Context, *Task, string) (*ListingKitPreview, error)
	buildSheinSubmitProductAPI func(context.Context, *Task) (sheinproduct.ProductAPI, error)
	buildSheinSubmitOtherAPI   func(context.Context, *Task) (sheinother.OtherAPI, error)
	recovery                   *taskSubmissionRecoveryService
	resolveRemoteStatus        func(sheinproduct.ProductAPI, sheinother.OtherAPI, string, string, []string, string, bool, string, time.Time, string) (*sheinRemoteConfirmation, error)
	refreshRunner              *submissiondomain.StatusRefreshService[sheinSubmissionRefreshState, sheinRemoteConfirmation, ListingKitPreview]
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
		submissiondomain.StatusRefreshServiceConfig[sheinSubmissionRefreshState, sheinRemoteConfirmation, ListingKitPreview]{
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

func (s *taskSubmissionRefreshService) resolveSubmissionRefreshConfirmation(taskID string, refreshState *sheinSubmissionRefreshState) (*sheinRemoteConfirmation, error) {
	request, err := buildSubmissionRefreshConfirmationRequest(taskID, refreshState)
	if err != nil {
		return nil, err
	}
	return s.resolveSubmissionRefreshRemoteConfirmation(request)
}

func (s *taskSubmissionRefreshService) finishSubmissionRefresh(ctx context.Context, taskID string, refreshState *sheinSubmissionRefreshState, confirmation *sheinRemoteConfirmation, remoteErr error) (*ListingKitPreview, error) {
	task, err := s.persistSheinSubmissionRefreshResult(ctx, taskID, refreshState, confirmation)
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

func buildSubmissionRefreshConfirmationRequest(taskID string, refreshState *sheinSubmissionRefreshState) (*sheinSubmissionRefreshConfirmationRequest, error) {
	if refreshState == nil {
		return nil, apperrors.New(apperrors.ErrCodeSystem, "submission refresh state is not available")
	}
	return &sheinSubmissionRefreshConfirmationRequest{
		productAPI:       refreshState.productAPI,
		otherAPI:         refreshState.otherAPI,
		action:           refreshState.action,
		requestID:        refreshState.requestID,
		lookupCodes:      refreshState.lookupCodes,
		spuName:          refreshState.spuName,
		defaultConfirmed: refreshState.defaultConfirmed,
		fallbackMessage:  refreshState.fallbackMessage,
		startedAt:        refreshState.startedAt,
		taskID:           taskID,
	}, nil
}

func (s *taskSubmissionRefreshService) resolveSubmissionRefreshRemoteConfirmation(request *sheinSubmissionRefreshConfirmationRequest) (*sheinRemoteConfirmation, error) {
	if request == nil {
		return nil, apperrors.New(apperrors.ErrCodeSystem, "submission refresh confirmation request is not available")
	}
	return s.resolveSheinSubmitRemoteStatus(
		request.productAPI,
		request.otherAPI,
		request.action,
		request.requestID,
		request.lookupCodes,
		request.spuName,
		request.defaultConfirmed,
		request.fallbackMessage,
		request.startedAt,
		request.taskID,
	)
}

func (s *taskSubmissionRefreshService) resolveSheinSubmitRemoteStatus(productAPI sheinproduct.ProductAPI, otherAPI sheinother.OtherAPI, action, requestID string, lookupCodes []string, spuName string, defaultConfirmed bool, fallbackMessage string, startedAt time.Time, taskID string) (*sheinRemoteConfirmation, error) {
	if s.resolveRemoteStatus == nil {
		return nil, apperrors.New(apperrors.ErrCodeSystem, "submit remote status resolution is not configured")
	}
	fallbackMessage = sheinmarketpub.BuildRemoteConfirmationPolicy(action, defaultConfirmed).RefreshFallbackMessage
	return s.resolveRemoteStatus(productAPI, otherAPI, action, requestID, lookupCodes, spuName, defaultConfirmed, fallbackMessage, startedAt, taskID)
}

func (s *taskSubmissionRefreshService) mutateSubmissionRefreshTask(ctx context.Context, taskID string, mutate TaskResultMutation) (*Task, error) {
	if s.recovery == nil {
		return nil, apperrors.New(apperrors.ErrCodeSystem, "submission refresh recovery is not configured")
	}
	return s.recovery.mutateTaskResult(ctx, taskID, mutate)
}

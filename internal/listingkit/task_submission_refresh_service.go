package listingkit

import (
	"context"
	"time"

	apperrors "task-processor/internal/core/errors"
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

func (s *taskSubmissionService) RefreshSubmissionStatus(ctx context.Context, taskID string) (*ListingKitPreview, error) {
	if s.lockSubmit != nil {
		unlockSubmit := s.lockSubmit(taskID + ":refresh_submission_status")
		defer unlockSubmit()
	}
	refreshState, err := s.loadSheinSubmissionRefreshState(ctx, taskID)
	if err != nil {
		return nil, err
	}
	confirmation, remoteErr := s.resolveSubmissionRefreshConfirmation(taskID, refreshState)
	if remoteErr != nil && confirmation == nil {
		return s.finishSubmissionRefresh(ctx, taskID, refreshState, nil, remoteErr)
	}
	return s.finishSubmissionRefresh(ctx, taskID, refreshState, confirmation, remoteErr)
}

func (s *taskSubmissionService) resolveSubmissionRefreshConfirmation(taskID string, refreshState *sheinSubmissionRefreshState) (*sheinRemoteConfirmation, error) {
	request, err := buildSubmissionRefreshConfirmationRequest(taskID, refreshState)
	if err != nil {
		return nil, err
	}
	return s.resolveSubmissionRefreshRemoteConfirmation(request)
}

func (s *taskSubmissionService) finishSubmissionRefresh(ctx context.Context, taskID string, refreshState *sheinSubmissionRefreshState, confirmation *sheinRemoteConfirmation, remoteErr error) (*ListingKitPreview, error) {
	task, err := s.persistSheinSubmissionRefreshResult(ctx, taskID, refreshState, confirmation)
	if err != nil {
		return nil, err
	}
	return s.completeSubmissionRefresh(ctx, task, remoteErr)
}

func (s *taskSubmissionService) loadSheinSubmissionRefreshState(ctx context.Context, taskID string) (*sheinSubmissionRefreshState, error) {
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

func (s *taskSubmissionService) loadSheinSubmissionRefreshTask(ctx context.Context, taskID string) (*Task, *SheinPackage, error) {
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

func (s *taskSubmissionService) completeSubmissionRefresh(ctx context.Context, task *Task, remoteErr error) (*ListingKitPreview, error) {
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

func (s *taskSubmissionService) resolveSubmissionRefreshRemoteConfirmation(request *sheinSubmissionRefreshConfirmationRequest) (*sheinRemoteConfirmation, error) {
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

func (s *taskSubmissionService) resolveSheinSubmitRemoteStatus(productAPI sheinproduct.ProductAPI, otherAPI sheinother.OtherAPI, action, requestID string, lookupCodes []string, spuName string, defaultConfirmed bool, fallbackMessage string, startedAt time.Time, taskID string) (*sheinRemoteConfirmation, error) {
	if s.resolveRemoteStatus == nil {
		return nil, apperrors.New(apperrors.ErrCodeSystem, "submit remote status resolution is not configured")
	}
	if defaultConfirmed {
		fallbackMessage = "SHEIN accepted publish request; remote record not yet visible"
	}
	return s.resolveRemoteStatus(productAPI, otherAPI, action, requestID, lookupCodes, spuName, defaultConfirmed, fallbackMessage, startedAt, taskID)
}

package api

import (
	"context"
	"errors"
	"testing"

	"task-processor/internal/listingkit"
)

type stubGenerationTaskService struct {
	page               *listingkit.GenerationTaskPage
	queue              *listingkit.GenerationQueuePage
	action             *listingkit.GenerationActionExecutionResult
	navigation         *listingkit.GenerationReviewNavigationDispatchResponse
	reviewSession      *listingkit.GenerationReviewSessionResponse
	reviewPreview      *listingkit.GenerationReviewPreviewResponse
	err                error
	lastTask           string
	query              *listingkit.GenerationTaskQuery
	queueQuery         *listingkit.GenerationQueueQuery
	retryReq           *listingkit.RetryGenerationTasksRequest
	actionReq          *listingkit.ExecuteGenerationActionRequest
	navigationReq      *listingkit.GenerationReviewNavigationDispatchRequest
	reviewSessionQuery *listingkit.GenerationQueueQuery
	reviewPreviewQuery *listingkit.GenerationQueueQuery
}

type stubCreateGenerateTaskHandlerService struct {
	stubTaskLifecycleHandlerService
	createdTask *listingkit.Task
	createReq   *listingkit.GenerateRequest
	err         error
}

func (s *stubCreateGenerateTaskHandlerService) CreateGenerateTask(ctx context.Context, req *listingkit.GenerateRequest) (*listingkit.Task, error) {
	s.createReq = req
	if s.createdTask != nil || s.err != nil {
		return s.createdTask, s.err
	}
	return nil, errors.New("not implemented")
}

func (s *stubGenerationTaskService) GetTaskGenerationTasks(ctx context.Context, taskID string, query *listingkit.GenerationTaskQuery) (*listingkit.GenerationTaskPage, error) {
	s.lastTask = taskID
	s.query = query
	return s.page, s.err
}

func (s *stubGenerationTaskService) GetTaskGenerationQueue(ctx context.Context, taskID string, query *listingkit.GenerationQueueQuery) (*listingkit.GenerationQueuePage, error) {
	s.lastTask = taskID
	s.queueQuery = query
	return s.queue, s.err
}

func (s *stubGenerationTaskService) GetTaskGenerationReviewSession(ctx context.Context, taskID string, query *listingkit.GenerationQueueQuery) (*listingkit.GenerationReviewSessionResponse, error) {
	s.lastTask = taskID
	s.reviewSessionQuery = query
	return s.reviewSession, s.err
}

func (s *stubGenerationTaskService) RetryTaskGenerationTasks(ctx context.Context, taskID string, req *listingkit.RetryGenerationTasksRequest) (*listingkit.GenerationTaskPage, error) {
	s.lastTask = taskID
	s.retryReq = req
	return s.page, s.err
}

func (s *stubGenerationTaskService) ExecuteTaskGenerationAction(ctx context.Context, taskID string, req *listingkit.ExecuteGenerationActionRequest) (*listingkit.GenerationActionExecutionResult, error) {
	s.lastTask = taskID
	s.actionReq = req
	return s.action, s.err
}

func (s *stubGenerationTaskService) DispatchTaskGenerationNavigation(ctx context.Context, taskID string, req *listingkit.GenerationReviewNavigationDispatchRequest) (*listingkit.GenerationReviewNavigationDispatchResponse, error) {
	s.lastTask = taskID
	s.navigationReq = req
	return s.navigation, s.err
}

func (s *stubGenerationTaskService) GetTaskGenerationReviewPreview(ctx context.Context, taskID string, query *listingkit.GenerationQueueQuery) (*listingkit.GenerationReviewPreviewResponse, error) {
	s.lastTask = taskID
	s.reviewPreviewQuery = query
	return s.reviewPreview, s.err
}

func newGenerationTaskHandler(t *testing.T, svc listingkit.GenerationTaskService, opts ...HandlerOption) *handler {
	t.Helper()
	options := append([]HandlerOption{WithGenerationTaskService(svc)}, opts...)
	h, err := NewHandler(&stubHandlerCoreService{}, options...)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	return h
}

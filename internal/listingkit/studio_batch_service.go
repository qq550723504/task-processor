package listingkit

import (
	"context"
	"errors"
)

var ErrStudioBatchActionValidation = errors.New("studio batch action validation failed")

type studioBatchActionValidationError struct {
	message string
}

func (e *studioBatchActionValidationError) Error() string {
	if e == nil {
		return ""
	}
	return e.message
}

func (e *studioBatchActionValidationError) Unwrap() error {
	return ErrStudioBatchActionValidation
}

func NewStudioBatchActionValidationError(message string) error {
	return &studioBatchActionValidationError{message: message}
}

type StudioBatchService interface {
	GetStudioBatchDetail(ctx context.Context, batchID string) (*StudioBatchDetail, error)
	PrepareStudioBatchGeneration(ctx context.Context, batchID string) (*StudioBatchDetail, error)
	ResumeStudioBatchGeneration(ctx context.Context, batchID string) (*StudioBatchDetail, error)
	StartStudioBatchGeneration(ctx context.Context, batchID string) (*StudioBatchDetail, error)
	PrepareRetryStudioBatchItems(ctx context.Context, batchID string, req *RetryStudioBatchItemsRequest) (*StudioBatchDetail, error)
	RetryStudioBatchItems(ctx context.Context, batchID string, req *RetryStudioBatchItemsRequest) (*StudioBatchDetail, error)
	ApproveStudioBatchDesigns(ctx context.Context, batchID string, req *ApproveStudioBatchDesignsRequest) (*StudioBatchDetail, error)
	PrepareCreateStudioBatchTasks(ctx context.Context, batchID string, req *CreateStudioBatchTasksRequest) (*CreateStudioBatchTasksResult, error)
	CreateStudioBatchTasks(ctx context.Context, batchID string, req *CreateStudioBatchTasksRequest) (*CreateStudioBatchTasksResult, error)
}

type StudioBatchDetail struct {
	Batch        *StudioBatchRecord       `json:"batch,omitempty"`
	Items        []StudioBatchItemDetail  `json:"items,omitempty"`
	CreatedTasks []SheinStudioCreatedTask `json:"created_tasks,omitempty"`
	FailedTasks  []SheinStudioFailedTask  `json:"failed_tasks,omitempty"`
}

type StudioBatchItemDetail struct {
	Item     StudioBatchItemRecord            `json:"item"`
	Attempts []StudioGenerationAttemptRecord  `json:"attempts,omitempty"`
	Designs  []StudioMaterializedDesignRecord `json:"designs,omitempty"`
}

type ApproveStudioBatchDesignsRequest struct {
	DesignIDs []string `json:"design_ids,omitempty"`
}

type CreateStudioBatchTasksResult struct {
	Batch        *StudioBatchRecord       `json:"batch,omitempty"`
	Items        []StudioBatchItemDetail  `json:"items,omitempty"`
	CreatedTasks []SheinStudioCreatedTask `json:"created_tasks,omitempty"`
	FailedTasks  []SheinStudioFailedTask  `json:"failed_tasks,omitempty"`
}

type taskStudioBatchServiceConfig struct {
	repo               StudioBatchRepository
	studioSessionRepo  studioBatchSeedSessionRepository
	generator          studioBatchGenerator
	createGenerateTask func(ctx context.Context, req *GenerateRequest) (*Task, error)
	getTask            func(ctx context.Context, taskID string) (*Task, error)
}

func (s *service) GetStudioBatchDetail(ctx context.Context, batchID string) (*StudioBatchDetail, error) {
	return s.taskStudioBatchOrDefault().GetStudioBatchDetail(ctx, batchID)
}

func (s *service) StartStudioBatchGeneration(ctx context.Context, batchID string) (*StudioBatchDetail, error) {
	return s.taskStudioBatchOrDefault().StartStudioBatchGeneration(ctx, batchID)
}

func (s *service) PrepareStudioBatchGeneration(ctx context.Context, batchID string) (*StudioBatchDetail, error) {
	return s.taskStudioBatchOrDefault().PrepareStudioBatchGeneration(ctx, batchID)
}

func (s *service) ResumeStudioBatchGeneration(ctx context.Context, batchID string) (*StudioBatchDetail, error) {
	return s.taskStudioBatchOrDefault().ResumeStudioBatchGeneration(ctx, batchID)
}

func (s *service) PrepareRetryStudioBatchItems(ctx context.Context, batchID string, req *RetryStudioBatchItemsRequest) (*StudioBatchDetail, error) {
	return s.taskStudioBatchOrDefault().PrepareRetryStudioBatchItems(ctx, batchID, req)
}

func (s *service) RetryStudioBatchItems(ctx context.Context, batchID string, req *RetryStudioBatchItemsRequest) (*StudioBatchDetail, error) {
	return s.taskStudioBatchOrDefault().RetryStudioBatchItems(ctx, batchID, req)
}

func (s *service) ApproveStudioBatchDesigns(ctx context.Context, batchID string, req *ApproveStudioBatchDesignsRequest) (*StudioBatchDetail, error) {
	return s.taskStudioBatchOrDefault().ApproveStudioBatchDesigns(ctx, batchID, req)
}

func (s *service) CreateStudioBatchTasks(ctx context.Context, batchID string, req *CreateStudioBatchTasksRequest) (*CreateStudioBatchTasksResult, error) {
	return s.taskStudioBatchOrDefault().CreateStudioBatchTasks(ctx, batchID, req)
}

func (s *service) PrepareCreateStudioBatchTasks(ctx context.Context, batchID string, req *CreateStudioBatchTasksRequest) (*CreateStudioBatchTasksResult, error) {
	return s.taskStudioBatchOrDefault().PrepareCreateStudioBatchTasks(ctx, batchID, req)
}

func (s *service) taskStudioBatchOrDefault() *taskStudioBatchService {
	if s.taskStudioBatch != nil {
		return s.taskStudioBatch
	}
	s.taskStudioBatch = newTaskStudioBatchService(buildTaskStudioBatchServiceConfig(s))
	return s.taskStudioBatch
}

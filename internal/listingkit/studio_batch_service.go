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
	batchRunner        *listingStudioBatchGenerationRunner
	detailRunner       *listingStudioBatchDetailRunner
	reviewRunner       *listingStudioBatchReviewRunner
	retryRunner        *listingStudioBatchRetryPrepareRunner
	taskCreationRunner *listingStudioBatchTaskCreationRunner
	taskExecuteRunner  *listingStudioBatchTaskExecuteRunner
	taskPrepareRunner  *listingStudioBatchTaskPrepareRunner
	taskResumeRunner   *listingStudioBatchTaskResumeRunner
}

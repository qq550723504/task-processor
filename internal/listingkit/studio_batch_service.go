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
	RetryStudioBatchDesignBackgroundRemoval(ctx context.Context, batchID string, req *RetryStudioBatchDesignBackgroundRemovalRequest) (*StudioBatchDetail, error)
	ApplyManualStudioBatchDesignBackgroundRemoval(ctx context.Context, batchID string, designID string, imageURL string) (*StudioBatchDetail, error)
	ApproveStudioBatchDesigns(ctx context.Context, batchID string, req *ApproveStudioBatchDesignsRequest) (*StudioBatchDetail, error)
}

type RetryStudioBatchDesignBackgroundRemovalRequest struct {
	DesignIDs []string `json:"design_ids,omitempty"`
}

type StudioBatchDetail struct {
	Batch        *StudioBatchRecord      `json:"batch,omitempty"`
	Items        []StudioBatchItemDetail `json:"items,omitempty"`
	StatusGroups StudioBatchStatusGroups `json:"status_groups,omitempty"`
}

type StudioBatchItemDetail struct {
	Item     StudioBatchItemRecord            `json:"item"`
	Attempts []StudioGenerationAttemptRecord  `json:"attempts,omitempty"`
	Designs  []StudioMaterializedDesignRecord `json:"designs,omitempty"`
}

type ApproveStudioBatchDesignsRequest struct {
	DesignIDs []string `json:"design_ids,omitempty"`
}

type taskStudioBatchServiceConfig struct {
	repo                     StudioBatchRepository
	batchRunRepo             StudioBatchRunRepository
	studioSessionRepo        studioBatchSeedSessionRepository
	sdsProductDetailProvider SDSBaselineRemoteProvider
	generator                studioBatchGenerator
	retryBackgroundRemoval   func(context.Context, string, string) (*studioBackgroundRemovalMaterialization, error)
	serviceRunner            *listingStudioBatchServiceRunner
	batchRunner              *listingStudioBatchGenerationRunner
	detailRunner             *listingStudioBatchDetailRunner
	reviewRunner             *listingStudioBatchReviewRunner
	retryRunner              *listingStudioBatchRetryPrepareRunner
}

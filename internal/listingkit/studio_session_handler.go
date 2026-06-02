package listingkit

import (
	"context"

	"github.com/gin-gonic/gin"
)

type StudioSessionHandlerService interface {
	ListStudioSessionGallery(ctx context.Context, limit int) (*StudioSessionGalleryResponse, error)
	ListStudioBatches(ctx context.Context, limit int) (*StudioBatchListResponse, error)
	GetStudioBatch(ctx context.Context, batchID string) (*StudioBatchDraftDetail, error)
	GetStudioBatchDetail(ctx context.Context, batchID string) (*StudioBatchDetail, error)
	PrepareStudioBatchGeneration(ctx context.Context, batchID string) (*StudioBatchDetail, error)
	ResumeStudioBatchGeneration(ctx context.Context, batchID string) (*StudioBatchDetail, error)
	StartStudioBatchGeneration(ctx context.Context, batchID string) (*StudioBatchDetail, error)
	PrepareRetryStudioBatchItems(ctx context.Context, batchID string, req *RetryStudioBatchItemsRequest) (*StudioBatchDetail, error)
	RetryStudioBatchItems(ctx context.Context, batchID string, req *RetryStudioBatchItemsRequest) (*StudioBatchDetail, error)
	ApproveStudioBatchDesigns(ctx context.Context, batchID string, req *ApproveStudioBatchDesignsRequest) (*StudioBatchDetail, error)
	CreateStudioBatchTasks(ctx context.Context, batchID string, req *CreateStudioBatchTasksRequest) (*CreateStudioBatchTasksResult, error)
	UpsertStudioBatch(ctx context.Context, req *UpsertStudioBatchRequest) (*StudioBatchDraftDetail, error)
	DeleteStudioBatch(ctx context.Context, batchID string) error
}

type StudioSessionHandler interface {
	ListStudioSessionGallery(c *gin.Context)
	ListStudioBatches(c *gin.Context)
	GetStudioBatch(c *gin.Context)
	StartStudioBatchGeneration(c *gin.Context)
	RetryStudioBatchItems(c *gin.Context)
	ApproveStudioBatchDesigns(c *gin.Context)
	CreateStudioBatchTasks(c *gin.Context)
	UpsertStudioBatch(c *gin.Context)
	DeleteStudioBatch(c *gin.Context)
}

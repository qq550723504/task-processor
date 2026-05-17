package listingkit

import (
	"context"

	"github.com/gin-gonic/gin"
)

type StudioSessionHandlerService interface {
	EnsureStudioSession(ctx context.Context, req *EnsureStudioSessionRequest) (*SheinStudioSessionDetail, error)
	GetStudioSession(ctx context.Context, sessionID string) (*SheinStudioSessionDetail, error)
	UpdateStudioSession(ctx context.Context, sessionID string, req *UpdateStudioSessionRequest) (*SheinStudioSessionDetail, error)
	ReplaceStudioSessionDesigns(ctx context.Context, sessionID string, req *ReplaceStudioSessionDesignsRequest) (*SheinStudioSessionDetail, error)
	ListStudioSessionGallery(ctx context.Context, limit int) (*StudioSessionGalleryResponse, error)
	ListStudioBatches(ctx context.Context, limit int) (*StudioBatchListResponse, error)
	GetStudioBatch(ctx context.Context, batchID string) (*SheinStudioSessionDetail, error)
	UpsertStudioBatch(ctx context.Context, req *UpsertStudioBatchRequest) (*SheinStudioSessionDetail, error)
	DeleteStudioBatch(ctx context.Context, batchID string) error
}

type StudioSessionHandler interface {
	EnsureStudioSession(c *gin.Context)
	GetStudioSession(c *gin.Context)
	UpdateStudioSession(c *gin.Context)
	ReplaceStudioSessionDesigns(c *gin.Context)
	ListStudioSessionGallery(c *gin.Context)
	ListStudioBatches(c *gin.Context)
	GetStudioBatch(c *gin.Context)
	UpsertStudioBatch(c *gin.Context)
	DeleteStudioBatch(c *gin.Context)
}

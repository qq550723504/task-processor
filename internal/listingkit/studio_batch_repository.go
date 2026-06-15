package listingkit

import (
	"context"
	"errors"
	"time"
)

var ErrStudioBatchUnknownItemReference = errors.New("studio batch graph references unknown item")
var ErrStudioBatchOwnershipConflict = errors.New("studio batch update conflicts with immutable ownership")

type StudioBatchRepository interface {
	CreateStudioBatchGraph(ctx context.Context, batch *StudioBatchRecord, items []StudioBatchItemRecord, attempts []StudioGenerationAttemptRecord, designs []StudioMaterializedDesignRecord) error
	ReplaceStudioBatchGenerationGraph(ctx context.Context, batch *StudioBatchRecord, items []StudioBatchItemRecord) error
	CreateStudioBatchItems(ctx context.Context, batchID string, items []StudioBatchItemRecord) error
	CreateStudioGenerationAttempt(ctx context.Context, attempt *StudioGenerationAttemptRecord) error
	ClaimStudioBatchItem(ctx context.Context, itemID string, fromStatus StudioBatchItemStatus, toStatus StudioBatchItemStatus, updatedAt time.Time) (*StudioBatchItemRecord, bool, error)
	GetStudioBatch(ctx context.Context, batchID string) (*StudioBatchRecord, error)
	GetStudioBatchItem(ctx context.Context, itemID string) (*StudioBatchItemRecord, error)
	GetStudioBatchDetail(ctx context.Context, batchID string) (*StudioBatchDetailGraph, error)
	ListStudioMaterializedDesignsByIDs(ctx context.Context, batchID string, designIDs []string) ([]StudioMaterializedDesignRecord, error)
	ReplaceStudioItemMaterializedDesigns(ctx context.Context, itemID string, designs []StudioMaterializedDesignRecord) error
	ReplaceStudioMaterializedDesignReviews(ctx context.Context, batchID string, designIDs []string, updatedAt time.Time) error
	UpdateStudioBatch(ctx context.Context, batch *StudioBatchRecord) error
	UpdateStudioBatchItem(ctx context.Context, item *StudioBatchItemRecord) error
	UpdateStudioGenerationAttempt(ctx context.Context, attempt *StudioGenerationAttemptRecord) error
	UpdateStudioMaterializedDesign(ctx context.Context, design *StudioMaterializedDesignRecord) error
}

package studio

import (
	"context"
	"errors"
	"time"
)

var ErrBatchUnknownItemReference = errors.New("studio batch graph references unknown item")
var ErrBatchOwnershipConflict = errors.New("studio batch update conflicts with immutable ownership")

type BatchRepository[Batch any, Item any, Attempt any, Design any, DetailGraph any, ItemStatus any] interface {
	CreateStudioBatchGraph(ctx context.Context, batch *Batch, items []Item, attempts []Attempt, designs []Design) error
	ReplaceStudioBatchGenerationGraph(ctx context.Context, batch *Batch, items []Item) error
	CreateStudioBatchItems(ctx context.Context, batchID string, items []Item) error
	CreateStudioGenerationAttempt(ctx context.Context, attempt *Attempt) error
	ClaimStudioBatchItem(ctx context.Context, itemID string, fromStatus ItemStatus, toStatus ItemStatus, updatedAt time.Time) (*Item, bool, error)
	GetStudioBatch(ctx context.Context, batchID string) (*Batch, error)
	GetStudioBatchItem(ctx context.Context, itemID string) (*Item, error)
	GetStudioBatchDetail(ctx context.Context, batchID string) (*DetailGraph, error)
	ListStudioMaterializedDesignsByIDs(ctx context.Context, batchID string, designIDs []string) ([]Design, error)
	ReplaceStudioItemMaterializedDesigns(ctx context.Context, itemID string, designs []Design) error
	ReplaceStudioMaterializedDesignReviews(ctx context.Context, batchID string, designIDs []string, updatedAt time.Time) error
	UpdateStudioBatch(ctx context.Context, batch *Batch) error
	UpdateStudioBatchItem(ctx context.Context, item *Item) error
	UpdateStudioGenerationAttempt(ctx context.Context, attempt *Attempt) error
	UpdateStudioMaterializedDesign(ctx context.Context, design *Design) error
}

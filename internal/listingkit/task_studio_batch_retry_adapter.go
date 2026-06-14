package listingkit

import (
	"context"

	studiodomain "task-processor/internal/listing/studio"
)

type listingStudioBatchRetryPrepareRunner = studiodomain.BatchRetryPrepareService[
	StudioBatchDetailGraph,
	StudioBatchItemRecord,
	StudioBatchDetail,
]

func newListingStudioBatchRetryPrepareService(
	repo StudioBatchRepository,
	loadDetail func(context.Context, string) (*StudioBatchDetail, error),
	resetItems func(context.Context, []StudioBatchItemRecord) error,
) *listingStudioBatchRetryPrepareRunner {
	return studiodomain.NewBatchRetryPrepareService(studiodomain.BatchRetryPrepareServiceConfig[
		StudioBatchDetailGraph,
		StudioBatchItemRecord,
		StudioBatchDetail,
	]{
		LoadDetail: func(ctx context.Context, batchID string) (*StudioBatchDetailGraph, error) {
			if repo == nil {
				return nil, nil
			}
			return repo.GetStudioBatchDetail(ctx, batchID)
		},
		SelectItems: selectStudioBatchRetryItems,
		ResetItems:  resetItems,
		LoadResult:  loadDetail,
	})
}

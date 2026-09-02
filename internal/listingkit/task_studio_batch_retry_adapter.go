package listingkit

import (
	"context"

	studiodomain "task-processor/internal/listing/studio"
)

type listingStudioBatchRetryPrepareRunner = studiodomain.BatchRetryPrepareService[
	studioBatchRetryDetailGraph,
	StudioBatchItemRecord,
	StudioBatchDetail,
]

type studioBatchRetryDetailGraph struct {
	*StudioBatchDetailGraph
}

func newListingStudioBatchRetryPrepareService(
	repo StudioBatchRepository,
	loadDetail func(context.Context, string) (*StudioBatchDetail, error),
	resetItems func(context.Context, []StudioBatchItemRecord) error,
) *listingStudioBatchRetryPrepareRunner {
	return studiodomain.NewBatchRetryPrepareService(studiodomain.BatchRetryPrepareServiceConfig[
		studioBatchRetryDetailGraph,
		StudioBatchItemRecord,
		StudioBatchDetail,
	]{
		LoadDetail: func(ctx context.Context, batchID string) (*studioBatchRetryDetailGraph, error) {
			if repo == nil {
				return nil, nil
			}
			detail, err := repo.GetStudioBatchDetail(ctx, batchID)
			if err != nil {
				return nil, err
			}
			graph := &studioBatchRetryDetailGraph{StudioBatchDetailGraph: detail}
			return graph, nil
		},
		SelectItems: selectStudioBatchRetryItems,
		ResetItems:  resetItems,
		LoadResult:  loadDetail,
	})
}

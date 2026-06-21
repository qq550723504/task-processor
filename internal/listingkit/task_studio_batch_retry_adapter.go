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
	TaskLinks []StudioBatchTaskLinkRecord
}

func newListingStudioBatchRetryPrepareService(
	repo StudioBatchRepository,
	taskLinkRepo StudioBatchTaskLinkRepository,
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
			if taskLinkRepo == nil {
				return graph, nil
			}
			links, err := taskLinkRepo.ListStudioBatchTaskLinksByBatchID(ctx, batchID)
			if err != nil {
				return nil, err
			}
			graph.TaskLinks = links
			return graph, nil
		},
		SelectItems: selectStudioBatchRetryItems,
		ResetItems:  resetItems,
		LoadResult:  loadDetail,
	})
}

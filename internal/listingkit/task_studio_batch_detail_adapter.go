package listingkit

import (
	"context"
	"errors"

	"gorm.io/gorm"
	studiodomain "task-processor/internal/listing/studio"
)

type listingStudioBatchDetailRunner = studiodomain.BatchDetailService[
	StudioBatchDetailGraph,
	StudioBatchDetail,
]

func newListingStudioBatchDetailService(
	repo StudioBatchRepository,
	studioSessionRepo studioBatchSeedSessionRepository,
	taskLinkRepo StudioBatchTaskLinkRepository,
	getTask func(context.Context, string) (*Task, error),
	ensureGraph func(context.Context, string) error,
) *listingStudioBatchDetailRunner {
	return studiodomain.NewBatchDetailService(studiodomain.BatchDetailServiceConfig[
		StudioBatchDetailGraph,
		StudioBatchDetail,
	]{
		LoadGraph: func(ctx context.Context, batchID string) (*StudioBatchDetailGraph, error) {
			if repo == nil {
				return nil, errors.New("studio batch repository is not configured")
			}
			return repo.GetStudioBatchDetail(ctx, batchID)
		},
		IsGraphMissing: func(err error) bool {
			return errors.Is(err, gorm.ErrRecordNotFound)
		},
		ResolveWithoutGraph: func(ctx context.Context, batchID string) (*StudioBatchDetail, bool, error) {
			return resolveStudioBatchDetailWithoutGraph(ctx, studioSessionRepo, batchID)
		},
		EnsureGraph: ensureGraph,
		ProjectDetail: func(ctx context.Context, batchID string, detail *StudioBatchDetailGraph) (*StudioBatchDetail, error) {
			draftUpdatedAt, createdTasks, rejectedTasks, failedTasks, err := loadStudioBatchDraftState(ctx, studioSessionRepo, taskLinkRepo, getTask, batchID)
			if err != nil {
				return nil, err
			}
			return projectStudioBatchDetail(detail, draftUpdatedAt, createdTasks, rejectedTasks, failedTasks), nil
		},
	})
}

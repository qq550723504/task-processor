package listingkit

import (
	"context"
	"time"

	studiodomain "task-processor/internal/listing/studio"
)

type listingStudioBatchReviewRunner = studiodomain.BatchDesignReviewService[StudioBatchDetail]

func newListingStudioBatchReviewService(
	repo StudioBatchRepository,
	loadDetail func(context.Context, string) (*StudioBatchDetail, error),
	currentTime func() time.Time,
) *listingStudioBatchReviewRunner {
	return studiodomain.NewBatchDesignReviewService(studiodomain.BatchDesignReviewServiceConfig[StudioBatchDetail]{
		EnsureBatchExists: func(ctx context.Context, batchID string) error {
			if repo == nil {
				return nil
			}
			_, err := repo.GetStudioBatchDetail(ctx, batchID)
			return err
		},
		ReplaceReviews: func(ctx context.Context, batchID string, designIDs []string, updatedAt time.Time) error {
			if repo == nil {
				return nil
			}
			return repo.ReplaceStudioMaterializedDesignReviews(ctx, batchID, designIDs, updatedAt)
		},
		LoadDetail:  loadDetail,
		CurrentTime: currentTime,
	})
}

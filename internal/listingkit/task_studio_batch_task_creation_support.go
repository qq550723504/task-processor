package listingkit

import (
	"context"
)

func shouldResumeStudioBatchTaskCreation(ctx context.Context, repo StudioBatchRepository, batchID string) bool {
	if repo == nil {
		return false
	}
	batch, err := repo.GetStudioBatch(ctx, batchID)
	if err != nil || batch == nil {
		return false
	}
	return batch.Status == StudioBatchStatusTasksCreating
}

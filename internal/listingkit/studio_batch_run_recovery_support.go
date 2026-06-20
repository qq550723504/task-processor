package listingkit

import (
	"context"
	"strings"
	"time"
)

func resetStudioBatchRunForRecovery(ctx context.Context, repo StudioBatchRunRepository, runID string, now time.Time) error {
	if repo == nil || strings.TrimSpace(runID) == "" {
		return nil
	}
	run, err := repo.GetStudioBatchRun(ctx, runID)
	if err != nil {
		return err
	}
	items, err := repo.ListStudioBatchRunItems(ctx, runID)
	if err != nil {
		return err
	}
	for index := range items {
		item := &items[index]
		switch item.Status {
		case StudioBatchRunItemStatusFailed, StudioBatchRunItemStatusCancelled:
			item.Status = StudioBatchRunItemStatusPending
			item.AsyncJobID = ""
			item.ErrorMessage = ""
			item.StartedAt = nil
			item.FinishedAt = nil
			item.UpdatedAt = now
			if err := repo.UpdateStudioBatchRunItem(ctx, item); err != nil {
				return err
			}
		}
	}
	run.Status = StudioBatchRunStatusPending
	run.CancelRequested = false
	run.CurrentBatchID = ""
	run.CurrentIndex = 0
	run.CompletedBatches = countCompletedStudioBatchRunItems(items)
	run.SucceededBatches = countSucceededStudioBatchRunItems(items)
	run.FailedBatches = countFailedStudioBatchRunItems(items)
	run.LastError = firstStudioBatchRunItemError(items)
	run.StartedAt = nil
	run.FinishedAt = nil
	run.UpdatedAt = now
	return repo.UpdateStudioBatchRun(ctx, run)
}

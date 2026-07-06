package listingkit

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func normalizeStudioBatchDesignIDs(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	result := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		normalized := strings.TrimSpace(id)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func normalizeStudioBatchItemIDs(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	result := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		normalized := strings.TrimSpace(id)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func isStudioBatchItemRetryable(status StudioBatchItemStatus) bool {
	switch status {
	case StudioBatchItemStatusReviewReady, StudioBatchItemStatusFailed:
		return true
	default:
		return false
	}
}

func selectStudioBatchRetryItems(
	detail *studioBatchRetryDetailGraph,
	itemIDs []string,
) ([]StudioBatchItemRecord, error) {
	if len(itemIDs) == 0 {
		return nil, NewStudioBatchActionValidationError("item_ids is required")
	}
	if detail == nil || detail.StudioBatchDetailGraph == nil {
		return nil, NewStudioBatchActionValidationError("studio batch detail is required")
	}

	itemsByID := make(map[string]StudioBatchItemRecord, len(detail.StudioBatchDetailGraph.Items))
	for _, item := range detail.StudioBatchDetailGraph.Items {
		itemsByID[item.ID] = item
	}

	itemsToRetry := make([]StudioBatchItemRecord, 0, len(itemIDs))
	for _, itemID := range itemIDs {
		item, ok := itemsByID[itemID]
		if !ok {
			return nil, NewStudioBatchActionValidationError(fmt.Sprintf("unknown item_id: %s", itemID))
		}
		if !isStudioBatchItemRetryable(item.Status) {
			return nil, NewStudioBatchActionValidationError(fmt.Sprintf("item %s is not retryable from status %s", itemID, item.Status))
		}
		itemsToRetry = append(itemsToRetry, item)
	}
	if err := rejectStudioBatchRetryItemsWithCreatedTaskLinks(itemsToRetry, detail.TaskLinks); err != nil {
		return nil, err
	}
	return itemsToRetry, nil
}

func rejectStudioBatchRetryItemsWithCreatedTaskLinks(
	items []StudioBatchItemRecord,
	links []StudioBatchTaskLinkRecord,
) error {
	if len(items) == 0 || len(links) == 0 {
		return nil
	}
	itemIDs := make(map[string]struct{}, len(items))
	for _, item := range items {
		if itemID := strings.TrimSpace(item.ID); itemID != "" {
			itemIDs[itemID] = struct{}{}
		}
	}
	for _, link := range links {
		itemID := strings.TrimSpace(link.ItemID)
		if _, ok := itemIDs[itemID]; !ok {
			continue
		}
		if !isStudioBatchCreatedOrReusedTaskLink(link) {
			continue
		}
		return NewStudioBatchActionValidationError(fmt.Sprintf("item %s has ListingKit task links: tasks_already_created", itemID))
	}
	return nil
}

func isStudioBatchCreatedOrReusedTaskLink(link StudioBatchTaskLinkRecord) bool {
	if strings.TrimSpace(link.ListingKitTaskID) == "" {
		return false
	}
	if strings.TrimSpace(link.Status) == studioBatchTaskLinkStatusCreated {
		return true
	}
	return strings.TrimSpace(link.ReasonCode) == studioBatchReusedTaskReasonCode
}

func (s *taskStudioBatchService) resetStudioBatchRetryItems(
	ctx context.Context,
	items []StudioBatchItemRecord,
) error {
	now := s.currentTime().UTC()
	batchIDs := make([]string, 0, len(items))
	seenBatchIDs := make(map[string]struct{}, len(items))
	for _, item := range items {
		item.Status = StudioBatchItemStatusPending
		item.LastError = ""
		item.UpdatedAt = now
		if err := s.repo.UpdateStudioBatchItem(ctx, &item); err != nil {
			return err
		}
		if batchID := strings.TrimSpace(item.BatchID); batchID != "" {
			if _, ok := seenBatchIDs[batchID]; !ok {
				seenBatchIDs[batchID] = struct{}{}
				batchIDs = append(batchIDs, batchID)
			}
		}
	}
	if err := s.resetStudioBatchRetryRunItems(ctx, batchIDs, now); err != nil {
		return err
	}
	return nil
}

func (s *taskStudioBatchService) resetStudioBatchRetryRunItems(ctx context.Context, batchIDs []string, now time.Time) error {
	if s == nil || s.batchRunRepo == nil || len(batchIDs) == 0 {
		return nil
	}
	for _, batchID := range batchIDs {
		runItems, err := s.batchRunRepo.ListStudioBatchRunItemsByBatchID(ctx, batchID)
		if err != nil {
			return err
		}
		if len(runItems) == 0 {
			continue
		}
		runItem, ok := selectStudioBatchRetryRunItem(runItems)
		if !ok {
			continue
		}
		runItem.Status = StudioBatchRunItemStatusPending
		runItem.AsyncJobID = ""
		runItem.ErrorMessage = ""
		runItem.StartedAt = nil
		runItem.FinishedAt = nil
		runItem.UpdatedAt = now
		if err := s.batchRunRepo.UpdateStudioBatchRunItem(ctx, &runItem); err != nil {
			return err
		}
		if err := s.refreshStudioBatchRunAfterRetryReset(ctx, runItem.RunID, now); err != nil {
			return err
		}
	}
	return nil
}

func selectStudioBatchRetryRunItem(runItems []StudioBatchRunItemRecord) (StudioBatchRunItemRecord, bool) {
	if len(runItems) != 1 {
		return StudioBatchRunItemRecord{}, false
	}
	return runItems[0], true
}

func (s *taskStudioBatchService) refreshStudioBatchRunAfterRetryReset(ctx context.Context, runID string, now time.Time) error {
	if s == nil || s.batchRunRepo == nil || strings.TrimSpace(runID) == "" {
		return nil
	}
	run, err := s.batchRunRepo.GetStudioBatchRun(ctx, runID)
	if err != nil {
		return err
	}
	items, err := s.batchRunRepo.ListStudioBatchRunItems(ctx, runID)
	if err != nil {
		return err
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
	return s.batchRunRepo.UpdateStudioBatchRun(ctx, run)
}

func countCompletedStudioBatchRunItems(items []StudioBatchRunItemRecord) int {
	return countSucceededStudioBatchRunItems(items) + countFailedStudioBatchRunItems(items)
}

func countSucceededStudioBatchRunItems(items []StudioBatchRunItemRecord) int {
	count := 0
	for _, item := range items {
		if item.Status == StudioBatchRunItemStatusSucceeded {
			count++
		}
	}
	return count
}

func countFailedStudioBatchRunItems(items []StudioBatchRunItemRecord) int {
	count := 0
	for _, item := range items {
		switch item.Status {
		case StudioBatchRunItemStatusFailed, StudioBatchRunItemStatusCancelled:
			count++
		}
	}
	return count
}

func firstStudioBatchRunItemError(items []StudioBatchRunItemRecord) string {
	for _, item := range items {
		if trimmed := strings.TrimSpace(item.ErrorMessage); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

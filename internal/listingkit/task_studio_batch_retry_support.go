package listingkit

import (
	"context"
	"fmt"
	"strings"
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
	detail *StudioBatchDetailGraph,
	itemIDs []string,
) ([]StudioBatchItemRecord, error) {
	if len(itemIDs) == 0 {
		return nil, NewStudioBatchActionValidationError("item_ids is required")
	}
	if detail == nil {
		return nil, NewStudioBatchActionValidationError("studio batch detail is required")
	}

	itemsByID := make(map[string]StudioBatchItemRecord, len(detail.Items))
	for _, item := range detail.Items {
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
	return itemsToRetry, nil
}

func (s *taskStudioBatchService) resetStudioBatchRetryItems(
	ctx context.Context,
	items []StudioBatchItemRecord,
) error {
	now := s.currentTime().UTC()
	for _, item := range items {
		item.Status = StudioBatchItemStatusPending
		item.LastError = ""
		item.UpdatedAt = now
		if err := s.repo.UpdateStudioBatchItem(ctx, &item); err != nil {
			return err
		}
	}
	return nil
}

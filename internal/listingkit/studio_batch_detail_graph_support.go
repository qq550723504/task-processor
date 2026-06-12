package listingkit

import (
	"slices"
)

func buildStudioBatchDetailGraph(
	batch *StudioBatchRecord,
	items []StudioBatchItemRecord,
	attempts []StudioGenerationAttemptRecord,
	designs []StudioMaterializedDesignRecord,
) *StudioBatchDetailGraph {
	if batch == nil {
		return &StudioBatchDetailGraph{}
	}

	sortedItems := append([]StudioBatchItemRecord(nil), items...)
	sortStudioBatchItems(sortedItems)

	itemIDs := make(map[string]struct{}, len(sortedItems))
	for _, item := range sortedItems {
		itemIDs[item.ID] = struct{}{}
	}

	attemptsByItem := map[string][]StudioGenerationAttemptRecord{}
	for _, attempt := range attempts {
		if _, ok := itemIDs[attempt.ItemID]; !ok {
			continue
		}
		attemptsByItem[attempt.ItemID] = append(attemptsByItem[attempt.ItemID], attempt)
	}
	for itemID := range attemptsByItem {
		sortStudioGenerationAttempts(attemptsByItem[itemID])
	}

	designsByItem := map[string][]StudioMaterializedDesignRecord{}
	for _, design := range designs {
		if _, ok := itemIDs[design.ItemID]; !ok {
			continue
		}
		designsByItem[design.ItemID] = append(designsByItem[design.ItemID], design)
	}
	for itemID := range designsByItem {
		sortStudioMaterializedDesigns(designsByItem[itemID])
	}

	clonedBatch := *batch
	return &StudioBatchDetailGraph{
		Batch:          &clonedBatch,
		Items:          sortedItems,
		AttemptsByItem: attemptsByItem,
		DesignsByItem:  designsByItem,
	}
}

func sortStudioBatchItems(items []StudioBatchItemRecord) {
	slices.SortStableFunc(items, func(a, b StudioBatchItemRecord) int {
		if a.CreatedAt.Before(b.CreatedAt) {
			return -1
		}
		if a.CreatedAt.After(b.CreatedAt) {
			return 1
		}
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
}

func sortStudioGenerationAttempts(attempts []StudioGenerationAttemptRecord) {
	slices.SortStableFunc(attempts, func(a, b StudioGenerationAttemptRecord) int {
		if a.AttemptNo < b.AttemptNo {
			return -1
		}
		if a.AttemptNo > b.AttemptNo {
			return 1
		}
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
}

func sortStudioMaterializedDesigns(designs []StudioMaterializedDesignRecord) {
	slices.SortStableFunc(designs, func(a, b StudioMaterializedDesignRecord) int {
		if a.SortOrder < b.SortOrder {
			return -1
		}
		if a.SortOrder > b.SortOrder {
			return 1
		}
		if a.CreatedAt.Before(b.CreatedAt) {
			return -1
		}
		if a.CreatedAt.After(b.CreatedAt) {
			return 1
		}
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
}

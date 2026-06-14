package listingkit

import studiodomain "task-processor/internal/listing/studio"

func studioBatchStatusSet() studiodomain.BatchStatusSet[StudioBatchItemStatus, StudioBatchStatus] {
	return studiodomain.BatchStatusSet[StudioBatchItemStatus, StudioBatchStatus]{
		Draft:                 StudioBatchStatusDraft,
		Generating:            StudioBatchStatusGenerating,
		PartiallyMaterialized: StudioBatchStatusPartiallyMaterialized,
		ReviewReady:           StudioBatchStatusReviewReady,
		PartiallyFailed:       StudioBatchStatusPartiallyFailed,
		Failed:                StudioBatchStatusFailed,
		ReviewReadyItem:       StudioBatchItemStatusReviewReady,
		FailedItem:            StudioBatchItemStatusFailed,
		ActiveItems: []StudioBatchItemStatus{
			StudioBatchItemStatusGenerating,
			StudioBatchItemStatusAwaitingMaterialization,
			StudioBatchItemStatusPending,
		},
	}
}

func studioBatchItemStatus(item *StudioBatchItemRecord) StudioBatchItemStatus {
	if item == nil {
		return ""
	}
	return item.Status
}

func resolveProjectedStudioBatchStatus(current StudioBatchStatus, items []StudioBatchItemRecord) StudioBatchStatus {
	return studiodomain.ResolveBatchStatus(
		current,
		items,
		studioBatchItemStatus,
		studioBatchStatusSet(),
		func(status StudioBatchStatus) bool {
			return status == StudioBatchStatusTasksCreated
		},
	)
}

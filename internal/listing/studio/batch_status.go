package studio

type BatchStatusSet[ItemStatus comparable, BatchStatus comparable] struct {
	Draft                 BatchStatus
	Generating            BatchStatus
	PartiallyMaterialized BatchStatus
	ReviewReady           BatchStatus
	PartiallyFailed       BatchStatus
	Failed                BatchStatus
	ReviewReadyItem       ItemStatus
	FailedItem            ItemStatus
	ActiveItems           []ItemStatus
}

// AggregateBatchStatus derives a studio batch status from item statuses.
func AggregateBatchStatus[Item any, ItemStatus comparable, BatchStatus comparable](
	items []Item,
	itemStatus func(*Item) ItemStatus,
	statuses BatchStatusSet[ItemStatus, BatchStatus],
) BatchStatus {
	if len(items) == 0 {
		return statuses.Draft
	}
	if itemStatus == nil {
		return statuses.Generating
	}

	activeStatuses := make(map[ItemStatus]struct{}, len(statuses.ActiveItems))
	for _, status := range statuses.ActiveItems {
		activeStatuses[status] = struct{}{}
	}

	reviewReady := 0
	failed := 0
	active := 0
	for index := range items {
		status := itemStatus(&items[index])
		switch status {
		case statuses.ReviewReadyItem:
			reviewReady++
		case statuses.FailedItem:
			failed++
		default:
			if _, ok := activeStatuses[status]; ok {
				active++
			}
		}
	}

	switch {
	case reviewReady == len(items):
		return statuses.ReviewReady
	case failed == len(items):
		return statuses.Failed
	case failed > 0 && reviewReady > 0:
		return statuses.PartiallyFailed
	case failed > 0 && active > 0:
		return statuses.PartiallyFailed
	case reviewReady > 0 && active > 0:
		return statuses.PartiallyMaterialized
	default:
		return statuses.Generating
	}
}

// ResolveBatchStatus preserves caller-owned terminal states before deriving an
// active batch status from item states.
func ResolveBatchStatus[Item any, ItemStatus comparable, BatchStatus comparable](
	current BatchStatus,
	items []Item,
	itemStatus func(*Item) ItemStatus,
	statuses BatchStatusSet[ItemStatus, BatchStatus],
	preserveCurrent func(BatchStatus) bool,
) BatchStatus {
	if preserveCurrent != nil && preserveCurrent(current) {
		return current
	}
	return AggregateBatchStatus(items, itemStatus, statuses)
}

package amazonlisting

import (
	"strings"

	amazonworkspace "task-processor/internal/marketplace/amazon/workspace"
	"task-processor/internal/product/catalog"
)

func buildReviewItemsFromSnapshot(snapshot *catalog.ProductSnapshot) []AmazonReviewItem {
	return amazonworkspace.BuildReviewItemsFromSnapshot(snapshot)
}

func appendReviewItem(draft *AmazonListingDraft, item AmazonReviewItem) {
	if draft == nil || strings.TrimSpace(item.Reason) == "" {
		return
	}
	draft.ReviewItems = dedupeReviewItems(append(draft.ReviewItems, item))
}

func dedupeReviewItems(items []AmazonReviewItem) []AmazonReviewItem {
	return amazonworkspace.DedupeReviewItems(items)
}

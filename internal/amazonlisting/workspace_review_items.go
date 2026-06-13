package amazonlisting

import (
	"strings"

	"task-processor/internal/catalog/canonical"
	amazonworkspace "task-processor/internal/marketplace/amazon/workspace"
)

func buildReviewItemsFromCanonical(product *canonical.Product) []AmazonReviewItem {
	return amazonworkspace.BuildReviewItemsFromCanonical(product)
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

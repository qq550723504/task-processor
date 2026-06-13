package amazonlisting

import (
	"task-processor/internal/catalog/canonical"
	amazonworkspace "task-processor/internal/marketplace/amazon/workspace"
)

func refreshCanonicalReviewItems(items []AmazonReviewItem, product *canonical.Product) []AmazonReviewItem {
	return amazonworkspace.RefreshCanonicalReviewItems(items, product)
}

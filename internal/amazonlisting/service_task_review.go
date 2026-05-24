package amazonlisting

import (
	"strings"

	"task-processor/internal/catalog/canonical"
)

func refreshCanonicalReviewItems(items []AmazonReviewItem, product *canonical.Product) []AmazonReviewItem {
	if product == nil {
		return items
	}
	filtered := make([]AmazonReviewItem, 0, len(items))
	for _, item := range items {
		switch item.Field {
		case "title", "brand", "category_path", "description", "selling_points", "seo_keywords", "attributes", "specifications", "product", "dimensions.unit", "weight.unit":
			continue
		default:
			if strings.HasPrefix(item.Field, "attributes.") {
				continue
			}
			if strings.HasPrefix(item.Field, "specifications.technical.") || strings.HasPrefix(item.Field, "dimensions.") || strings.HasPrefix(item.Field, "weight.") {
				continue
			}
			if strings.HasPrefix(item.Field, "package.") || strings.HasPrefix(item.Field, "variants[") {
				continue
			}
			filtered = append(filtered, item)
		}
	}
	return dedupeReviewItems(append(filtered, buildReviewItemsFromCanonical(product)...))
}

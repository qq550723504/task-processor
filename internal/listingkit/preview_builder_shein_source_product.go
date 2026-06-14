package listingkit

import (
	"task-processor/internal/catalog/canonical"
	sheinworkspace "task-processor/internal/listingkit/workspace/shein"
)

func buildSheinSourceProductSummary(product *canonical.Product) *SheinSourceProductSummary {
	return sheinworkspace.BuildSourceProductSummary(product)
}

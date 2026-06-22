package listingkit

import (
	"task-processor/internal/catalog/canonical"
	sheinworkspace "task-processor/internal/marketplace/shein/workspace"
)

func buildSheinSourceProductSummary(product *canonical.Product) *SheinSourceProductSummary {
	return sheinworkspace.BuildSourceProductSummary(product)
}

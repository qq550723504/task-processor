package shein

import (
	"task-processor/internal/catalog/canonical"
	sheinmarketplace "task-processor/internal/marketplace/shein/workspace"
)

type SourceProductSummary = sheinmarketplace.SourceProductSummary

func BuildSourceProductSummary(product *canonical.Product) *SourceProductSummary {
	return sheinmarketplace.BuildSourceProductSummary(product)
}

package shein

import (
	sheinmarketplace "task-processor/internal/marketplace/shein/workspace"
	sheinpub "task-processor/internal/publishing/shein"
)

func BuildCategoryPayload(pkg *sheinpub.Package) *sheinpub.InspectionCategoryPayload {
	return sheinmarketplace.BuildCategoryPayload(pkg)
}

func BuildAttributePayload(pkg *sheinpub.Package) *sheinpub.InspectionAttributePayload {
	return sheinmarketplace.BuildAttributePayload(pkg)
}

func BuildSaleAttributePayload(pkg *sheinpub.Package) *sheinpub.InspectionSaleAttributePayload {
	return sheinmarketplace.BuildSaleAttributePayload(pkg)
}

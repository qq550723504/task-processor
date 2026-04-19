package shein

import sheinpub "task-processor/internal/publishing/shein"

func BuildCategoryPayload(pkg *sheinpub.Package) *sheinpub.InspectionCategoryPayload {
	return buildCategoryPayload(pkg)
}

func BuildAttributePayload(pkg *sheinpub.Package) *sheinpub.InspectionAttributePayload {
	return buildAttributePayload(pkg)
}

func BuildSaleAttributePayload(pkg *sheinpub.Package) *sheinpub.InspectionSaleAttributePayload {
	return buildSaleAttributePayload(pkg)
}

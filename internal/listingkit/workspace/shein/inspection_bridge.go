package shein

import (
	sheinmarketplace "task-processor/internal/marketplace/shein/workspace"
)

func BuildInspection(pkg *Package) *Inspection {
	return sheinmarketplace.BuildInspection(pkg)
}

func BuildCategoryPayload(pkg *Package) *InspectionCategoryPayload {
	return sheinmarketplace.BuildCategoryPayload(pkg)
}

func BuildAttributePayload(pkg *Package) *InspectionAttributePayload {
	return sheinmarketplace.BuildAttributePayload(pkg)
}

func BuildSaleAttributePayload(pkg *Package) *InspectionSaleAttributePayload {
	return sheinmarketplace.BuildSaleAttributePayload(pkg)
}

func FilterManualReviewNotes(notes []string) []string {
	return sheinmarketplace.FilterManualReviewNotes(notes)
}

func IsCategoryResolved(pkg *Package) bool {
	return sheinmarketplace.IsCategoryResolved(pkg)
}

func IsAttributeResolved(pkg *Package) bool {
	return sheinmarketplace.IsAttributeResolved(pkg)
}

func IsSaleAttributeResolved(pkg *Package) bool {
	return sheinmarketplace.IsSaleAttributeResolved(pkg)
}

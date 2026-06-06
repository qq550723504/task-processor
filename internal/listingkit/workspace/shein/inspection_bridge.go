package shein

import sheinworkspace "task-processor/internal/workspace/shein"

func BuildInspection(pkg *Package) *Inspection {
	return sheinworkspace.BuildInspection(pkg)
}

func BuildCategoryPayload(pkg *Package) *InspectionCategoryPayload {
	return sheinworkspace.BuildCategoryPayload(pkg)
}

func BuildAttributePayload(pkg *Package) *InspectionAttributePayload {
	return sheinworkspace.BuildAttributePayload(pkg)
}

func BuildSaleAttributePayload(pkg *Package) *InspectionSaleAttributePayload {
	return sheinworkspace.BuildSaleAttributePayload(pkg)
}

func FilterManualReviewNotes(notes []string) []string {
	return sheinworkspace.FilterManualReviewNotes(notes)
}

func IsCategoryResolved(pkg *Package) bool {
	return sheinworkspace.IsCategoryResolved(pkg)
}

func IsAttributeResolved(pkg *Package) bool {
	return sheinworkspace.IsAttributeResolved(pkg)
}

func IsSaleAttributeResolved(pkg *Package) bool {
	return sheinworkspace.IsSaleAttributeResolved(pkg)
}

// Adapter-only bridge. Keep domain rules in internal/workspace/shein.
package listingkit

import sheinworkspace "task-processor/internal/workspace/shein"

func buildSheinInspection(pkg *SheinPackage) *SheinInspection {
	return sheinworkspace.BuildInspection(pkg)
}

func buildSheinCategoryPayload(pkg *SheinPackage) *SheinInspectionCategoryPayload {
	return sheinworkspace.BuildCategoryPayload(pkg)
}

func buildSheinAttributePayload(pkg *SheinPackage) *SheinInspectionAttributePayload {
	return sheinworkspace.BuildAttributePayload(pkg)
}

func buildSheinSaleAttributePayload(pkg *SheinPackage) *SheinInspectionSaleAttributePayload {
	return sheinworkspace.BuildSaleAttributePayload(pkg)
}

func filterManualSheinReviewNotes(notes []string) []string {
	return sheinworkspace.FilterManualReviewNotes(notes)
}

func isSheinCategoryResolved(pkg *SheinPackage) bool {
	return sheinworkspace.IsCategoryResolved(pkg)
}

func isSheinAttributeResolved(pkg *SheinPackage) bool {
	return sheinworkspace.IsAttributeResolved(pkg)
}

func isSheinSaleAttributeResolved(pkg *SheinPackage) bool {
	return sheinworkspace.IsSaleAttributeResolved(pkg)
}

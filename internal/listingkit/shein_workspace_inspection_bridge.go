// Adapter-only bridge. Keep domain rules in internal/marketplace/shein/workspace.
package listingkit

import listingworkspace "task-processor/internal/listingkit/workspace/shein"

func buildSheinInspection(pkg *SheinPackage) *SheinInspection {
	return listingworkspace.BuildInspection(pkg)
}

func buildSheinCategoryPayload(pkg *SheinPackage) *SheinInspectionCategoryPayload {
	return listingworkspace.BuildCategoryPayload(pkg)
}

func buildSheinAttributePayload(pkg *SheinPackage) *SheinInspectionAttributePayload {
	return listingworkspace.BuildAttributePayload(pkg)
}

func buildSheinSaleAttributePayload(pkg *SheinPackage) *SheinInspectionSaleAttributePayload {
	return listingworkspace.BuildSaleAttributePayload(pkg)
}

func filterManualSheinReviewNotes(notes []string) []string {
	return listingworkspace.FilterManualReviewNotes(notes)
}

func isSheinCategoryResolved(pkg *SheinPackage) bool {
	return listingworkspace.IsCategoryResolved(pkg)
}

func isSheinAttributeResolved(pkg *SheinPackage) bool {
	return listingworkspace.IsAttributeResolved(pkg)
}

func isSheinSaleAttributeResolved(pkg *SheinPackage) bool {
	return listingworkspace.IsSaleAttributeResolved(pkg)
}

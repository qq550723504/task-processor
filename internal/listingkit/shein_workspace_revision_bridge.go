// Adapter-only bridge. Keep domain rules in internal/workspace/shein.
package listingkit

import listingworkspace "task-processor/internal/listingkit/workspace/shein"

type SheinRevisionInput = listingworkspace.RevisionInput
type SheinCategoryResolutionPatch = listingworkspace.CategoryResolutionPatch
type SheinAttributeResolutionPatch = listingworkspace.AttributeResolutionPatch
type SheinSaleAttributeResolutionPatch = listingworkspace.SaleAttributeResolutionPatch
type SheinSKCRevisionPatch = listingworkspace.SKCRevisionPatch
type SheinSKURevisionPatch = listingworkspace.SKURevisionPatch
type SheinEditorRevisionSkeleton = listingworkspace.EditorRevisionSkeleton

func buildSheinEditorRevisionSkeleton(pkg *SheinPackage) *SheinEditorRevisionSkeleton {
	return listingworkspace.BuildEditorRevisionSkeleton(pkg)
}

func buildSheinMinimalRevisionSkeleton(pkg *SheinPackage) *SheinEditorRevisionSkeleton {
	return listingworkspace.BuildMinimalRevisionSkeleton(pkg)
}

func pruneSheinRevisionInput(input *SheinRevisionInput) *SheinRevisionInput {
	return listingworkspace.PruneRevisionInput(input)
}

func isEmptySheinRevisionInput(input *SheinRevisionInput) bool {
	return listingworkspace.IsEmptyRevisionInput(input)
}

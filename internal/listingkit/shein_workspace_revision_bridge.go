// Adapter-only bridge. Keep domain rules in internal/marketplace/shein/workspace.
package listingkit

import sheinworkspace "task-processor/internal/marketplace/shein/workspace"

type SheinRevisionInput = sheinworkspace.RevisionInput
type SheinCategoryResolutionPatch = sheinworkspace.CategoryResolutionPatch
type SheinAttributeResolutionPatch = sheinworkspace.AttributeResolutionPatch
type SheinSaleAttributeResolutionPatch = sheinworkspace.SaleAttributeResolutionPatch
type SheinSKCRevisionPatch = sheinworkspace.SKCRevisionPatch
type SheinSKURevisionPatch = sheinworkspace.SKURevisionPatch
type SheinEditorRevisionSkeleton = sheinworkspace.EditorRevisionSkeleton

func buildSheinEditorRevisionSkeleton(pkg *SheinPackage) *SheinEditorRevisionSkeleton {
	return sheinworkspace.BuildEditorRevisionSkeleton(
		pkg,
		buildSheinCategoryResolutionPatch(pkg),
		buildSheinAttributeResolutionPatch(pkg),
		buildSheinSaleAttributeResolutionPatch(pkg),
		buildSheinEditorSKCPatches(pkg),
	)
}

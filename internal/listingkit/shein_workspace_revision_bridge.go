// Adapter-only bridge. Keep domain rules in internal/workspace/shein.
package listingkit

import sheinworkspace "task-processor/internal/workspace/shein"

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

func buildSheinMinimalRevisionSkeleton(pkg *SheinPackage) *SheinEditorRevisionSkeleton {
	return sheinworkspace.BuildMinimalRevisionSkeleton(buildSheinEditorRevisionSkeleton(pkg))
}

func pruneSheinRevisionInput(input *SheinRevisionInput) *SheinRevisionInput {
	return sheinworkspace.PruneRevisionInput(input)
}

func isEmptySheinRevisionInput(input *SheinRevisionInput) bool {
	return sheinworkspace.IsEmptyRevisionInput(input)
}

func stringPointerOrNil(value string) *string {
	if value == "" {
		return nil
	}
	copied := value
	return &copied
}

func clonePlatformImageSetForEditor(set *PlatformImageSet) *PlatformImageSet {
	if set == nil {
		return nil
	}
	return &PlatformImageSet{
		MainImage:    set.MainImage,
		WhiteBgImage: set.WhiteBgImage,
		Gallery:      append([]string(nil), set.Gallery...),
		SourceImages: append([]string(nil), set.SourceImages...),
	}
}

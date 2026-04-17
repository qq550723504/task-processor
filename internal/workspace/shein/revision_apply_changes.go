package shein

import sheinpub "task-processor/internal/publishing/shein"

func BuildAppliedChangesPreview(before, after *sheinpub.Package) *RevisionDiffPreview {
	if before == nil || after == nil {
		return nil
	}
	return BuildRevisionDiffPreview(before, BuildMinimalRevisionSkeleton(BuildEditorRevisionSkeleton(
		after,
		BuildCategoryResolutionPatch(after),
		BuildAttributeResolutionPatch(after),
		BuildSaleAttributeResolutionPatch(after),
		BuildEditorSKCPatches(after),
	)))
}

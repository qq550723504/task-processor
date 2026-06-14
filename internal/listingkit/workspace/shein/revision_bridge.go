package shein

import sheinmarketplace "task-processor/internal/marketplace/shein/workspace"

func BuildEditorRevisionSkeleton(pkg *Package) *EditorRevisionSkeleton {
	return sheinmarketplace.BuildEditorRevisionSkeleton(
		pkg,
		BuildCategoryResolutionPatch(pkg),
		BuildAttributeResolutionPatch(pkg),
		BuildSaleAttributeResolutionPatch(pkg),
		BuildEditorSKCPatches(pkg),
	)
}

func BuildMinimalRevisionSkeleton(pkg *Package) *EditorRevisionSkeleton {
	return sheinmarketplace.BuildMinimalRevisionSkeleton(BuildEditorRevisionSkeleton(pkg))
}

func PruneRevisionInput(input *RevisionInput) *RevisionInput {
	return sheinmarketplace.PruneRevisionInput(input)
}

func IsEmptyRevisionInput(input *RevisionInput) bool {
	return sheinmarketplace.IsEmptyRevisionInput(input)
}

func BuildRevisionInputFromEditorContext(ctx *EditorContext) *RevisionInput {
	return sheinmarketplace.BuildRevisionInputFromEditorContext(ctx)
}

func BuildRestoreDraftFromSkeleton(reason string, skeleton *EditorRevisionSkeleton) *EditorRevisionSkeleton {
	return sheinmarketplace.BuildRestoreDraftFromSkeleton(reason, skeleton)
}

func CloneEditorRevisionSkeleton(src *EditorRevisionSkeleton) *EditorRevisionSkeleton {
	return sheinmarketplace.CloneEditorRevisionSkeleton(src)
}

func CloneRevisionInput(src *RevisionInput) *RevisionInput {
	return sheinmarketplace.CloneRevisionInput(src)
}

func BuildRestoreRequestSeed(draft *EditorRevisionSkeleton) *sheinmarketplace.RestoreRequestSeed {
	return sheinmarketplace.BuildRestoreRequestSeed(draft)
}

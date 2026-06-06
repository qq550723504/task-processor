package shein

import sheinworkspace "task-processor/internal/workspace/shein"

func BuildEditorRevisionSkeleton(pkg *Package) *EditorRevisionSkeleton {
	return sheinworkspace.BuildEditorRevisionSkeleton(
		pkg,
		BuildCategoryResolutionPatch(pkg),
		BuildAttributeResolutionPatch(pkg),
		BuildSaleAttributeResolutionPatch(pkg),
		BuildEditorSKCPatches(pkg),
	)
}

func BuildMinimalRevisionSkeleton(pkg *Package) *EditorRevisionSkeleton {
	return sheinworkspace.BuildMinimalRevisionSkeleton(BuildEditorRevisionSkeleton(pkg))
}

func PruneRevisionInput(input *RevisionInput) *RevisionInput {
	return sheinworkspace.PruneRevisionInput(input)
}

func IsEmptyRevisionInput(input *RevisionInput) bool {
	return sheinworkspace.IsEmptyRevisionInput(input)
}

func BuildRevisionInputFromEditorContext(ctx *EditorContext) *RevisionInput {
	return sheinworkspace.BuildRevisionInputFromEditorContext(ctx)
}

func BuildRestoreDraftFromSkeleton(reason string, skeleton *EditorRevisionSkeleton) *EditorRevisionSkeleton {
	return sheinworkspace.BuildRestoreDraftFromSkeleton(reason, skeleton)
}

func CloneEditorRevisionSkeleton(src *EditorRevisionSkeleton) *EditorRevisionSkeleton {
	return sheinworkspace.CloneEditorRevisionSkeleton(src)
}

func CloneRevisionInput(src *RevisionInput) *RevisionInput {
	return sheinworkspace.CloneRevisionInput(src)
}

func BuildRestoreRequestSeed(draft *EditorRevisionSkeleton) *sheinworkspace.RestoreRequestSeed {
	return sheinworkspace.BuildRestoreRequestSeed(draft)
}

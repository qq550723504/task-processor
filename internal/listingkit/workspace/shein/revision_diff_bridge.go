package shein

import sheinmarketplace "task-processor/internal/marketplace/shein/workspace"

type RevisionDiffPreview = sheinmarketplace.RevisionDiffPreview
type RevisionFieldChange = sheinmarketplace.RevisionFieldChange

func BuildRevisionDiffPreview(pkg *Package, revision *EditorRevisionSkeleton) *RevisionDiffPreview {
	return sheinmarketplace.BuildRevisionDiffPreview(pkg, revision)
}

func BuildRevisionDiffPreviewFromInput(revision *EditorRevisionSkeleton) *RevisionDiffPreview {
	return sheinmarketplace.BuildRevisionDiffPreviewFromInput(revision)
}

func BuildRevisionDiffBetweenRevisions(base, target *EditorRevisionSkeleton) *RevisionDiffPreview {
	return sheinmarketplace.BuildRevisionDiffBetweenRevisions(base, target)
}

func BuildAppliedChangesPreview(before, after *Package) *RevisionDiffPreview {
	return sheinmarketplace.BuildAppliedChangesPreview(before, after)
}

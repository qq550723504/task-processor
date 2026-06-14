package shein

import (
	sheinmarketplace "task-processor/internal/marketplace/shein/workspace"
	sheinpub "task-processor/internal/publishing/shein"
)

func BuildRevisionDiffPreview(pkg *sheinpub.Package, revision *EditorRevisionSkeleton) *RevisionDiffPreview {
	return sheinmarketplace.BuildRevisionDiffPreview(pkg, revision)
}

func BuildRevisionDiffPreviewFromInput(revision *EditorRevisionSkeleton) *RevisionDiffPreview {
	return sheinmarketplace.BuildRevisionDiffPreviewFromInput(revision)
}

func BuildRevisionDiffBetweenRevisions(base, target *EditorRevisionSkeleton) *RevisionDiffPreview {
	return sheinmarketplace.BuildRevisionDiffBetweenRevisions(base, target)
}

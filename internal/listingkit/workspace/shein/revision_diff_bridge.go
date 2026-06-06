package shein

import sheinworkspace "task-processor/internal/workspace/shein"

type RevisionDiffPreview = sheinworkspace.RevisionDiffPreview
type RevisionFieldChange = sheinworkspace.RevisionFieldChange

func BuildRevisionDiffPreview(pkg *Package, revision *EditorRevisionSkeleton) *RevisionDiffPreview {
	return sheinworkspace.BuildRevisionDiffPreview(pkg, revision)
}

func BuildRevisionDiffPreviewFromInput(revision *EditorRevisionSkeleton) *RevisionDiffPreview {
	return sheinworkspace.BuildRevisionDiffPreviewFromInput(revision)
}

func BuildRevisionDiffBetweenRevisions(base, target *EditorRevisionSkeleton) *RevisionDiffPreview {
	return sheinworkspace.BuildRevisionDiffBetweenRevisions(base, target)
}

func BuildAppliedChangesPreview(before, after *Package) *RevisionDiffPreview {
	return sheinworkspace.BuildAppliedChangesPreview(before, after)
}

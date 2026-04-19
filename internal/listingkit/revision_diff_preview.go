package listingkit

import sheinworkspace "task-processor/internal/workspace/shein"

type RevisionDiffPreview = sheinworkspace.RevisionDiffPreview
type RevisionFieldChange = sheinworkspace.RevisionFieldChange

func buildSheinRevisionDiffPreview(pkg *SheinPackage, revision *SheinEditorRevisionSkeleton) *RevisionDiffPreview {
	return sheinworkspace.BuildRevisionDiffPreview(pkg, revision)
}

func buildSheinRevisionDiffPreviewFromInput(revision *SheinEditorRevisionSkeleton) *RevisionDiffPreview {
	return sheinworkspace.BuildRevisionDiffPreviewFromInput(revision)
}

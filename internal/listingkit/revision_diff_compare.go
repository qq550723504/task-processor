package listingkit

import sheinworkspace "task-processor/internal/workspace/shein"

func buildSheinRevisionDiffBetweenRevisions(base, target *SheinEditorRevisionSkeleton) *RevisionDiffPreview {
	return sheinworkspace.BuildRevisionDiffBetweenRevisions(base, target)
}

package shein

import (
	sheinmarketplace "task-processor/internal/marketplace/shein/workspace"
	sheinpub "task-processor/internal/publishing/shein"
)

func BuildAppliedChangesPreview(before, after *sheinpub.Package) *RevisionDiffPreview {
	return sheinmarketplace.BuildAppliedChangesPreview(before, after)
}

package listingkit

import (
	"strings"

	sheinworkspace "task-processor/internal/listingkit/workspace/shein"
	sheinpub "task-processor/internal/publishing/shein"
)

func buildAppliedChangesPreview(platform string, before, after *ListingKitResult) *RevisionDiffPreview {
	platform = strings.ToLower(strings.TrimSpace(platform))
	switch platform {
	case "shein":
		if before == nil || after == nil || before.Shein == nil || after.Shein == nil {
			return nil
		}
		return buildSheinAppliedChanges(before.Shein, after.Shein)
	default:
		return nil
	}
}

func buildSheinAppliedChanges(before, after *sheinpub.Package) *RevisionDiffPreview {
	return sheinworkspace.BuildAppliedChangesPreview(before, after)
}

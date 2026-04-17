package listingkit

import "strings"
import sheinpub "task-processor/internal/publishing/shein"
import sheinworkspace "task-processor/internal/workspace/shein"

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

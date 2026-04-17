package listingkit

import "strings"

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

func buildSheinAppliedChanges(before, after *SheinPackage) *RevisionDiffPreview {
	if before == nil || after == nil {
		return nil
	}
	return buildSheinRevisionDiffPreview(before, buildSheinMinimalRevisionSkeleton(after))
}

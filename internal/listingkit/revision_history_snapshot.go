package listingkit

import "strings"

func buildRevisionHistorySnapshot(platform string, result *ListingKitResult) *SheinEditorContext {
	if result == nil {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "shein":
		if result.Shein == nil {
			return nil
		}
		return buildSheinEditorContext(result.Shein)
	default:
		return nil
	}
}

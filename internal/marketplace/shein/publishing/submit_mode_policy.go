package publishing

import "strings"

// NormalizeFinalDraftSubmitMode normalizes user-selected final draft submit mode.
func NormalizeFinalDraftSubmitMode(mode string) string {
	normalized := strings.ToLower(strings.TrimSpace(mode))
	switch normalized {
	case "publish", "save_draft":
		return normalized
	default:
		return ""
	}
}

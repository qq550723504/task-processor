package submission

import (
	"fmt"
	"strings"
)

func NormalizeSubmitAction(action, fallback string) string {
	action = strings.ToLower(strings.TrimSpace(action))
	if action != "" {
		return action
	}
	return strings.ToLower(strings.TrimSpace(fallback))
}

func IsSupportedSubmitAction(action string) bool {
	switch NormalizeSubmitAction(action, "") {
	case "publish", "save_draft":
		return true
	default:
		return false
	}
}

func PreferredSubmitAction(candidates ...string) string {
	for _, candidate := range candidates {
		action := NormalizeSubmitAction(candidate, "")
		if IsSupportedSubmitAction(action) {
			return action
		}
	}
	return ""
}

func UnsupportedSubmitActionError(action string) error {
	return fmt.Errorf("unsupported submit action: %s", action)
}

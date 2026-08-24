package listingkit

import (
	"strings"
)

// ImageAIClientNameGPTImage2 is the canonical tenant/user client used by the
// default image-generation route and its settings health check.
const ImageAIClientNameGPTImage2 = "image_gpt_image_2"

func aiSettingsUserID(identity RequestIdentity, scope string) string {
	if strings.EqualFold(strings.TrimSpace(scope), "tenant") {
		return ""
	}
	return strings.TrimSpace(identity.UserID)
}

func normalizeAISettingsScope(scope string, userID string) string {
	if strings.EqualFold(strings.TrimSpace(scope), "tenant") || userID == "" {
		return "tenant"
	}
	return "user"
}

func normalizeAIClientName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "default"
	}
	return name
}

package submission

import "strings"

func ResolveRefreshAction(lastAction string, hasPublish, hasSaveDraft bool) string {
	if action := strings.TrimSpace(lastAction); action != "" {
		return action
	}
	if hasPublish {
		return "publish"
	}
	if hasSaveDraft {
		return "save_draft"
	}
	return ""
}

func ResolveRefreshSupplierCode(recordSupplierCode, packageSupplierCode string) string {
	if value := strings.TrimSpace(recordSupplierCode); value != "" {
		return value
	}
	return strings.TrimSpace(packageSupplierCode)
}

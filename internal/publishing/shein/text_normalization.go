package shein

import sheinmarketpub "task-processor/internal/marketplace/shein/publishing"

func normalizeSheinContentText(text string) string {
	return sheinmarketpub.NormalizeSheinContentText(text)
}

func sanitizeSheinAttributeText(value string) string {
	return sheinmarketpub.SanitizeSheinAttributeText(value)
}

func isValidSheinAttributeText(value string) bool {
	return sheinmarketpub.IsValidSheinAttributeText(value)
}

func isOnlySheinAttributeSpecialChars(value string) bool {
	return sheinmarketpub.IsOnlySheinAttributeSpecialChars(value)
}

func removeRemainingSheinAttributeSpecialChars(text string) string {
	return sheinmarketpub.RemoveRemainingSheinAttributeSpecialChars(text)
}

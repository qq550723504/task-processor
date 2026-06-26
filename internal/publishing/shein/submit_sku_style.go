package shein

import sheinmarketpub "task-processor/internal/marketplace/shein/publishing"

// LooksLikeSubmitRequestToken reports whether a SKU token looks like a submit request discriminator.
func LooksLikeSubmitRequestToken(token string) bool {
	return sheinmarketpub.LooksLikeSubmitRequestToken(token)
}

// LooksLikeSubmitTaskToken reports whether a SKU token looks like a submit task discriminator.
func LooksLikeSubmitTaskToken(token string) bool {
	return sheinmarketpub.LooksLikeSubmitTaskToken(token)
}

// DeriveSubmitStyleSuffix derives a compact style suffix from product text candidates.
func DeriveSubmitStyleSuffix(values ...string) string {
	return sheinmarketpub.DeriveSubmitStyleSuffix(values...)
}

// TokenizeStyleSuffixWords tokenizes text candidates for style suffix derivation.
func TokenizeStyleSuffixWords(value string) []string {
	return sheinmarketpub.TokenizeStyleSuffixWords(value)
}

// SubmitTaskDiscriminator returns the task-scoped SKU discriminator for a task ID.
func SubmitTaskDiscriminator(taskID string) string {
	return sheinmarketpub.SubmitTaskDiscriminator(taskID)
}

// SubmitRequestDiscriminator returns the request-scoped SKU discriminator for a request ID.
func SubmitRequestDiscriminator(requestID string) string {
	return sheinmarketpub.SubmitRequestDiscriminator(requestID)
}

// CombineSubmitDiscriminators joins non-empty submit discriminator parts.
func CombineSubmitDiscriminators(values ...string) string {
	return sheinmarketpub.CombineSubmitDiscriminators(values...)
}

// NormalizeSubmitStyleSuffix normalizes a style suffix to its SKU-safe 8-character form.
func NormalizeSubmitStyleSuffix(value string) string {
	return sheinmarketpub.NormalizeSubmitStyleSuffix(value)
}

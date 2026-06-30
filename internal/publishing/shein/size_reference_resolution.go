package shein

import sheinmarketpub "task-processor/internal/marketplace/shein/publishing"

// SizeReferenceVariantInput describes an SDS variant's source size-reference images.
type SizeReferenceVariantInput = sheinmarketpub.SizeReferenceVariantInput

// SizeReferenceVariantSummary describes rendered SDS output for a variant.
type SizeReferenceVariantSummary = sheinmarketpub.SizeReferenceVariantSummary

// ResolveRenderedSizeReferenceImages maps raw size-reference mockups to rendered SDS images.
func ResolveRenderedSizeReferenceImages(rawRefs []string, sourceMockups []string, renderedMockups []string) []string {
	return sheinmarketpub.ResolveRenderedSizeReferenceImages(rawRefs, sourceMockups, renderedMockups)
}

// FindSizeReferenceVariantSummary finds the rendered SDS summary matching a variant option.
func FindSizeReferenceVariantSummary(variant SizeReferenceVariantInput, summaries []SizeReferenceVariantSummary) (SizeReferenceVariantSummary, bool) {
	return sheinmarketpub.FindSizeReferenceVariantSummary(variant, summaries)
}

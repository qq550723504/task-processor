package shein

import sheinmarketpub "task-processor/internal/marketplace/shein/publishing"

// SubmitVariantOption is the subset of an SDS variant needed for SHEIN submit SKU normalization.
type SubmitVariantOption = sheinmarketpub.SubmitVariantOption

// SubmitVariantContext is the subset of SDS options needed for SHEIN submit SKU normalization.
type SubmitVariantContext = sheinmarketpub.SubmitVariantContext

// MatchSubmitVariantOptionIndex matches a draft SKU to an SDS variant and returns its index.
func MatchSubmitVariantOptionIndex(input *SubmitVariantContext, draftSKCValue string, draftSKU *SKUDraft, globalIndex int) int {
	return sheinmarketpub.MatchSubmitVariantOptionIndex(input, draftSKCValue, adaptSubmitDraftSKU(draftSKU), globalIndex)
}

// SubmitVariantMatches reports whether a variant matches draft color/size attributes.
func SubmitVariantMatches(item *SubmitVariantOption, color, size string) bool {
	return sheinmarketpub.SubmitVariantMatches(item, color, size)
}

// ResolveSubmitBaseSKU resolves the base SKU for a draft SKU and optional matched SDS variant.
func ResolveSubmitBaseSKU(input *SubmitVariantContext, draftSKU *SKUDraft, match *SubmitVariantOption, oldSKU string) string {
	return sheinmarketpub.ResolveSubmitBaseSKU(input, adaptSubmitDraftSKU(draftSKU), match, oldSKU)
}

// ResolveSubmitVariantDiscriminator resolves the variant discriminator for a draft SKU.
func ResolveSubmitVariantDiscriminator(input *SubmitVariantContext, draftSKU *SKUDraft, match *SubmitVariantOption, matchedIndex, globalIndex int, taskDiscriminator string) string {
	return sheinmarketpub.ResolveSubmitVariantDiscriminator(input, adaptSubmitDraftSKU(draftSKU), match, matchedIndex, globalIndex, taskDiscriminator)
}

// SubmitRequiresVariantDiscriminator reports whether the normalized SKU needs a variant discriminator.
func SubmitRequiresVariantDiscriminator(input *SubmitVariantContext, baseSKU string) bool {
	return sheinmarketpub.SubmitRequiresVariantDiscriminator(input, baseSKU)
}

// InferSubmitBaseSKUFromOld removes style and variant suffixes from a legacy studio submit SKU.
func InferSubmitBaseSKUFromOld(oldSKU, styleID string) string {
	return sheinmarketpub.InferSubmitBaseSKUFromOld(oldSKU, styleID)
}

func normalizeSubmitVariantDiscriminator(value string) string {
	return sheinmarketpub.NormalizeSubmitVariantDiscriminator(value)
}

func adaptSubmitDraftSKU(draftSKU *SKUDraft) *sheinmarketpub.SubmitDraftSKU {
	if draftSKU == nil {
		return nil
	}
	return &sheinmarketpub.SubmitDraftSKU{Attributes: draftSKU.Attributes}
}

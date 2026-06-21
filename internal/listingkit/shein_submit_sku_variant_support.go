package listingkit

import (
	"strconv"

	sheinpub "task-processor/internal/publishing/shein"
)

func matchStudioSubmitVariantOption(sds *SDSSyncOptions, draftSKC *SheinSKCRequestDraft, draftSKU *sheinpub.SKUDraft, globalIndex int) (*SDSSyncVariantOption, int) {
	index := sheinpub.MatchSubmitVariantOptionIndex(adaptSubmitVariantContext(sds), sheinDraftSKCSaleAttributeValue(draftSKC), draftSKU, globalIndex)
	if index < 0 || sds == nil || index >= len(sds.Variants) {
		return nil, -1
	}
	return &sds.Variants[index], index
}

func studioSubmitVariantMatches(item *SDSSyncVariantOption, color, size string) bool {
	return sheinpub.SubmitVariantMatches(adaptSubmitVariantOption(item), color, size)
}

func resolveStudioSubmitBaseSKU(sds *SDSSyncOptions, draftSKU *sheinpub.SKUDraft, match *SDSSyncVariantOption, oldSKU string) string {
	return sheinpub.ResolveSubmitBaseSKU(adaptSubmitVariantContext(sds), draftSKU, adaptSubmitVariantOption(match), oldSKU)
}

func resolveStudioSubmitVariantDiscriminator(sds *SDSSyncOptions, draftSKU *sheinpub.SKUDraft, match *SDSSyncVariantOption, matchedIndex, globalIndex int, taskDiscriminator string) string {
	return sheinpub.ResolveSubmitVariantDiscriminator(adaptSubmitVariantContext(sds), draftSKU, adaptSubmitVariantOption(match), matchedIndex, globalIndex, taskDiscriminator)
}

func studioSubmitRequiresVariantDiscriminator(sds *SDSSyncOptions, baseSKU string) bool {
	return sheinpub.SubmitRequiresVariantDiscriminator(adaptSubmitVariantContext(sds), baseSKU)
}

func inferStudioSubmitBaseSKUFromOld(oldSKU, styleID string) string {
	return sheinpub.InferSubmitBaseSKUFromOld(oldSKU, styleID)
}

func sheinDraftSKCSaleAttributeValue(draft *SheinSKCRequestDraft) string {
	if draft == nil || draft.SaleAttribute == nil {
		return ""
	}
	return draft.SaleAttribute.Value
}

func itoa(value int) string {
	return strconv.Itoa(value)
}

func adaptSubmitVariantContext(sds *SDSSyncOptions) *sheinpub.SubmitVariantContext {
	if sds == nil {
		return nil
	}
	variants := make([]sheinpub.SubmitVariantOption, 0, len(sds.Variants))
	for i := range sds.Variants {
		variants = append(variants, *adaptSubmitVariantOption(&sds.Variants[i]))
	}
	return &sheinpub.SubmitVariantContext{
		VariantID:    sds.VariantID,
		VariantSKU:   sds.VariantSKU,
		VariantSize:  sds.VariantSize,
		VariantColor: sds.VariantColor,
		ProductSKU:   sds.ProductSKU,
		StyleID:      sds.StyleID,
		Variants:     variants,
	}
}

func adaptSubmitVariantOption(item *SDSSyncVariantOption) *sheinpub.SubmitVariantOption {
	if item == nil {
		return nil
	}
	return &sheinpub.SubmitVariantOption{
		VariantID:  item.VariantID,
		VariantSKU: item.VariantSKU,
		Size:       item.Size,
		Color:      item.Color,
	}
}

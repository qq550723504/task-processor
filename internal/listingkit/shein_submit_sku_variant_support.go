package listingkit

import (
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
)

func matchStudioSubmitVariantOption(sds *SDSSyncOptions, draftSKC *SheinSKCRequestDraft, draftSKU *sheinpub.SKUDraft, globalIndex int) (*SDSSyncVariantOption, int) {
	if sds == nil || len(sds.Variants) == 0 {
		return nil, -1
	}

	sourceSKU := strings.TrimSpace(draftSKU.Attributes["source_sds_sku"])
	color := firstNonEmptyString(
		draftSKU.Attributes["Color"],
		draftSKU.Attributes["color"],
		sheinDraftSKCSaleAttributeValue(draftSKC),
	)
	size := firstNonEmptyString(
		draftSKU.Attributes["Size"],
		draftSKU.Attributes["size"],
	)

	if sourceSKU != "" {
		for i := range sds.Variants {
			if strings.EqualFold(strings.TrimSpace(sds.Variants[i].VariantSKU), sourceSKU) {
				return &sds.Variants[i], i
			}
		}
	}

	if color != "" || size != "" {
		for i := range sds.Variants {
			if studioSubmitVariantMatches(&sds.Variants[i], color, size) {
				return &sds.Variants[i], i
			}
		}
	}

	colorMatches := make([]int, 0, len(sds.Variants))
	if color != "" {
		for i := range sds.Variants {
			if strings.EqualFold(strings.TrimSpace(sds.Variants[i].Color), strings.TrimSpace(color)) {
				colorMatches = append(colorMatches, i)
			}
		}
		if len(colorMatches) == 1 {
			return &sds.Variants[colorMatches[0]], colorMatches[0]
		}
	}

	if globalIndex >= 0 && globalIndex < len(sds.Variants) {
		return &sds.Variants[globalIndex], globalIndex
	}
	return nil, -1
}

func studioSubmitVariantMatches(item *SDSSyncVariantOption, color, size string) bool {
	if item == nil {
		return false
	}
	if color != "" && !strings.EqualFold(strings.TrimSpace(item.Color), strings.TrimSpace(color)) {
		return false
	}
	if size != "" && !strings.EqualFold(strings.TrimSpace(item.Size), strings.TrimSpace(size)) {
		return false
	}
	return color != "" || size != ""
}

func resolveStudioSubmitBaseSKU(sds *SDSSyncOptions, draftSKU *sheinpub.SKUDraft, match *SDSSyncVariantOption, oldSKU string) string {
	if match != nil {
		return firstNonEmptyString(match.VariantSKU, sds.VariantSKU, sds.ProductSKU)
	}
	return firstNonEmptyString(
		draftSKU.Attributes["source_sds_sku"],
		sds.VariantSKU,
		sds.ProductSKU,
		inferStudioSubmitBaseSKUFromOld(oldSKU, sds.StyleID),
	)
}

func resolveStudioSubmitVariantDiscriminator(sds *SDSSyncOptions, draftSKU *sheinpub.SKUDraft, match *SDSSyncVariantOption, matchedIndex, globalIndex int, taskDiscriminator string) string {
	base := ""
	if match != nil {
		base = studioVariantDiscriminator(*match, matchedIndex)
	} else {
		color := firstNonEmptyString(draftSKU.Attributes["Color"], draftSKU.Attributes["color"])
		size := firstNonEmptyString(draftSKU.Attributes["Size"], draftSKU.Attributes["size"])
		if color != "" || size != "" {
			base = strings.Join([]string{
				strings.TrimSpace(color),
				strings.TrimSpace(size),
				"V" + itoa(globalIndex+1),
			}, "-")
		} else {
			base = studioFallbackVariantDiscriminator(sds)
		}
	}
	if taskDiscriminator == "" {
		return base
	}
	if base == "" {
		return taskDiscriminator
	}
	return strings.Join([]string{
		base,
		taskDiscriminator,
	}, "-")
}

func studioSubmitRequiresVariantDiscriminator(sds *SDSSyncOptions, baseSKU string) bool {
	if sds == nil {
		return false
	}
	if len(sds.Variants) > 0 {
		key := strings.TrimSpace(baseSKU)
		if key == "" {
			key = "__empty__"
		}
		return studioVariantBaseSKUCounts(sds)[key] > 1
	}
	return strings.TrimSpace(sds.VariantSKU) == ""
}

func inferStudioSubmitBaseSKUFromOld(oldSKU, styleID string) string {
	oldSKU = strings.TrimSpace(oldSKU)
	if oldSKU == "" {
		return ""
	}
	styleSuffix := normalizeStyleIDSuffix(styleID)
	if styleSuffix == "" {
		return oldSKU
	}
	suffix := "-" + styleSuffix
	upper := strings.ToUpper(oldSKU)
	if !strings.HasSuffix(upper, suffix) {
		return oldSKU
	}
	base := strings.TrimSpace(oldSKU[:len(oldSKU)-len(suffix)])
	if idx := strings.LastIndex(base, "-"); idx > 0 {
		prefix := strings.TrimSpace(base[:idx])
		discriminator := normalizeStudioVariantDiscriminator(base[idx+1:])
		if prefix != "" && discriminator != "" && strings.HasPrefix(discriminator, "V") {
			return prefix
		}
	}
	return base
}

func sheinDraftSKCSaleAttributeValue(draft *SheinSKCRequestDraft) string {
	if draft == nil || draft.SaleAttribute == nil {
		return ""
	}
	return draft.SaleAttribute.Value
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := [20]byte{}
	index := len(digits)
	v := value
	for v > 0 {
		index--
		digits[index] = byte('0' + v%10)
		v /= 10
	}
	return string(digits[index:])
}

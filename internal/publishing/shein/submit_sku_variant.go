package shein

import (
	"strconv"
	"strings"
)

// SubmitVariantOption is the subset of an SDS variant needed for SHEIN submit SKU normalization.
type SubmitVariantOption struct {
	VariantID  int64
	VariantSKU string
	Size       string
	Color      string
}

// SubmitVariantContext is the subset of SDS options needed for SHEIN submit SKU normalization.
type SubmitVariantContext struct {
	VariantID    int64
	VariantSKU   string
	VariantSize  string
	VariantColor string
	ProductSKU   string
	StyleID      string
	Variants     []SubmitVariantOption
}

// MatchSubmitVariantOptionIndex matches a draft SKU to an SDS variant and returns its index.
func MatchSubmitVariantOptionIndex(input *SubmitVariantContext, draftSKCValue string, draftSKU *SKUDraft, globalIndex int) int {
	if input == nil || len(input.Variants) == 0 {
		return -1
	}
	sourceSKU := ""
	color := draftSKCValue
	size := ""
	if draftSKU != nil {
		sourceSKU = strings.TrimSpace(draftSKU.Attributes["source_sds_sku"])
		color = firstSubmitNonEmptyString(draftSKU.Attributes["Color"], draftSKU.Attributes["color"], draftSKCValue)
		size = firstSubmitNonEmptyString(draftSKU.Attributes["Size"], draftSKU.Attributes["size"])
	}

	if sourceSKU != "" {
		for i := range input.Variants {
			if strings.EqualFold(strings.TrimSpace(input.Variants[i].VariantSKU), sourceSKU) {
				return i
			}
		}
	}

	if color != "" || size != "" {
		for i := range input.Variants {
			if SubmitVariantMatches(&input.Variants[i], color, size) {
				return i
			}
		}
	}

	colorMatches := make([]int, 0, len(input.Variants))
	if color != "" {
		for i := range input.Variants {
			if strings.EqualFold(strings.TrimSpace(input.Variants[i].Color), strings.TrimSpace(color)) {
				colorMatches = append(colorMatches, i)
			}
		}
		if len(colorMatches) == 1 {
			return colorMatches[0]
		}
	}

	if globalIndex >= 0 && globalIndex < len(input.Variants) {
		return globalIndex
	}
	return -1
}

// SubmitVariantMatches reports whether a variant matches draft color/size attributes.
func SubmitVariantMatches(item *SubmitVariantOption, color, size string) bool {
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

// ResolveSubmitBaseSKU resolves the base SKU for a draft SKU and optional matched SDS variant.
func ResolveSubmitBaseSKU(input *SubmitVariantContext, draftSKU *SKUDraft, match *SubmitVariantOption, oldSKU string) string {
	if match != nil {
		return firstSubmitNonEmptyString(match.VariantSKU, submitVariantSKU(input), submitProductSKU(input))
	}
	sourceSKU := ""
	if draftSKU != nil {
		sourceSKU = draftSKU.Attributes["source_sds_sku"]
	}
	return firstSubmitNonEmptyString(
		sourceSKU,
		submitVariantSKU(input),
		submitProductSKU(input),
		InferSubmitBaseSKUFromOld(oldSKU, submitStyleID(input)),
	)
}

// ResolveSubmitVariantDiscriminator resolves the variant discriminator for a draft SKU.
func ResolveSubmitVariantDiscriminator(input *SubmitVariantContext, draftSKU *SKUDraft, match *SubmitVariantOption, matchedIndex, globalIndex int, taskDiscriminator string) string {
	base := ""
	if match != nil {
		base = submitVariantDiscriminator(*match, matchedIndex)
	} else {
		color := ""
		size := ""
		if draftSKU != nil {
			color = firstSubmitNonEmptyString(draftSKU.Attributes["Color"], draftSKU.Attributes["color"])
			size = firstSubmitNonEmptyString(draftSKU.Attributes["Size"], draftSKU.Attributes["size"])
		}
		if color != "" || size != "" {
			base = strings.Join([]string{
				strings.TrimSpace(color),
				strings.TrimSpace(size),
				"V" + strconv.Itoa(globalIndex+1),
			}, "-")
		} else {
			base = submitFallbackVariantDiscriminator(input)
		}
	}
	if taskDiscriminator == "" {
		return base
	}
	if base == "" {
		return taskDiscriminator
	}
	return strings.Join([]string{base, taskDiscriminator}, "-")
}

// SubmitRequiresVariantDiscriminator reports whether the normalized SKU needs a variant discriminator.
func SubmitRequiresVariantDiscriminator(input *SubmitVariantContext, baseSKU string) bool {
	if input == nil {
		return false
	}
	if len(input.Variants) > 0 {
		key := strings.TrimSpace(baseSKU)
		if key == "" {
			key = "__empty__"
		}
		return submitVariantBaseSKUCounts(input)[key] > 1
	}
	return strings.TrimSpace(input.VariantSKU) == ""
}

// InferSubmitBaseSKUFromOld removes style and variant suffixes from a legacy studio submit SKU.
func InferSubmitBaseSKUFromOld(oldSKU, styleID string) string {
	oldSKU = strings.TrimSpace(oldSKU)
	if oldSKU == "" {
		return ""
	}
	styleSuffix := NormalizeSubmitStyleSuffix(styleID)
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
		discriminator := normalizeSubmitVariantDiscriminator(base[idx+1:])
		if prefix != "" && discriminator != "" && strings.HasPrefix(discriminator, "V") {
			return prefix
		}
	}
	return base
}

func submitVariantDiscriminator(item SubmitVariantOption, index int) string {
	if item.VariantID > 0 {
		return "V" + strconv.FormatInt(item.VariantID, 10)
	}
	return strings.Join([]string{
		strings.TrimSpace(item.Color),
		strings.TrimSpace(item.Size),
		"V" + strconv.Itoa(index+1),
	}, "-")
}

func submitFallbackVariantDiscriminator(input *SubmitVariantContext) string {
	if input == nil {
		return ""
	}
	if input.VariantID > 0 {
		return "V" + strconv.FormatInt(input.VariantID, 10)
	}
	if strings.TrimSpace(input.VariantSKU) != "" {
		return ""
	}
	return strings.Join([]string{
		strings.TrimSpace(input.VariantColor),
		strings.TrimSpace(input.VariantSize),
	}, "-")
}

func normalizeSubmitVariantDiscriminator(value string) string {
	value = strings.TrimSpace(strings.ToUpper(value))
	if value == "" {
		return ""
	}
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == '-' || r == '_' || r == ' ' || r == '/':
			if b.Len() == 0 || lastDash {
				continue
			}
			b.WriteRune('-')
			lastDash = true
		}
	}
	result := strings.Trim(b.String(), "-")
	if len(result) > 24 {
		result = result[:24]
		result = strings.TrimRight(result, "-")
	}
	return result
}

func submitVariantBaseSKUCounts(input *SubmitVariantContext) map[string]int {
	counts := map[string]int{}
	if input == nil {
		return counts
	}
	for _, item := range input.Variants {
		key := firstSubmitNonEmptyString(item.VariantSKU, input.VariantSKU, input.ProductSKU)
		if strings.TrimSpace(key) == "" {
			key = "__empty__"
		}
		counts[key]++
	}
	return counts
}

func firstSubmitNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func submitVariantSKU(input *SubmitVariantContext) string {
	if input == nil {
		return ""
	}
	return input.VariantSKU
}

func submitProductSKU(input *SubmitVariantContext) string {
	if input == nil {
		return ""
	}
	return input.ProductSKU
}

func submitStyleID(input *SubmitVariantContext) string {
	if input == nil {
		return ""
	}
	return input.StyleID
}

package listingkit

import (
	"strconv"
	"strings"

	"task-processor/internal/catalog/canonical"
)

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func addAttribute(attrs map[string]canonical.Attribute, key, value string, trace canonical.FieldTrace) {
	if strings.TrimSpace(value) == "" {
		return
	}
	attrs[key] = canonical.Attribute{Value: strings.TrimSpace(value), Trace: trace}
}

func addTechnicalSpec(specs map[string]string, key, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	specs[key] = strings.TrimSpace(value)
}

func studioStyleName(sds *SDSSyncOptions) string {
	if sds == nil {
		return ""
	}
	if name := strings.TrimSpace(sds.StyleName); name != "" {
		return name
	}
	if suffix := normalizeStyleIDSuffix(sds.StyleID); suffix != "" {
		return "Style " + suffix
	}
	return ""
}

func normalizeStyleIDSuffix(value string) string {
	value = strings.TrimSpace(strings.ToUpper(value))
	if value == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		}
		if b.Len() >= 8 {
			break
		}
	}
	return b.String()
}

func buildStudioVariantSKU(baseSKU, styleID, variantDiscriminator string, requireVariantDiscriminator bool, seen map[string]int) string {
	baseSKU = strings.TrimSpace(baseSKU)
	styleSuffix := normalizeStyleIDSuffix(styleID)
	variantDiscriminator = normalizeStudioVariantDiscriminator(variantDiscriminator)

	parts := make([]string, 0, 2)
	if baseSKU != "" {
		parts = append(parts, baseSKU)
	}
	if styleSuffix != "" {
		parts = append(parts, styleSuffix)
	}
	baseCandidate := strings.Join(parts, "-")
	if baseCandidate == "" {
		baseCandidate = "SDS-STUDIO-001"
	}
	if !requireVariantDiscriminator && seen == nil {
		return baseCandidate
	}
	if !requireVariantDiscriminator {
		if _, exists := seen[baseCandidate]; !exists {
			seen[baseCandidate] = 1
			return baseCandidate
		}
	}
	parts = parts[:0]
	if baseSKU != "" {
		parts = append(parts, baseSKU)
	}
	if variantDiscriminator != "" {
		parts = append(parts, variantDiscriminator)
	}
	if styleSuffix != "" {
		parts = append(parts, styleSuffix)
	}
	candidate := strings.Join(parts, "-")
	if candidate == "" {
		candidate = baseCandidate
	}
	if seen == nil {
		return candidate
	}
	if _, exists := seen[candidate]; !exists {
		seen[candidate] = 1
		return candidate
	}
	seen[candidate]++
	return candidate + "-" + strconv.Itoa(seen[candidate])
}

func studioVariantDiscriminator(item SDSSyncVariantOption, index int) string {
	if item.VariantID > 0 {
		return "V" + strconv.FormatInt(item.VariantID, 10)
	}
	return strings.Join([]string{
		strings.TrimSpace(item.Color),
		strings.TrimSpace(item.Size),
		"V" + strconv.Itoa(index+1),
	}, "-")
}

func studioFallbackVariantDiscriminator(sds *SDSSyncOptions) string {
	if sds == nil {
		return ""
	}
	if sds.VariantID > 0 {
		return "V" + strconv.FormatInt(sds.VariantID, 10)
	}
	if strings.TrimSpace(sds.VariantSKU) != "" {
		return ""
	}
	return strings.Join([]string{
		strings.TrimSpace(sds.VariantColor),
		strings.TrimSpace(sds.VariantSize),
	}, "-")
}

func normalizeStudioVariantDiscriminator(value string) string {
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

func studioVariantBaseSKUCounts(sds *SDSSyncOptions) map[string]int {
	counts := map[string]int{}
	if sds == nil {
		return counts
	}
	for _, item := range sds.Variants {
		key := firstNonEmptyString(item.VariantSKU, sds.VariantSKU, sds.ProductSKU)
		if strings.TrimSpace(key) == "" {
			key = "__empty__"
		}
		counts[key]++
	}
	return counts
}

func appendNonEmpty(values []string, candidates ...string) []string {
	for _, candidate := range candidates {
		if trimmed := strings.TrimSpace(candidate); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}

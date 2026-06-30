package publishing

import "strings"

// SizeReferenceVariantInput describes an SDS variant's source size-reference images.
type SizeReferenceVariantInput struct {
	VariantID              int64
	VariantSKU             string
	Color                  string
	MockupImageURLs        []string
	SizeReferenceImageURLs []string
}

// SizeReferenceVariantSummary describes rendered SDS output for a variant.
type SizeReferenceVariantSummary struct {
	VariantID       int64
	VariantSKU      string
	VariantColor    string
	MockupImageURLs []string
}

// ResolveRenderedSizeReferenceImages maps raw size-reference mockups to rendered SDS images.
func ResolveRenderedSizeReferenceImages(rawRefs []string, sourceMockups []string, renderedMockups []string) []string {
	rawRefs = uniqueNonEmptyImageStrings(rawRefs)
	sourceMockups = uniqueNonEmptyImageStrings(sourceMockups)
	renderedMockups = uniqueNonEmptyImageStrings(renderedMockups)
	if len(rawRefs) == 0 || len(sourceMockups) == 0 || len(renderedMockups) == 0 {
		return nil
	}
	sourceIndex := map[string]int{}
	for index, url := range sourceMockups {
		sourceIndex[normalizeSizeReferenceURLForMatch(url)] = index
	}
	var rendered []string
	for _, ref := range rawRefs {
		index, ok := sourceIndex[normalizeSizeReferenceURLForMatch(ref)]
		if !ok || index < 0 || index >= len(renderedMockups) {
			continue
		}
		rendered = append(rendered, renderedMockups[index])
	}
	return uniqueNonEmptyImageStrings(rendered)
}

// FindSizeReferenceVariantSummary finds the rendered SDS summary matching a variant option.
func FindSizeReferenceVariantSummary(variant SizeReferenceVariantInput, summaries []SizeReferenceVariantSummary) (SizeReferenceVariantSummary, bool) {
	for _, summary := range summaries {
		if variant.VariantID > 0 && summary.VariantID == variant.VariantID {
			return summary, true
		}
		if strings.TrimSpace(variant.VariantSKU) != "" && strings.EqualFold(strings.TrimSpace(summary.VariantSKU), strings.TrimSpace(variant.VariantSKU)) {
			return summary, true
		}
		if strings.TrimSpace(variant.Color) != "" && strings.EqualFold(strings.TrimSpace(summary.VariantColor), strings.TrimSpace(variant.Color)) {
			return summary, true
		}
	}
	return SizeReferenceVariantSummary{}, false
}

func normalizeSizeReferenceURLForMatch(value string) string {
	return strings.TrimSpace(value)
}

func uniqueNonEmptyImageStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

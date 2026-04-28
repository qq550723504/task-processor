package listingkit

import "strings"

func resolveSheinSizeReferenceImages(req *GenerateRequest, sdsSummary *SDSSyncSummary) []string {
	rawRefs := rawSheinSizeReferenceImages(req)
	if len(rawRefs) == 0 {
		return nil
	}
	if renderedRefs := resolveRenderedSDSSizeReferenceImages(req, sdsSummary, rawRefs); len(renderedRefs) > 0 {
		return renderedRefs
	}
	return rawRefs
}

func rawSheinSizeReferenceImages(req *GenerateRequest) []string {
	if req == nil || req.Options == nil {
		return nil
	}
	var refs []string
	if req.Options.SheinStudio != nil {
		refs = append(refs, req.Options.SheinStudio.SizeReferenceImageURLs...)
	}
	if req.Options.SDS != nil {
		for _, variant := range req.Options.SDS.Variants {
			refs = append(refs, variant.SizeReferenceImageURLs...)
		}
	}
	return uniqueNonEmptyStrings(refs)
}

func resolveRenderedSDSSizeReferenceImages(req *GenerateRequest, sdsSummary *SDSSyncSummary, rawRefs []string) []string {
	if req == nil || req.Options == nil || req.Options.SDS == nil || sdsSummary == nil {
		return nil
	}
	options := req.Options.SDS
	var rendered []string
	rendered = append(rendered, renderedSizeReferenceImagesFromMockups(rawRefs, options.MockupImageURLs, sdsSummary.MockupImageURLs)...)
	for _, variant := range options.Variants {
		summary, ok := findSDSVariantSummaryForSizeReference(variant, sdsSummary.VariantResults)
		if !ok {
			continue
		}
		rendered = append(rendered, renderedSizeReferenceImagesFromMockups(variant.SizeReferenceImageURLs, variant.MockupImageURLs, summary.MockupImageURLs)...)
	}
	return uniqueNonEmptyStrings(rendered)
}

func renderedSizeReferenceImagesFromMockups(sizeRefs []string, sourceMockups []string, renderedMockups []string) []string {
	sizeRefs = uniqueNonEmptyStrings(sizeRefs)
	sourceMockups = uniqueNonEmptyStrings(sourceMockups)
	renderedMockups = uniqueNonEmptyStrings(renderedMockups)
	if len(sizeRefs) == 0 || len(sourceMockups) == 0 || len(renderedMockups) == 0 {
		return nil
	}
	sourceIndex := map[string]int{}
	for index, url := range sourceMockups {
		sourceIndex[normalizeImageURLForMatch(url)] = index
	}
	var rendered []string
	for _, ref := range sizeRefs {
		index, ok := sourceIndex[normalizeImageURLForMatch(ref)]
		if !ok || index < 0 || index >= len(renderedMockups) {
			continue
		}
		rendered = append(rendered, renderedMockups[index])
	}
	return uniqueNonEmptyStrings(rendered)
}

func findSDSVariantSummaryForSizeReference(variant SDSSyncVariantOption, summaries []SDSSyncSummary) (SDSSyncSummary, bool) {
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
	return SDSSyncSummary{}, false
}

func normalizeImageURLForMatch(value string) string {
	return strings.TrimSpace(value)
}

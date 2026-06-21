package listingkit

import sheinpub "task-processor/internal/publishing/shein"

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
	return sheinpub.ResolveRenderedSizeReferenceImages(sizeRefs, sourceMockups, renderedMockups)
}

func findSDSVariantSummaryForSizeReference(variant SDSSyncVariantOption, summaries []SDSSyncSummary) (SDSSyncSummary, bool) {
	match, ok := sheinpub.FindSizeReferenceVariantSummary(
		sheinpub.SizeReferenceVariantInput{
			VariantID:  variant.VariantID,
			VariantSKU: variant.VariantSKU,
			Color:      variant.Color,
		},
		sheinSizeReferenceVariantSummaries(summaries),
	)
	if !ok {
		return SDSSyncSummary{}, false
	}
	for _, summary := range summaries {
		if summary.VariantID == match.VariantID &&
			summary.VariantSKU == match.VariantSKU &&
			summary.VariantColor == match.VariantColor {
			return summary, true
		}
	}
	return SDSSyncSummary{VariantID: match.VariantID, VariantSKU: match.VariantSKU, VariantColor: match.VariantColor, MockupImageURLs: match.MockupImageURLs}, true
}

func sheinSizeReferenceVariantSummaries(summaries []SDSSyncSummary) []sheinpub.SizeReferenceVariantSummary {
	if len(summaries) == 0 {
		return nil
	}
	out := make([]sheinpub.SizeReferenceVariantSummary, 0, len(summaries))
	for _, summary := range summaries {
		out = append(out, sheinpub.SizeReferenceVariantSummary{
			VariantID:       summary.VariantID,
			VariantSKU:      summary.VariantSKU,
			VariantColor:    summary.VariantColor,
			MockupImageURLs: append([]string(nil), summary.MockupImageURLs...),
		})
	}
	return out
}

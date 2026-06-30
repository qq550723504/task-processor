package listingkit

import (
	"fmt"
	"strings"
)

func sdsVariantIDs(variants []SDSSyncVariantOption) []int64 {
	ids := make([]int64, 0, len(variants))
	seen := map[int64]struct{}{}
	for _, variant := range variants {
		if variant.VariantID <= 0 {
			continue
		}
		if _, ok := seen[variant.VariantID]; ok {
			continue
		}
		seen[variant.VariantID] = struct{}{}
		ids = append(ids, variant.VariantID)
	}
	return ids
}

func representativeSDSVariantsByColor(variants []SDSSyncVariantOption) []SDSSyncVariantOption {
	seen := map[string]struct{}{}
	result := make([]SDSSyncVariantOption, 0, len(variants))
	for _, variant := range variants {
		key := strings.ToLower(strings.TrimSpace(variant.Color))
		if key == "" {
			key = "__default__"
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, variant)
	}
	return result
}

func mergeSDSVariantSyncSummaries(options *SDSSyncOptions, summaries []SDSSyncSummary) *SDSSyncSummary {
	merged := &SDSSyncSummary{Status: "failed", Error: "SDS did not render any selected color variants"}
	if options != nil {
		merged.VariantID = options.VariantID
	}
	var failedColors []string
	var authFailureDetail string
	var primary *SDSSyncSummary
	var renderedURLs []string
	for _, summary := range summaries {
		if summary.Status == "failed" || len(summary.MockupImageURLs) == 0 {
			if authFailureDetail == "" && isSDSAuthRequiredError(fmt.Errorf("%s", strings.TrimSpace(summary.Error))) {
				authFailureDetail = strings.TrimSpace(summary.Error)
			}
			label := strings.TrimSpace(summary.VariantColor)
			if label == "" {
				label = strings.TrimSpace(summary.VariantSKU)
			}
			if label == "" {
				label = "unknown"
			}
			failedColors = append(failedColors, label)
			continue
		}
		if primary == nil {
			copy := summary
			primary = &copy
		}
		renderedURLs = append(renderedURLs, summary.MockupImageURLs...)
	}
	if primary != nil {
		*merged = *primary
		merged.MockupImageURLs = uniqueNonEmptyStrings(renderedURLs)
		merged.VariantResults = append([]SDSSyncSummary(nil), summaries...)
	}
	if authFailureDetail != "" {
		merged.Status = "failed"
		merged.Error = sdsAuthRequiredMessage
		merged.MockupImageURLs = nil
		return merged
	}
	if len(failedColors) > 0 {
		merged.Status = "failed"
		merged.Error = "SDS render failed for selected color variants: " + strings.Join(uniqueNonEmptyStrings(failedColors), ", ")
		merged.MockupImageURLs = nil
	}
	return merged
}

func firstNonZeroInt64(values ...int64) int64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

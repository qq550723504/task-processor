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
	var failureDetails []string
	var failureSourceImages []string
	var authFailureDetail string
	var primary *SDSSyncSummary
	var renderedURLs []string
	for _, summary := range summaries {
		if summary.Status == "failed" || len(summary.MockupImageURLs) == 0 {
			errorDetail := strings.TrimSpace(summary.Error)
			if authFailureDetail == "" && isSDSAuthRequiredError(fmt.Errorf("%s", errorDetail)) {
				authFailureDetail = errorDetail
			}
			if errorDetail != "" {
				failureDetails = append(failureDetails, errorDetail)
			}
			if summary.Diagnostics != nil && strings.TrimSpace(summary.Diagnostics.MaterialImageURL) != "" {
				failureSourceImages = append(failureSourceImages, strings.TrimSpace(summary.Diagnostics.MaterialImageURL))
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
		errorParts := []string{
			"SDS render failed for selected color variants: " + strings.Join(uniqueNonEmptyStrings(failedColors), ", "),
		}
		if details := uniqueNonEmptyStrings(failureDetails); len(details) > 0 {
			errorParts = append(errorParts, "detail: "+strings.Join(details, " | "))
		}
		if sourceImages := uniqueNonEmptyStrings(failureSourceImages); len(sourceImages) > 0 {
			errorParts = append(errorParts, "source image: "+strings.Join(sourceImages, " | "))
		}
		merged.Error = strings.Join(errorParts, "; ")
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

package listingkit

import (
	"strings"

	"task-processor/internal/productenrich"
)

func applySDSSyncMetadataToCanonical(canonical *productenrich.CanonicalProduct, summary *SDSSyncSummary, options *SDSSyncOptions) bool {
	if canonical == nil {
		return false
	}
	changed := applyStudioStyleDimension(canonical, options)
	title := trustedSDSProductName(summary, options)
	if title == "" {
		return changed
	}
	if strings.TrimSpace(canonical.Title) == title {
		return changed
	}
	canonical.Title = title
	if canonical.FieldTraces == nil {
		canonical.FieldTraces = map[string]productenrich.FieldTrace{}
	}
	canonical.FieldTraces["title"] = productenrich.FieldTrace{
		Sources: []productenrich.CanonicalSource{{
			Type:   productenrich.CanonicalSourceDerived,
			Detail: "SDS design product detail",
		}},
		Confidence:  0.96,
		IsInferred:  false,
		NeedsReview: false,
	}
	return true
}

func trustedSDSProductName(summary *SDSSyncSummary, options *SDSSyncOptions) string {
	if summary != nil {
		if name := strings.TrimSpace(summary.ProductName); name != "" {
			return name
		}
	}
	if options != nil {
		if name := strings.TrimSpace(options.ProductName); name != "" {
			return name
		}
	}
	return ""
}

package listingkit

import (
	"strings"

	"task-processor/internal/catalog/canonical"
)

func applySDSSyncMetadataToCanonical(product *canonical.Product, summary *SDSSyncSummary, options *SDSSyncOptions) bool {
	if product == nil {
		return false
	}
	changed := applyStudioStyleDimension(product, options)
	title := trustedSDSProductName(summary, options)
	if title == "" {
		return changed
	}
	if strings.TrimSpace(product.Title) == title {
		return changed
	}
	product.Title = title
	if product.FieldTraces == nil {
		product.FieldTraces = map[string]canonical.FieldTrace{}
	}
	product.FieldTraces["title"] = canonical.FieldTrace{
		Sources: []canonical.Source{{
			Type:   canonical.SourceDerived,
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

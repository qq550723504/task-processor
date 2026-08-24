package listingkit

import (
	"strconv"
	"strings"
)

// normalizeGenerateRequestSource establishes the durable provenance contract at
// task creation time. Preview code must only read GenerateRequest.Source; task
// options are operational inputs and are never a provenance fallback there.
func normalizeGenerateRequestSource(req *GenerateRequest) {
	if req == nil || hasSourceReference(req.Source) {
		return
	}
	if req.Options != nil && req.Options.SDS != nil {
		sds := req.Options.SDS
		parentProductID := ""
		if sds.ParentProductID > 0 {
			parentProductID = strconv.FormatInt(sds.ParentProductID, 10)
		}
		variantID := ""
		if sds.VariantID > 0 {
			variantID = strconv.FormatInt(sds.VariantID, 10)
		}
		if parentProductID != "" || variantID != "" {
			sourceID := parentProductID
			key := "sds:" + parentProductID
			if sourceID == "" {
				sourceID = variantID
				key = "sds:variant:" + variantID
			}
			sourceURL := ""
			if parentProductID != "" {
				sourceURL = "https://www.sdsdiy.com/portal/detail/" + parentProductID
			}
			req.Source = &SourceReference{
				Key:      key,
				Type:     "sds",
				Platform: "sds",
				ID:       sourceID,
				URL:      sourceURL,
			}
			return
		}
	}
	if productURL := strings.TrimSpace(req.ProductURL); productURL != "" {
		req.Source = &SourceReference{Type: "product_url", URL: productURL}
	}
}

func hasSourceReference(source *SourceReference) bool {
	if source == nil {
		return false
	}
	return strings.TrimSpace(source.Key) != "" ||
		strings.TrimSpace(source.Type) != "" ||
		strings.TrimSpace(source.Platform) != "" ||
		strings.TrimSpace(source.ID) != "" ||
		strings.TrimSpace(source.URL) != ""
}

func isSDSSourceReference(source *SourceReference) bool {
	if source == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(source.Type), "sds") ||
		strings.EqualFold(strings.TrimSpace(source.Platform), "sds")
}

package listingkit

import (
	"context"
	"strconv"
	"strings"
	"time"

	sdstemplate "task-processor/internal/sds/template"
)

const trustedSDSProvenanceValidationTimeout = 2 * time.Second

// normalizeGenerateRequestSource establishes the durable provenance contract at
// task creation time. Preview code must only read GenerateRequest.Source; task
// options are operational inputs and are never a provenance fallback there.
func normalizeGenerateRequestSource(req *GenerateRequest) {
	if req == nil || hasSourceReference(req.Source) {
		return
	}
}

// normalizeTrustedGenerateRequestSource is called by the service creation
// boundary after the public HTTP layer has stripped caller-supplied lineage.
// SDS IDs are copied into provenance only after the configured SDS provider
// confirms that the requested parent/variant exists.
func (s *service) normalizeTrustedGenerateRequestSource(ctx context.Context, req *GenerateRequest) {
	if req == nil || hasSourceReference(req.Source) {
		return
	}
	validationCtx, cancel := context.WithTimeout(ctx, trustedSDSProvenanceValidationTimeout)
	defer cancel()
	if source := s.validatedSDSSourceReference(validationCtx, req.Options); source != nil {
		req.Source = source
		return
	}
	normalizeGenerateRequestSource(req)
}

func (s *service) validatedSDSSourceReference(ctx context.Context, options *GenerateOptions) *SourceReference {
	if s == nil || options == nil || options.SDS == nil {
		return nil
	}
	sds := options.SDS
	if sds.ParentProductID <= 0 && sds.VariantID <= 0 {
		return nil
	}
	provider := resolveSDSBaselineRemoteProvider(s)
	if provider == nil {
		return nil
	}
	var detail *sdstemplate.ProductDetail
	if sds.ParentProductID > 0 {
		var err error
		detail, err = provider.GetProductDetail(ctx, sds.ParentProductID)
		if err != nil || detail == nil || (detail.ID > 0 && detail.ID != sds.ParentProductID) {
			return nil
		}
	}
	if sds.VariantID > 0 && !validatedSDSVariantReference(ctx, provider, detail, sds.ParentProductID, sds.VariantID) {
		return nil
	}
	parentProductID := ""
	if sds.ParentProductID > 0 {
		parentProductID = strconv.FormatInt(sds.ParentProductID, 10)
	}
	sourceID := parentProductID
	key := "sds:" + parentProductID
	if sourceID == "" {
		sourceID = strconv.FormatInt(sds.VariantID, 10)
		key = "sds:variant:" + sourceID
	}
	sourceURL := ""
	if parentProductID != "" {
		sourceURL = "https://www.sdsdiy.com/portal/detail/" + parentProductID
	}
	return &SourceReference{
		Key:      key,
		Type:     "sds",
		Platform: "sds",
		ID:       sourceID,
		URL:      sourceURL,
	}
}

func validatedSDSVariantReference(ctx context.Context, provider SDSBaselineRemoteProvider, detail *sdstemplate.ProductDetail, parentProductID, variantID int64) bool {
	if detail != nil && detail.Subproducts != nil && len(detail.Subproducts.Items) > 0 {
		for _, variant := range detail.Subproducts.Items {
			if variant.ID != variantID {
				continue
			}
			return variant.ParentID <= 0 || parentProductID <= 0 || variant.ParentID == parentProductID
		}
		return false
	}
	page, err := provider.GetDesignProduct(ctx, variantID)
	if err != nil || page == nil || page.Product.ID != variantID {
		return false
	}
	resolvedParentID := page.Product.ParentID
	if resolvedParentID <= 0 {
		resolvedParentID = page.MerchantProductParentID
	}
	return parentProductID <= 0 || resolvedParentID == parentProductID
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

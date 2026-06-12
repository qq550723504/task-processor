package listingkit

import (
	"strings"

	"task-processor/internal/catalog/canonical"
	sheinworkspace "task-processor/internal/listingkit/workspace/shein"
	sheinpub "task-processor/internal/publishing/shein"
)

func buildSheinFinalReviewPayload(pkg *sheinpub.Package, canonical *canonical.Product, readiness *SheinSubmitReadiness) *SheinFinalReview {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil {
		return nil
	}
	final := &SheinFinalReview{
		SourceProduct: buildSheinSourceProductSummary(canonical),
		Title:         firstNonEmpty(pkg.ProductNameEn, pkg.SpuName),
		Description:   pkg.Description,
		CategoryPath:  append([]string(nil), pkg.CategoryPath...),
		CategoryID:    pkg.CategoryID,
		Attributes:    append([]sheinpub.ResolvedAttribute(nil), pkg.ResolvedAttributes...),
		BlockingItems: sheinworkspace.CloneReadinessItems(readiness.BlockingItems),
	}
	if pkg.FinalSubmissionDraft != nil {
		final.Confirmed = pkg.FinalSubmissionDraft.Confirmed
		final.SubmitMode = pkg.FinalSubmissionDraft.SubmitMode
	}
	if pkg.SaleAttributeResolution != nil {
		final.SaleAttributes = append(final.SaleAttributes, pkg.SaleAttributeResolution.SKCAttributes...)
		final.SaleAttributes = append(final.SaleAttributes, pkg.SaleAttributeResolution.SKUAttributes...)
	}
	if pkg.DraftPayload != nil {
		final.SKUs = buildSheinFinalReviewSKUs(pkg.DraftPayload)
		final.Images = buildSheinFinalReviewImages(pkg.DraftPayload, pkg.FinalSubmissionDraft, pkg.PreviewPayload)
	}
	return final
}

func buildSheinFinalReviewSKUs(draft *sheinpub.RequestDraft) []SheinFinalReviewSKU {
	if draft == nil {
		return nil
	}
	out := []SheinFinalReviewSKU{}
	for _, skc := range draft.SKCList {
		for _, sku := range skc.SKUList {
			out = append(out, buildSheinFinalReviewSKU(skc.SupplierCode, sku))
		}
	}
	return out
}

func buildSheinFinalReviewSKU(supplierCode string, sku SheinSKUDraft) SheinFinalReviewSKU {
	item := SheinFinalReviewSKU{
		SupplierCode: supplierCode,
		SupplierSKU:  sku.SupplierSKU,
		Price:        parseMoney(sku.BasePrice),
		Currency:     sku.Currency,
		Stock:        sku.StockCount,
		Weight:       sku.Weight,
	}
	for _, attr := range sku.SaleAttributes {
		switch normalizeSheinFinalReviewAttributeName(attr.Name) {
		case "color":
			item.Color = attr.Value
		case "size":
			item.Size = attr.Value
		}
	}
	return item
}

func normalizeSheinFinalReviewAttributeName(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "color", "颜色":
		return "color"
	case "size", "尺码", "尺寸":
		return "size"
	default:
		return ""
	}
}

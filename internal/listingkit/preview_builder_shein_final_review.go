package listingkit

import (
	sheinworkspace "task-processor/internal/marketplace/shein/workspace"
	"task-processor/internal/product/catalog/canonical"
	sheinpub "task-processor/internal/publishing/shein"
)

func buildSheinFinalReviewPayload(pkg *sheinpub.Package, canonical *canonical.Product, readiness *SheinSubmitReadiness) *SheinFinalReview {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil {
		return nil
	}
	final := &SheinFinalReview{
		SourceProduct: sheinworkspace.BuildSourceProductSummary(canonical),
		Title:         sheinDisplayTitle(pkg),
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
		final.SKUs = sheinworkspace.BuildFinalReviewSKUs(pkg.DraftPayload)
		var offeredSourceImageURLs []string
		if final.SourceProduct != nil {
			offeredSourceImageURLs = final.SourceProduct.ImageURLs
		}
		final.Images = sheinworkspace.BuildFinalReviewImages(pkg.DraftPayload, pkg.FinalSubmissionDraft, pkg.PreviewPayload, offeredSourceImageURLs)
	}
	return final
}

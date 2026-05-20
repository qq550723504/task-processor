package listingkit

import (
	"strconv"

	sheinpub "task-processor/internal/publishing/shein"
)

func (s *service) refreshSheinDerivedState(task *Task, req *ApplyRevisionRequest) {
	if s == nil || task == nil || task.Result == nil || task.Result.Shein == nil || req == nil || req.Shein == nil {
		return
	}
	if !shouldRefreshSheinDerivedState(req.Shein) {
		return
	}
	if task.Result.CanonicalProduct == nil {
		return
	}
	if task.Request != nil && task.Request.Options != nil {
		applyStudioStyleDimension(task.Result.CanonicalProduct, task.Request.Options.SDS)
	}

	buildReq := buildSheinPublishRequest(task.Request)
	if task.Result.Shein.CategoryID > 0 {
		buildReq.TargetCategoryHint = strconv.Itoa(task.Result.Shein.CategoryID)
	}
	sheinpub.RefreshDerivedState(
		buildReq,
		task.Result.CanonicalProduct,
		task.Result.ImageAssets,
		task.Result.Shein,
		s.sheinCategoryResolver,
		s.sheinAttributeResolver,
		s.sheinSaleAttributeResolver,
		s.sheinPricingPolicy,
	)
	applySheinSaleAttributeReviewOverride(task.Result.Shein, req.Shein.SaleAttributeResolution)
	normalizeSheinCategoryRefreshSaleAttributeState(task.Result.Shein)
	sheinpub.NormalizeListingCopy(task.Result.Shein, task.Result.CanonicalProduct, buildReq.Language)
	syncSheinDraftFromPackage(task.Result.Shein)
	task.Result.Shein.PreviewProduct = sheinpub.BuildPreviewProduct(task.Result.Shein)
	refreshSheinReviewState(task.Result.Shein)
}

func applySheinSaleAttributeReviewOverride(pkg *sheinpub.Package, patch *SheinSaleAttributeResolutionPatch) {
	if pkg == nil || patch == nil ||
		(patch.RecommendCategoryReview == nil && patch.CategoryReviewReason == nil) {
		return
	}
	if pkg.SaleAttributeResolution == nil {
		pkg.SaleAttributeResolution = &sheinpub.SaleAttributeResolution{}
	}
	confirmedCategoryReview := patch.RecommendCategoryReview != nil && !*patch.RecommendCategoryReview
	if patch.RecommendCategoryReview != nil {
		pkg.SaleAttributeResolution.RecommendCategoryReview = *patch.RecommendCategoryReview
		if !*patch.RecommendCategoryReview && pkg.CategoryResolution != nil {
			pkg.CategoryResolution.SuggestedCategory = nil
		}
	}
	if patch.CategoryReviewReason != nil {
		if confirmedCategoryReview {
			pkg.SaleAttributeResolution.CategoryReviewReason = ""
		} else {
			pkg.SaleAttributeResolution.CategoryReviewReason = *patch.CategoryReviewReason
		}
	} else if confirmedCategoryReview {
		pkg.SaleAttributeResolution.CategoryReviewReason = ""
	}
}

func shouldRefreshSheinDerivedState(req *SheinRevisionInput) bool {
	if req == nil || req.CategoryResolution == nil {
		return false
	}
	if req.AttributeResolution != nil {
		return false
	}
	if req.RequestDraft != nil || len(req.SKCPatches) > 0 || req.ResolvedAttributes != nil {
		return false
	}
	return true
}

func normalizeSheinCategoryRefreshSaleAttributeState(pkg *sheinpub.Package) {
	if pkg == nil || pkg.SaleAttributeResolution == nil {
		return
	}
	if sheinSaleAttributesReadyForSubmit(pkg) {
		return
	}
	if pkg.SaleAttributeResolution.Status == "" || pkg.SaleAttributeResolution.Status == "resolved" {
		pkg.SaleAttributeResolution.Status = "partial"
	}
	pkg.SaleAttributeResolution.ReviewNotes = uniqueStrings(append(
		[]string(nil),
		append(
			pkg.SaleAttributeResolution.ReviewNotes,
			"类目变更后已重新生成销售属性，但当前仍缺少真实 sale attribute value 映射，请重新确认规格。",
		)...,
	))
}

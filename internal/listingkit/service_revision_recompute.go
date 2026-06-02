package listingkit

import (
	"strconv"
	"strings"

	common "task-processor/internal/publishing/common"
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
	task.Result.Shein = sheinpub.NormalizePackageSemanticFields(task.Result.Shein)
	if task.Request != nil && task.Request.Options != nil {
		applyStudioStyleDimension(task.Result.CanonicalProduct, task.Request.Options.SDS)
	}

	buildReq := buildSheinPublishRequestForTask(task, task.Request)
	if task.Result.Shein.CategoryID > 0 {
		buildReq.TargetCategoryHint = strconv.Itoa(task.Result.Shein.CategoryID)
	}
	if req.Shein.RegenerateAttributes {
		s.refreshSheinAttributeDerivedState(task, buildReq)
		return
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
	cookieNote := strings.TrimSpace(s.resolveSheinCookieAvailabilityNote(buildReq.Context, task))
	if cookieNote == "" {
		stripSheinCookieUnavailableReviewNotes(task.Result.Shein)
	}
	applySheinSaleAttributeReviewOverride(task.Result.Shein, req.Shein.SaleAttributeResolution)
	normalizeSheinCategoryRefreshSaleAttributeState(task.Result.Shein)
	sheinpub.NormalizeListingCopy(buildReq.Context, task.Result.Shein, task.Result.CanonicalProduct, buildReq.Language)
	syncSheinDraftFromPackage(task.Result.Shein)
	preview := sheinpub.BuildPreviewProduct(task.Result.Shein)
	sheinpub.SetPreviewPayload(task.Result.Shein, preview)
	if cookieNote != "" {
		refreshSheinReviewState(task.Result.Shein, cookieNote)
		return
	}
	refreshSheinReviewState(task.Result.Shein)
}

func (s *service) refreshSheinAttributeDerivedState(task *Task, buildReq *sheinpub.BuildRequest) {
	if s == nil || task == nil || task.Result == nil || task.Result.Shein == nil || task.Result.CanonicalProduct == nil {
		return
	}
	pkg := task.Result.Shein
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	pkg.ProductAttributes = common.BuildAttributes(task.Result.CanonicalProduct.Attributes)
	if pkg.DraftPayload == nil {
		pkg.DraftPayload = &sheinpub.RequestDraft{}
	}
	if s.sheinAttributeResolver != nil {
		pkg.AttributeResolution = s.sheinAttributeResolver.Resolve(buildReq, task.Result.CanonicalProduct, pkg)
		sheinpub.ApplyAttributeResolution(pkg, pkg.AttributeResolution)
	}
	cookieNote := strings.TrimSpace(s.resolveSheinCookieAvailabilityNote(buildReq.Context, task))
	if cookieNote == "" {
		stripSheinCookieUnavailableReviewNotes(pkg)
	}
	sheinpub.NormalizeListingCopy(buildReq.Context, pkg, task.Result.CanonicalProduct, buildReq.Language)
	syncSheinDraftFromPackage(pkg)
	preview := sheinpub.BuildPreviewProduct(pkg)
	sheinpub.SetPreviewPayload(pkg, preview)
	if cookieNote != "" {
		refreshSheinReviewState(pkg, cookieNote)
		return
	}
	refreshSheinReviewState(pkg)
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
	if req == nil {
		return false
	}
	if req.RegenerateAttributes {
		return true
	}
	if req.RegenerateSaleAttributes {
		return true
	}
	if req.CategoryResolution == nil {
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
	normalizeSheinSaleAttributeState(pkg)
	if !containsReviewNote(pkg.SaleAttributeResolution.ReviewNotes, "类目变更后已重新生成销售属性，但当前仍缺少真实 sale attribute value 映射，请重新确认规格。") {
		pkg.SaleAttributeResolution.ReviewNotes = uniqueStrings(append(
			[]string(nil),
			append(
				pkg.SaleAttributeResolution.ReviewNotes,
				"类目变更后已重新生成销售属性，但当前仍缺少真实 sale attribute value 映射，请重新确认规格。",
			)...,
		))
	}
}

func containsReviewNote(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

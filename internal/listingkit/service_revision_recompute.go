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
	sheinpub.NormalizeListingCopy(task.Result.Shein, task.Result.CanonicalProduct, buildReq.Language)
	syncSheinDraftFromPackage(task.Result.Shein)
	task.Result.Shein.PreviewProduct = sheinpub.BuildPreviewProduct(task.Result.Shein)
	refreshSheinReviewState(task.Result.Shein)
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

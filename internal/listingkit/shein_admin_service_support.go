package listingkit

import (
	"context"
	"fmt"
	"strings"
	"time"

	"task-processor/internal/catalog/canonical"
	sheinpub "task-processor/internal/publishing/shein"
	sheincategory "task-processor/internal/shein/api/category"
	sheinclient "task-processor/internal/shein/client"
)

func (s *sheinAdminService) loadAdminTask(ctx context.Context, taskID string) (*Task, error) {
	if s == nil || s.repo == nil {
		return nil, ErrTaskNotFound
	}
	return s.repo.GetTask(ctx, taskID)
}

func (s *sheinAdminService) loadAdminTaskPackage(ctx context.Context, taskID string) (*Task, *sheinpub.Package, error) {
	task, err := s.loadAdminTask(ctx, taskID)
	if err != nil {
		return nil, nil, err
	}
	if task.Result == nil || task.Result.Shein == nil {
		return nil, nil, ErrTaskResultUnavailable
	}
	pkg := sheinpub.NormalizePackageSemanticFields(task.Result.Shein)
	task.Result.Shein = pkg
	return task, pkg, nil
}

func (s *sheinAdminService) newSheinCategoryAPI(ctx context.Context, task *Task) (*sheincategory.Client, error) {
	apiClient, storeID, err := s.newSheinAPIClient(ctx, task)
	if err != nil {
		return nil, fmt.Errorf("%w for category search", err)
	}
	if !apiClient.HasCookies() {
		if err := apiClient.ForceRefreshCookies(); err != nil {
			return nil, fmt.Errorf("shein store cookies are unavailable for category search: %w", err)
		}
	}
	if !apiClient.HasCookies() {
		return nil, fmt.Errorf("shein store cookies are unavailable for category search")
	}

	baseAPI := sheinclient.NewBaseAPIClient(
		apiClient.GetBaseURL(),
		apiClient.GetTenantID(),
		storeID,
		apiClient.GetHTTPClient(),
	)
	baseAPI.SetAuthRefreshFunc(apiClient.ForceRefreshCookies)
	return sheincategory.NewClient(baseAPI), nil
}

func applySheinAdminPricingReview(task *Task, review *sheinpub.PricingReview) error {
	if task == nil || task.Result == nil || task.Result.Shein == nil {
		return ErrTaskResultUnavailable
	}
	applySheinPricingReview(task.Result.Shein, review)
	task.Result.UpdatedAt = time.Now()
	return nil
}

func (s *sheinAdminService) applySheinFinalDraftUpdate(task *Task, req *SheinFinalDraftUpdateRequest) error {
	if task == nil || task.Result == nil || task.Result.Shein == nil {
		return ErrTaskResultUnavailable
	}
	pkg := sheinpub.NormalizePackageSemanticFields(task.Result.Shein)
	task.Result.Shein = pkg
	if pkg.FinalSubmissionDraft == nil {
		pkg.FinalSubmissionDraft = &sheinpub.FinalDraft{}
	}
	applySheinFinalDraftRequest(pkg, req)

	now := time.Now()
	pkg.FinalSubmissionDraft.UpdatedAt = &now
	rule := s.currentPricingRule()
	if pkg.Pricing != nil && pkg.Pricing.RuleSnapshot != nil {
		rule = *pkg.Pricing.RuleSnapshot
	}
	review := buildSheinDraftBackedPricingReview(pkg, rule, pkg.FinalSubmissionDraft.ManualPriceOverrides)
	applySheinPricingReview(pkg, review)
	applySheinFinalImageDraft(pkg)
	applySheinVariantImageCoverageGuard(task.Result, task.Request, pkg)
	task.Result.UpdatedAt = now
	return nil
}

func applySheinFinalDraftRequest(pkg *sheinpub.Package, req *SheinFinalDraftUpdateRequest) {
	if pkg == nil || pkg.FinalSubmissionDraft == nil || req == nil {
		return
	}
	if req.SubmitMode != "" {
		mode := strings.ToLower(strings.TrimSpace(req.SubmitMode))
		if mode == "publish" || mode == "save_draft" {
			pkg.FinalSubmissionDraft.SubmitMode = mode
		}
	}
	if len(req.ManualPriceOverrides) > 0 {
		pkg.FinalSubmissionDraft.ManualPriceOverrides = clonePriceOverrides(req.ManualPriceOverrides)
	}
	if req.FinalImageOrder != nil {
		pkg.FinalSubmissionDraft.FinalImageOrder = uniqueNonEmptyStrings(*req.FinalImageOrder)
	}
	if value := strings.TrimSpace(req.MainImageURL); value != "" {
		pkg.FinalSubmissionDraft.MainImageURL = value
	}
	if req.DeletedImageURLs != nil {
		pkg.FinalSubmissionDraft.DeletedImageURLs = uniqueNonEmptyStrings(*req.DeletedImageURLs)
	}
	if len(req.ImageRoleOverrides) > 0 {
		pkg.FinalSubmissionDraft.ImageRoleOverrides = normalizeImageRoleOverrides(req.ImageRoleOverrides)
	}
	if req.Confirmed != nil {
		pkg.FinalSubmissionDraft.Confirmed = *req.Confirmed
		if *req.Confirmed {
			now := time.Now()
			pkg.FinalSubmissionDraft.ConfirmedAt = &now
		} else {
			pkg.FinalSubmissionDraft.ConfirmedAt = nil
		}
	}
}

func (s *sheinAdminService) clearSheinAdminResolutionKinds(kind string, buildReq *sheinpub.BuildRequest, canonicalProduct *canonical.Product, pkg *sheinpub.Package) ([]string, error) {
	deletedKinds := make([]string, 0, 3)

	if kind == "all" || kind == sheinpub.ResolutionCacheKindCategory {
		if cache, ok := s.categoryResolver.(sheinpub.CategoryResolutionCache); ok {
			if err := cache.ClearCategoryResolution(buildReq, canonicalProduct, pkg); err != nil {
				return nil, err
			}
			deletedKinds = append(deletedKinds, sheinpub.ResolutionCacheKindCategory)
		}
	}
	if kind == "all" || kind == sheinpub.ResolutionCacheKindAttribute {
		if cache, ok := s.attributeResolver.(sheinpub.AttributeResolutionCache); ok {
			if err := cache.ClearAttributeResolution(buildReq, canonicalProduct, pkg); err != nil {
				return nil, err
			}
			deletedKinds = append(deletedKinds, sheinpub.ResolutionCacheKindAttribute)
		}
	}
	if kind == "all" || kind == sheinpub.ResolutionCacheKindSaleAttribute {
		if cache, ok := s.saleAttributeResolver.(sheinpub.SaleAttributeResolutionCache); ok {
			if err := cache.ClearSaleAttributeResolution(buildReq, canonicalProduct, pkg); err != nil {
				return nil, err
			}
			deletedKinds = append(deletedKinds, sheinpub.ResolutionCacheKindSaleAttribute)
		}
	}
	if kind == "all" || kind == sheinpub.ResolutionCacheKindPricing {
		if err := s.clearPricingCache(buildReq, pkg); err != nil {
			return nil, err
		}
		deletedKinds = append(deletedKinds, sheinpub.ResolutionCacheKindPricing)
	}

	return deletedKinds, nil
}

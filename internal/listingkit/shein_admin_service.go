package listingkit

import (
	"context"
	"fmt"
	"strings"
	"time"

	sheinpub "task-processor/internal/publishing/shein"
	sheincategory "task-processor/internal/shein/api/category"
	sheinclient "task-processor/internal/shein/client"
)

type sheinAdminServiceConfig struct {
	repo                  Repository
	mutateTaskResult      func(context.Context, string, TaskResultMutation) (*Task, error)
	currentPricingRule    func() sheinpub.PricingRule
	newSheinAPIClient     func(context.Context, *Task) (*sheinclient.APIClient, int64, error)
	buildTaskPreview      func(context.Context, *Task, string) (*ListingKitPreview, error)
	categoryResolver      sheinpub.CategoryResolver
	attributeResolver     sheinpub.AttributeResolver
	saleAttributeResolver sheinpub.SaleAttributeResolver
	clearPricingCache     func(*sheinpub.BuildRequest, *sheinpub.Package) error
}

type sheinAdminService struct {
	repo                  Repository
	mutateTaskResult      func(context.Context, string, TaskResultMutation) (*Task, error)
	currentPricingRule    func() sheinpub.PricingRule
	newSheinAPIClient     func(context.Context, *Task) (*sheinclient.APIClient, int64, error)
	buildTaskPreview      func(context.Context, *Task, string) (*ListingKitPreview, error)
	categoryResolver      sheinpub.CategoryResolver
	attributeResolver     sheinpub.AttributeResolver
	saleAttributeResolver sheinpub.SaleAttributeResolver
	clearPricingCache     func(*sheinpub.BuildRequest, *sheinpub.Package) error
}

func newSheinAdminService(config sheinAdminServiceConfig) *sheinAdminService {
	return &sheinAdminService{
		repo:                  config.repo,
		mutateTaskResult:      config.mutateTaskResult,
		currentPricingRule:    config.currentPricingRule,
		newSheinAPIClient:     config.newSheinAPIClient,
		buildTaskPreview:      config.buildTaskPreview,
		categoryResolver:      config.categoryResolver,
		attributeResolver:     config.attributeResolver,
		saleAttributeResolver: config.saleAttributeResolver,
		clearPricingCache:     config.clearPricingCache,
	}
}

func (s *service) sheinAdminOrDefault() *sheinAdminService {
	if s.sheinAdmin != nil {
		return s.sheinAdmin
	}
	s.sheinAdmin = newSheinAdminService(sheinAdminServiceConfig{
		repo:                  s.repo,
		mutateTaskResult:      s.mutateTaskResult,
		currentPricingRule:    s.currentSheinPricingRule,
		newSheinAPIClient:     s.newSheinAPIClient,
		buildTaskPreview:      s.buildTaskPreview,
		categoryResolver:      s.sheinCategoryResolver,
		attributeResolver:     s.sheinAttributeResolver,
		saleAttributeResolver: s.sheinSaleAttributeResolver,
		clearPricingCache:     s.clearSheinPricingCache,
	})
	return s.sheinAdmin
}

func (s *sheinAdminService) PreviewSheinPrice(ctx context.Context, taskID string, req *SheinPricePreviewRequest) (*sheinpub.PricingReview, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task.Result == nil || task.Result.Shein == nil {
		return nil, ErrTaskResultUnavailable
	}
	rule := s.currentPricingRule()
	overrides := map[string]float64{}
	task.Result.Shein = sheinpub.NormalizePackageSemanticFields(task.Result.Shein)
	if task.Result.Shein.FinalSubmissionDraft != nil {
		for sku, price := range task.Result.Shein.FinalSubmissionDraft.ManualPriceOverrides {
			overrides[sku] = price
		}
	}
	applyToTask := false
	if req != nil {
		if req.Rule != nil {
			rule = normalizeSheinPricingRule(*req.Rule, rule)
		}
		for sku, price := range req.ManualOverrides {
			if strings.TrimSpace(sku) != "" && price > 0 {
				overrides[sku] = price
			}
		}
		applyToTask = req.ApplyToTask
	}
	review := buildSheinPricingReview(task.Result.Shein, rule, overrides)
	if applyToTask {
		task, err = s.mutateTaskResult(ctx, taskID, func(task *Task) error {
			if task.Result == nil || task.Result.Shein == nil {
				return ErrTaskResultUnavailable
			}
			applySheinPricingReview(task.Result.Shein, review)
			task.Result.UpdatedAt = time.Now()
			return nil
		})
		if err != nil {
			return nil, err
		}
		_ = task
	}
	return review, nil
}

func (s *sheinAdminService) SearchSheinCategories(ctx context.Context, taskID string, query string) (*SheinCategorySearchResult, error) {
	trimmedQuery := strings.TrimSpace(query)
	if trimmedQuery == "" {
		return nil, ErrInvalidSheinCategorySearchQuery
	}

	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}

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
	categoryAPI := sheincategory.NewClient(baseAPI)
	tree, err := categoryAPI.GetCategoryTree()
	if err != nil {
		return nil, err
	}

	return &SheinCategorySearchResult{
		TaskID: taskID,
		Query:  trimmedQuery,
		Items:  searchSheinCategoryCandidates(tree.Data, trimmedQuery),
	}, nil
}

func (s *sheinAdminService) GetSubmissionEvents(ctx context.Context, taskID string) (*SheinSubmissionEventPage, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task.Result == nil || task.Result.Shein == nil {
		return nil, ErrTaskResultUnavailable
	}
	return &SheinSubmissionEventPage{
		TaskID: taskID,
		Items:  sheinSubmissionEventsWithStoreResolution(task.Result.Shein.SubmissionEvents, task),
	}, nil
}

func (s *sheinAdminService) UpdateSheinFinalDraft(ctx context.Context, taskID string, req *SheinFinalDraftUpdateRequest) (*ListingKitPreview, error) {
	task, err := s.mutateTaskResult(ctx, taskID, func(task *Task) error {
		if task.Result == nil || task.Result.Shein == nil {
			return ErrTaskResultUnavailable
		}
		pkg := sheinpub.NormalizePackageSemanticFields(task.Result.Shein)
		if pkg.FinalSubmissionDraft == nil {
			pkg.FinalSubmissionDraft = &sheinpub.FinalDraft{}
		}
		if req != nil {
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
	})
	if err != nil {
		return nil, err
	}
	return s.buildTaskPreview(ctx, task, "shein")
}

func (s *sheinAdminService) ClearSheinResolutionCache(ctx context.Context, taskID string, kind string) (*SheinResolutionCacheClearResult, error) {
	if s == nil {
		return nil, ErrTaskNotFound
	}
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task.Result == nil || task.Result.Shein == nil {
		return nil, ErrTaskResultUnavailable
	}

	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "" {
		kind = "all"
	}
	if kind != "all" && kind != sheinpub.ResolutionCacheKindCategory && kind != sheinpub.ResolutionCacheKindAttribute && kind != sheinpub.ResolutionCacheKindSaleAttribute && kind != sheinpub.ResolutionCacheKindPricing {
		return nil, ErrInvalidSheinResolutionCacheKind
	}

	buildReq := buildSheinPublishRequest(task.Request)
	pkg := sheinpub.NormalizePackageSemanticFields(task.Result.Shein)
	canonical := task.Result.CanonicalProduct
	deletedKinds := make([]string, 0, 3)

	if kind == "all" || kind == sheinpub.ResolutionCacheKindCategory {
		if cache, ok := s.categoryResolver.(sheinpub.CategoryResolutionCache); ok {
			if err := cache.ClearCategoryResolution(buildReq, canonical, pkg); err != nil {
				return nil, err
			}
			deletedKinds = append(deletedKinds, sheinpub.ResolutionCacheKindCategory)
		}
	}
	if kind == "all" || kind == sheinpub.ResolutionCacheKindAttribute {
		if cache, ok := s.attributeResolver.(sheinpub.AttributeResolutionCache); ok {
			if err := cache.ClearAttributeResolution(buildReq, canonical, pkg); err != nil {
				return nil, err
			}
			deletedKinds = append(deletedKinds, sheinpub.ResolutionCacheKindAttribute)
		}
	}
	if kind == "all" || kind == sheinpub.ResolutionCacheKindSaleAttribute {
		if cache, ok := s.saleAttributeResolver.(sheinpub.SaleAttributeResolutionCache); ok {
			if err := cache.ClearSaleAttributeResolution(buildReq, canonical, pkg); err != nil {
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

	return &SheinResolutionCacheClearResult{
		TaskID:       taskID,
		Kind:         kind,
		DeletedKinds: deletedKinds,
	}, nil
}

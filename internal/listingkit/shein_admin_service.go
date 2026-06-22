package listingkit

import (
	"context"
	"fmt"
	"strings"
	"time"

	"task-processor/internal/catalog/canonical"
	sheinworkspace "task-processor/internal/marketplace/shein/workspace"
	sheinpub "task-processor/internal/publishing/shein"
	sheincategory "task-processor/internal/shein/api/category"
	sheinclient "task-processor/internal/shein/client"
)

type sheinAdminServiceConfig struct {
	repo                  Repository
	recovery              *taskSubmissionRecoveryService
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
	recovery              *taskSubmissionRecoveryService
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
		recovery:              config.recovery,
		currentPricingRule:    config.currentPricingRule,
		newSheinAPIClient:     config.newSheinAPIClient,
		buildTaskPreview:      config.buildTaskPreview,
		categoryResolver:      config.categoryResolver,
		attributeResolver:     config.attributeResolver,
		saleAttributeResolver: config.saleAttributeResolver,
		clearPricingCache:     config.clearPricingCache,
	}
}

func (s *sheinAdminService) PreviewSheinPrice(ctx context.Context, taskID string, req *SheinPricePreviewRequest) (*sheinpub.PricingReview, error) {
	_, pkg, err := s.loadAdminTaskPackage(ctx, taskID)
	if err != nil {
		return nil, err
	}
	rule := s.currentPricingRule()
	overrides := map[string]float64{}
	if pkg.FinalSubmissionDraft != nil {
		for sku, price := range pkg.FinalSubmissionDraft.ManualPriceOverrides {
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
	review := buildSheinPricingReview(pkg, rule, overrides)
	if applyToTask {
		if _, err := s.mutateAdminTask(ctx, taskID, func(task *Task) error {
			return applySheinAdminPricingReview(task, review)
		}); err != nil {
			return nil, err
		}
	}
	return review, nil
}

func (s *sheinAdminService) SearchSheinCategories(ctx context.Context, taskID string, query string) (*SheinCategorySearchResult, error) {
	trimmedQuery := strings.TrimSpace(query)
	if trimmedQuery == "" {
		return nil, ErrInvalidSheinCategorySearchQuery
	}

	task, err := s.loadAdminTask(ctx, taskID)
	if err != nil {
		return nil, err
	}

	categoryAPI, err := s.newSheinCategoryAPI(ctx, task)
	if err != nil {
		return nil, err
	}
	tree, err := categoryAPI.GetCategoryTree()
	if err != nil {
		return nil, err
	}

	return &SheinCategorySearchResult{
		TaskID: taskID,
		Query:  trimmedQuery,
		Items:  buildSheinCategorySearchCandidates(sheinworkspace.SearchCategoryCandidates(tree.Data, trimmedQuery)),
	}, nil
}

func buildSheinCategorySearchCandidates(items []sheinworkspace.CategorySearchCandidate) []SheinCategorySearchCandidate {
	if len(items) == 0 {
		return nil
	}
	result := make([]SheinCategorySearchCandidate, 0, len(items))
	for _, item := range items {
		result = append(result, SheinCategorySearchCandidate{
			CategoryID:     item.CategoryID,
			CategoryIDList: append([]int(nil), item.CategoryIDList...),
			CategoryPath:   append([]string(nil), item.CategoryPath...),
			ProductTypeID:  item.ProductTypeID,
			TopCategoryID:  item.TopCategoryID,
			Source:         item.Source,
			MatchReason:    item.MatchReason,
		})
	}
	return result
}

func (s *sheinAdminService) GetSubmissionEvents(ctx context.Context, taskID string) (*SheinSubmissionEventPage, error) {
	task, pkg, err := s.loadAdminTaskPackage(ctx, taskID)
	if err != nil {
		return nil, err
	}
	return &SheinSubmissionEventPage{
		TaskID: taskID,
		Items:  sheinSubmissionEventsWithStoreResolution(pkg.SubmissionEvents, task),
	}, nil
}

func (s *sheinAdminService) UpdateSheinFinalDraft(ctx context.Context, taskID string, req *SheinFinalDraftUpdateRequest) (*ListingKitPreview, error) {
	task, err := s.mutateAdminTask(ctx, taskID, func(task *Task) error {
		return s.applySheinFinalDraftUpdate(task, req)
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
	task, pkg, err := s.loadAdminTaskPackage(ctx, taskID)
	if err != nil {
		return nil, err
	}

	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "" {
		kind = "all"
	}
	if kind != "all" && kind != sheinpub.ResolutionCacheKindCategory && kind != sheinpub.ResolutionCacheKindAttribute && kind != sheinpub.ResolutionCacheKindSaleAttribute && kind != sheinpub.ResolutionCacheKindPricing {
		return nil, ErrInvalidSheinResolutionCacheKind
	}

	buildReq := buildSheinPublishRequest(task.Request)
	deletedKinds, err := s.clearSheinAdminResolutionKinds(kind, buildReq, task.Result.CanonicalProduct, pkg)
	if err != nil {
		return nil, err
	}

	return &SheinResolutionCacheClearResult{
		TaskID:       taskID,
		Kind:         kind,
		DeletedKinds: deletedKinds,
	}, nil
}

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
	sheinpub.ApplyFinalImageDraft(pkg)
	applySheinVariantImageCoverageGuard(task.Result, task.Request, pkg)
	task.Result.UpdatedAt = now
	return nil
}

func applySheinFinalDraftRequest(pkg *sheinpub.Package, req *SheinFinalDraftUpdateRequest) {
	if pkg == nil || req == nil {
		return
	}
	sheinpub.ApplyFinalDraftUpdate(pkg, sheinpub.FinalDraftUpdate{
		Confirmed:            req.Confirmed,
		SubmitMode:           req.SubmitMode,
		ManualPriceOverrides: req.ManualPriceOverrides,
		FinalImageOrder:      req.FinalImageOrder,
		MainImageURL:         req.MainImageURL,
		DeletedImageURLs:     req.DeletedImageURLs,
		ImageRoleOverrides:   req.ImageRoleOverrides,
	}, time.Now())
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

func (s *sheinAdminService) mutateAdminTask(ctx context.Context, taskID string, mutate TaskResultMutation) (*Task, error) {
	if s.recovery == nil {
		return nil, ErrTaskResultUnavailable
	}
	return s.recovery.mutateTaskResult(ctx, taskID, mutate)
}

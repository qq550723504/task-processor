package listingkit

import (
	"context"
	"strings"

	sheinworkspace "task-processor/internal/marketplace/shein/workspace"
	sheinpub "task-processor/internal/publishing/shein"
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

func (s *sheinAdminService) mutateAdminTask(ctx context.Context, taskID string, mutate TaskResultMutation) (*Task, error) {
	if s.recovery == nil {
		return nil, ErrTaskResultUnavailable
	}
	return s.recovery.mutateTaskResult(ctx, taskID, mutate)
}

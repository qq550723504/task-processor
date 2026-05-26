package listingkit

import (
	"context"
	"strings"
	"time"

	sheinpub "task-processor/internal/publishing/shein"
)

type sheinAdminServiceConfig struct {
	repo                  Repository
	mutateTaskResult      func(context.Context, string, TaskResultMutation) (*Task, error)
	currentPricingRule    func() sheinpub.PricingRule
	categoryResolver      sheinpub.CategoryResolver
	attributeResolver     sheinpub.AttributeResolver
	saleAttributeResolver sheinpub.SaleAttributeResolver
	clearPricingCache     func(*sheinpub.BuildRequest, *sheinpub.Package) error
}

type sheinAdminService struct {
	repo                  Repository
	mutateTaskResult      func(context.Context, string, TaskResultMutation) (*Task, error)
	currentPricingRule    func() sheinpub.PricingRule
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
	if task.Result.Shein.FinalDraft != nil {
		for sku, price := range task.Result.Shein.FinalDraft.ManualPriceOverrides {
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
	pkg := task.Result.Shein
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

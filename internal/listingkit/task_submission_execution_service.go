package listingkit

import (
	"context"
	"fmt"

	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

type taskSubmissionExecutionServiceConfig struct {
	sheinProductAPIBuilder   sheinpub.ProductAPIBuilder
	sheinImageAPIBuilder     sheinpub.ImageAPIBuilder
	sheinTranslateAPIBuilder sheinpub.TranslateAPIBuilder
	sheinContentOptimizer    AIChatCompleter
	currentSheinPricingRule  func() sheinpub.PricingRule
	resolveSheinStoreID      func(context.Context, *Task) (int64, error)
	resolveSubmitSettings    func(context.Context, *Task) SheinSettings
}

type taskSubmissionExecutionService struct {
	sheinProductAPIBuilder   sheinpub.ProductAPIBuilder
	sheinImageAPIBuilder     sheinpub.ImageAPIBuilder
	sheinTranslateAPIBuilder sheinpub.TranslateAPIBuilder
	sheinContentOptimizer    AIChatCompleter
	currentSheinPricingRule  func() sheinpub.PricingRule
	resolveSheinStoreID      func(context.Context, *Task) (int64, error)
	resolveSubmitSettings    func(context.Context, *Task) SheinSettings
}

func newTaskSubmissionExecutionService(config taskSubmissionExecutionServiceConfig) *taskSubmissionExecutionService {
	return &taskSubmissionExecutionService{
		sheinProductAPIBuilder:   config.sheinProductAPIBuilder,
		sheinImageAPIBuilder:     config.sheinImageAPIBuilder,
		sheinTranslateAPIBuilder: config.sheinTranslateAPIBuilder,
		sheinContentOptimizer:    config.sheinContentOptimizer,
		currentSheinPricingRule:  config.currentSheinPricingRule,
		resolveSheinStoreID:      config.resolveSheinStoreID,
		resolveSubmitSettings:    config.resolveSubmitSettings,
	}
}

func (s *taskSubmissionExecutionService) buildSheinSubmitProductAPI(ctx context.Context, task *Task) (sheinproduct.ProductAPI, error) {
	if s.sheinProductAPIBuilder == nil {
		return nil, fmt.Errorf("shein product api builder is not configured")
	}
	runtimeCtx, storeID, err := s.resolveSheinStoreRuntime(ctx, task, "submit")
	if err != nil {
		return nil, err
	}
	return s.buildSheinSubmitProductAPIForStore(runtimeCtx, storeID)
}

func (s *taskSubmissionExecutionService) resolveSheinStoreRuntime(ctx context.Context, task *Task, action string) (context.Context, int64, error) {
	runtimeCtx, err := withSheinSubmitTaskIdentity(ctx, task)
	if err != nil {
		return nil, 0, err
	}
	storeID, err := s.resolveSheinStoreID(runtimeCtx, task)
	if err != nil || storeID <= 0 {
		return nil, 0, fmt.Errorf("shein store id is unavailable for %s", action)
	}
	return runtimeCtx, storeID, nil
}

func (s *taskSubmissionExecutionService) buildSheinSubmitProductAPIForStore(ctx context.Context, storeID int64) (sheinproduct.ProductAPI, error) {
	productAPI, fallback := s.sheinProductAPIBuilder.BuildProductAPI(ctx, storeID)
	if productAPI == nil {
		return nil, fmt.Errorf("shein submit unavailable: %s", fallback)
	}
	return productAPI, nil
}

func (s *taskSubmissionExecutionService) sheinPricingRule() sheinpub.PricingRule {
	if s == nil || s.currentSheinPricingRule == nil {
		return sheinpub.PricingRule{}
	}
	return s.currentSheinPricingRule()
}

package listingkit

import (
	"context"
	"fmt"
	"strings"
	"time"

	openaiclient "task-processor/internal/infra/clients/openai"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
	sheintranslateapi "task-processor/internal/shein/api/translate"
)

type taskSubmissionExecutionServiceConfig struct {
	sheinProductAPIBuilder   sheinpub.ProductAPIBuilder
	sheinImageAPIBuilder     sheinpub.ImageAPIBuilder
	sheinTranslateAPIBuilder sheinpub.TranslateAPIBuilder
	sheinContentOptimizer    openaiclient.ChatCompleter
	currentSheinPricingRule  func() sheinpub.PricingRule
	resolveSheinStoreID      func(context.Context, *Task) (int64, error)
	resolveSubmitSettings    func(context.Context, *Task) SheinSettings
}

type taskSubmissionExecutionService struct {
	sheinProductAPIBuilder   sheinpub.ProductAPIBuilder
	sheinImageAPIBuilder     sheinpub.ImageAPIBuilder
	sheinTranslateAPIBuilder sheinpub.TranslateAPIBuilder
	sheinContentOptimizer    openaiclient.ChatCompleter
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

var defaultTaskSubmissionExecutionService = newTaskSubmissionExecutionService(taskSubmissionExecutionServiceConfig{})

func (s *taskSubmissionExecutionService) normalizeSheinSubmitPackage(task *Task, pkg *SheinPackage, req *SubmitTaskRequest, action string) {
	normalizeSheinStudioSubmitSupplierSKUs(task, pkg, normalizedSubmitIdempotencyKey(req))
	if pkg.Pricing == nil || !pkg.Pricing.Ready {
		review := buildSheinDraftBackedPricingReview(pkg, s.currentSheinPricingRule(), nil)
		applySheinPricingReview(pkg, review)
	} else {
		applySheinPricingReview(pkg, pkg.Pricing)
	}
	if req != nil && req.ConfirmedFinal {
		if pkg.FinalDraft == nil {
			pkg.FinalDraft = &sheinpub.FinalDraft{}
		}
		now := time.Now()
		pkg.FinalDraft.Confirmed = true
		pkg.FinalDraft.ConfirmedAt = &now
		pkg.FinalDraft.UpdatedAt = &now
		if pkg.FinalDraft.SubmitMode == "" {
			pkg.FinalDraft.SubmitMode = action
		}
	}
	repairSheinSubmitSaleAttributes(pkg)
	applySheinFinalImageDraft(pkg)
	applySheinVariantImageCoverageGuard(task.Result, task.Request, pkg)
}

func (s *taskSubmissionExecutionService) buildSheinSubmitProductAPI(ctx context.Context, task *Task) (sheinproduct.ProductAPI, error) {
	if s.sheinProductAPIBuilder == nil {
		return nil, fmt.Errorf("shein product api builder is not configured")
	}
	runtimeCtx, err := withSheinSubmitTaskIdentity(ctx, task)
	if err != nil {
		return nil, err
	}
	storeID, err := s.resolveSheinStoreID(runtimeCtx, task)
	if err != nil || storeID <= 0 {
		return nil, fmt.Errorf("shein store id is unavailable for submit")
	}
	productAPI, fallback := s.sheinProductAPIBuilder.BuildProductAPI(runtimeCtx, storeID)
	if productAPI == nil {
		return nil, fmt.Errorf("shein submit unavailable: %s", fallback)
	}
	return productAPI, nil
}

func (s *taskSubmissionExecutionService) prepareSheinSubmitProduct(ctx context.Context, task *Task, pkg *SheinPackage, action string) (*sheinproduct.Product, error) {
	runtimeCtx, err := withSheinSubmitTaskIdentity(ctx, task)
	if err != nil {
		return nil, err
	}
	submitProduct, err := cloneSheinProductForSubmit(pkg.PreviewProduct)
	if err != nil {
		return nil, err
	}
	if attrs := sheinpub.BuildProductAttributes(pkg); sheinProductAttributesReadyForSubmit(attrs) {
		submitProduct.ProductAttributeList = attrs
	}
	var translateAPI sheintranslateapi.TranslateAPI
	if sheinpub.SubmitProductNeedsTranslation(submitProduct) || sheinpub.SubmitProductNeedsTargetLanguages(submitProduct, task.Request.Country) {
		if s.sheinTranslateAPIBuilder != nil {
			var fallback string
			storeID, resolveErr := s.resolveSheinStoreID(runtimeCtx, task)
			if resolveErr == nil && storeID > 0 {
				translateAPI, fallback = s.sheinTranslateAPIBuilder.BuildTranslateAPI(runtimeCtx, storeID)
			}
			if translateAPI == nil && strings.TrimSpace(fallback) != "" {
				translateAPI = nil
			}
		}
	}
	if err := sheinpub.PrepareSubmitProductContent(runtimeCtx, submitProduct, task.Request.Country, s.sheinContentOptimizer, translateAPI); err != nil {
		return nil, err
	}
	prepareSheinProductForSubmit(submitProduct, s.resolveSubmitSettings(ctx, task))
	if action == "publish" {
		if err := validateSheinProductPublishPayload(submitProduct); err != nil {
			return nil, err
		}
	}
	return submitProduct, nil
}

func (s *taskSubmissionExecutionService) uploadSheinSubmitImages(ctx context.Context, task *Task, pkg *SheinPackage, submitProduct *sheinproduct.Product) error {
	if s.sheinImageAPIBuilder == nil {
		return fmt.Errorf("shein image upload api builder is not configured")
	}
	runtimeCtx, err := withSheinSubmitTaskIdentity(ctx, task)
	if err != nil {
		return err
	}
	storeID, err := s.resolveSheinStoreID(runtimeCtx, task)
	if err != nil || storeID <= 0 {
		return fmt.Errorf("shein store id is unavailable for image upload")
	}
	imageAPI, fallback := s.sheinImageAPIBuilder.BuildImageAPI(runtimeCtx, storeID)
	if imageAPI == nil {
		return fmt.Errorf("shein image upload unavailable: %s", fallback)
	}
	_, uploadCache, err := uploadSheinProductImages(submitProduct, imageAPI, sheinImageUploadCache(pkg))
	if err != nil {
		return err
	}
	if len(uploadCache) > 0 {
		if pkg.FinalDraft == nil {
			pkg.FinalDraft = &sheinpub.FinalDraft{}
		}
		pkg.FinalDraft.SheinImageUploadCache = uploadCache
		now := time.Now()
		pkg.FinalDraft.UpdatedAt = &now
	}
	return nil
}

func (s *taskSubmissionExecutionService) preValidateSheinSubmitProduct(submitProduct *sheinproduct.Product) error {
	return sheinpub.PreValidateSubmitProduct(submitProduct)
}

func (s *taskSubmissionExecutionService) executeSheinSubmitRemote(productAPI sheinproduct.ProductAPI, action string, submitProduct *sheinproduct.Product) (*sheinpub.SubmissionResponse, error) {
	switch action {
	case "save_draft":
		raw, _, err := productAPI.SaveDraftProduct(submitProduct)
		return sheinpub.BuildSubmissionResponseSummary(raw), err
	case "publish":
		raw, _, err := productAPI.PublishProduct(submitProduct)
		return sheinpub.BuildSubmissionResponseSummary(raw), err
	default:
		return nil, unsupportedSubmitActionError(action)
	}
}

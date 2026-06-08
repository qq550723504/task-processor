package listingkit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	openaiclient "task-processor/internal/infra/clients/openai"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
	sheintranslateapi "task-processor/internal/shein/api/translate"

	"github.com/sirupsen/logrus"
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

func (s *taskSubmissionExecutionService) normalizeSheinSubmitPackage(task *Task, pkg *SheinPackage, req *SubmitTaskRequest, action string) {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	normalizeSheinStudioSubmitSupplierSKUs(task, pkg, normalizedSubmitIdempotencyKey(req))
	if pkg.Pricing == nil || !pkg.Pricing.Ready {
		review := buildSheinDraftBackedPricingReview(pkg, s.currentSheinPricingRule(), nil)
		applySheinPricingReview(pkg, review)
	} else {
		applySheinPricingReview(pkg, pkg.Pricing)
	}
	if req != nil && req.ConfirmedFinal {
		if pkg.FinalSubmissionDraft == nil {
			pkg.FinalSubmissionDraft = &sheinpub.FinalDraft{}
		}
		now := time.Now()
		pkg.FinalSubmissionDraft.Confirmed = true
		pkg.FinalSubmissionDraft.ConfirmedAt = &now
		pkg.FinalSubmissionDraft.UpdatedAt = &now
		if pkg.FinalSubmissionDraft.SubmitMode == "" {
			pkg.FinalSubmissionDraft.SubmitMode = action
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
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	submitProduct, err := cloneSheinProductForSubmit(pkg.PreviewPayload)
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
		pkg = sheinpub.NormalizePackageSemanticFields(pkg)
		if pkg.FinalSubmissionDraft == nil {
			pkg.FinalSubmissionDraft = &sheinpub.FinalDraft{}
		}
		pkg.FinalSubmissionDraft.SheinImageUploadCache = uploadCache
		now := time.Now()
		pkg.FinalSubmissionDraft.UpdatedAt = &now
	}
	return nil
}

func (s *taskSubmissionExecutionService) preValidateSheinSubmitProduct(pkg *SheinPackage, submitProduct *sheinproduct.Product) error {
	return sheinpub.PreValidateSubmitProductWithOptions(submitProduct, !sheinSecondarySaleAttributeRequired(pkg))
}

func (s *taskSubmissionExecutionService) executeSheinSubmitRemote(productAPI sheinproduct.ProductAPI, action string, submitProduct *sheinproduct.Product) (*sheinpub.SubmissionResponse, error) {
	switch action {
	case "save_draft":
		raw, _, err := productAPI.SaveDraftProduct(submitProduct)
		logSheinSubmitRemoteResponse(action, submitProduct, raw, err)
		return sheinpub.BuildSubmissionResponseSummary(raw), err
	case "publish":
		raw, _, err := productAPI.PublishProduct(submitProduct)
		logSheinSubmitRemoteResponse(action, submitProduct, raw, err)
		return sheinpub.BuildSubmissionResponseSummary(raw), err
	default:
		return nil, unsupportedSubmitActionError(action)
	}
}

func logSheinSubmitRemoteResponse(action string, submitProduct *sheinproduct.Product, raw *sheinproduct.SheinResponse, err error) {
	fields := logrus.Fields{
		"action":        action,
		"supplier_code": "",
		"spu_name":      "",
	}
	if submitProduct != nil {
		fields["supplier_code"] = strings.TrimSpace(submitProduct.SupplierCode)
		fields["spu_name"] = strings.TrimSpace(submitProduct.SPUName)
	}
	if raw != nil {
		fields["response_code"] = strings.TrimSpace(raw.Code)
		fields["response_msg"] = strings.TrimSpace(raw.Msg)
		if encoded, marshalErr := json.Marshal(raw); marshalErr == nil {
			fields["response_json"] = string(encoded)
		} else {
			fields["response_json_error"] = marshalErr.Error()
		}
	}
	if err != nil {
		fields["error"] = err.Error()
		logrus.WithFields(fields).Warn("listingkit shein submit remote completed with error")
		return
	}
	logrus.WithFields(fields).Info("listingkit shein submit remote response")
}

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
	applyConfirmedFinalSubmissionDraft(pkg, req, action)
	repairSheinSubmitSaleAttributes(pkg)
	applySheinFinalImageDraft(pkg)
	applySheinVariantImageCoverageGuard(task.Result, task.Request, pkg)
}

func (s *taskSubmissionExecutionService) buildSheinSubmitProductAPI(ctx context.Context, task *Task) (sheinproduct.ProductAPI, error) {
	if s.sheinProductAPIBuilder == nil {
		return nil, fmt.Errorf("shein product api builder is not configured")
	}
	runtimeCtx, storeID, err := s.resolveSheinSubmitRuntime(ctx, task)
	if err != nil {
		return nil, err
	}
	return s.buildSheinSubmitProductAPIForStore(runtimeCtx, storeID)
}

func applyConfirmedFinalSubmissionDraft(pkg *SheinPackage, req *SubmitTaskRequest, action string) {
	if pkg == nil || req == nil || !req.ConfirmedFinal {
		return
	}
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

func (s *taskSubmissionExecutionService) resolveSheinSubmitRuntime(ctx context.Context, task *Task) (context.Context, int64, error) {
	runtimeCtx, err := withSheinSubmitTaskIdentity(ctx, task)
	if err != nil {
		return nil, 0, err
	}
	storeID, err := s.resolveSheinStoreID(runtimeCtx, task)
	if err != nil || storeID <= 0 {
		return nil, 0, fmt.Errorf("shein store id is unavailable for submit")
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

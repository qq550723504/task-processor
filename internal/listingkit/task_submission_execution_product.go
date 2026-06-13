package listingkit

import (
	"context"
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
	sheintranslateapi "task-processor/internal/shein/api/translate"
)

func (s *taskSubmissionExecutionService) prepareSheinSubmitProduct(ctx context.Context, task *Task, pkg *SheinPackage, action string) (*sheinproduct.Product, error) {
	runtimeCtx, err := s.resolveSheinSubmitContext(ctx, task)
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
	translateAPI := s.buildSheinSubmitTranslateAPI(runtimeCtx, task, submitProduct)
	if err := sheinpub.PrepareSubmitProductContent(runtimeCtx, submitProduct, task.Request.Country, s.sheinContentOptimizer, translateAPI); err != nil {
		return nil, err
	}
	prepareSheinProductForSubmit(submitProduct, s.resolveSubmitSettings(runtimeCtx, task))
	if action == "publish" {
		if err := validateSheinProductPublishPayload(submitProduct); err != nil {
			return nil, err
		}
	}
	return submitProduct, nil
}

func (s *taskSubmissionExecutionService) buildSheinSubmitTranslateAPI(ctx context.Context, task *Task, submitProduct *sheinproduct.Product) sheintranslateapi.TranslateAPI {
	if !s.sheinSubmitTranslationNeeded(task, submitProduct) || s.sheinTranslateAPIBuilder == nil {
		return nil
	}
	storeID, err := s.resolveSheinStoreID(ctx, task)
	if err != nil || storeID <= 0 {
		return nil
	}
	return s.buildSheinSubmitTranslateAPIForStore(ctx, storeID)
}

func (s *taskSubmissionExecutionService) buildSheinSubmitTranslateAPIForStore(ctx context.Context, storeID int64) sheintranslateapi.TranslateAPI {
	translateAPI, fallback := s.sheinTranslateAPIBuilder.BuildTranslateAPI(ctx, storeID)
	if translateAPI == nil && strings.TrimSpace(fallback) != "" {
		return nil
	}
	return translateAPI
}

func (s *taskSubmissionExecutionService) sheinSubmitTranslationNeeded(task *Task, submitProduct *sheinproduct.Product) bool {
	if submitProduct == nil {
		return false
	}
	region := ""
	if task != nil && task.Request != nil {
		region = task.Request.Country
	}
	return sheinpub.SubmitProductNeedsTranslation(submitProduct) || sheinpub.SubmitProductNeedsTargetLanguages(submitProduct, region)
}

func (s *taskSubmissionExecutionService) preValidateSheinSubmitProduct(pkg *SheinPackage, submitProduct *sheinproduct.Product) error {
	return sheinpub.PreValidateSubmitProductWithOptions(submitProduct, !sheinSecondarySaleAttributeRequired(pkg))
}

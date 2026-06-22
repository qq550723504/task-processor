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
	submitProduct, err := sheinpub.CloneProductForSubmit(pkg.PreviewPayload)
	if err != nil {
		return nil, err
	}
	if attrs := sheinpub.BuildProductAttributes(pkg); sheinpub.ProductAttributesReadyForSubmit(attrs) {
		submitProduct.ProductAttributeList = attrs
	}
	translateAPI := s.buildSheinSubmitTranslateAPI(runtimeCtx, task, submitProduct)
	if err := sheinpub.PrepareSubmitProductContent(runtimeCtx, submitProduct, task.Request.Country, translateAPI); err != nil {
		return nil, err
	}
	sheinpub.PrepareProductForSubmit(submitProduct, sheinSubmitPayloadSettings(s.resolveSubmitSettings(runtimeCtx, task)))
	if action == "publish" {
		if err := sheinpub.ValidateProductPublishPayload(submitProduct); err != nil {
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
	region := ""
	if task != nil && task.Request != nil {
		region = task.Request.Country
	}
	return sheinpub.SubmitProductTranslationNeeded(submitProduct, region)
}

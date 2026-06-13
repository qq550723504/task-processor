package listingkit

import (
	"context"
	"fmt"
	"time"

	sheinpub "task-processor/internal/publishing/shein"
	sheinimage "task-processor/internal/shein/api/image"
	sheinproduct "task-processor/internal/shein/api/product"
)

func (s *taskSubmissionExecutionService) uploadSheinSubmitImages(ctx context.Context, task *Task, pkg *SheinPackage, submitProduct *sheinproduct.Product) error {
	if s.sheinImageAPIBuilder == nil {
		return fmt.Errorf("shein image upload api builder is not configured")
	}
	runtimeCtx, storeID, err := s.resolveSheinImageUploadRuntime(ctx, task)
	if err != nil {
		return err
	}
	imageAPI, err := s.buildSheinImageUploadAPI(runtimeCtx, storeID)
	if err != nil {
		return err
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

func (s *taskSubmissionExecutionService) resolveSheinImageUploadRuntime(ctx context.Context, task *Task) (context.Context, int64, error) {
	return s.resolveSheinStoreRuntime(ctx, task, "image upload")
}

func (s *taskSubmissionExecutionService) buildSheinImageUploadAPI(ctx context.Context, storeID int64) (sheinimage.ImageAPI, error) {
	imageAPI, fallback := s.sheinImageAPIBuilder.BuildImageAPI(ctx, storeID)
	if imageAPI == nil {
		return nil, fmt.Errorf("shein image upload unavailable: %s", fallback)
	}
	return imageAPI, nil
}

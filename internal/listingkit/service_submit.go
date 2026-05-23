package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	listingsubmission "task-processor/internal/listingkit/submission"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
	sheintranslateapi "task-processor/internal/shein/api/translate"
)

const sheinSubmitInFlightTTL = listingsubmission.InFlightTTL

var (
	errSheinSubmitReplayExisting = errors.New("shein submit replay existing")
	errSheinSubmitRecoverRemote  = errors.New("shein submit recover remote")
	errSheinSubmitMissingPackage = errors.New("shein submit missing package")
)

func (s *service) SubmitTask(ctx context.Context, taskID string, req *SubmitTaskRequest) (*ListingKitPreview, error) {
	platform, action, err := normalizeSubmitTarget(req)
	if err != nil {
		return nil, err
	}

	startedAt := time.Now()
	requestID := normalizedSubmitIdempotencyKey(req)
	useWorkflow := s.shouldStartSheinPublishWorkflow(platform, action)
	if useWorkflow && requestID == "" {
		requestID = derivedSheinSubmitRequestID(taskID, action, startedAt)
	}
	unlockSubmit := s.sheinSubmitLocks.lock(taskID + ":" + action)
	defer unlockSubmit()
	task, preview, err := s.acquireSheinSubmitTask(ctx, taskID, action, requestID, startedAt)
	if preview != nil || err != nil {
		return preview, err
	}
	if useWorkflow {
		return s.submitSheinTaskWithWorkflow(ctx, taskID, task, req, sheinWorkflowSubmitOptions{
			platform:  platform,
			action:    action,
			requestID: requestID,
			startedAt: startedAt,
		})
	}
	return s.submitSheinTaskDirect(ctx, taskID, task, req, sheinDirectSubmitOptions{
		action:    action,
		requestID: requestID,
		startedAt: startedAt,
	})
}

type sheinWorkflowSubmitOptions struct {
	platform  string
	action    string
	requestID string
	startedAt time.Time
}

type sheinDirectSubmitOptions struct {
	action    string
	requestID string
	startedAt time.Time
}

func normalizeSubmitTarget(req *SubmitTaskRequest) (platform string, action string, err error) {
	platform = "shein"
	action = "publish"
	if req != nil {
		if value := strings.ToLower(strings.TrimSpace(req.Platform)); value != "" {
			platform = value
		}
		if value := strings.ToLower(strings.TrimSpace(req.Action)); value != "" {
			action = value
		}
	}
	if platform != "shein" {
		return "", "", fmt.Errorf("%w: %s", ErrUnsupportedSubmitPlatform, platform)
	}
	if !isSupportedSubmitAction(action) {
		return "", "", unsupportedSubmitActionError(action)
	}
	return platform, action, nil
}

func (s *service) acquireSheinSubmitTask(ctx context.Context, taskID, action, requestID string, startedAt time.Time) (*Task, *ListingKitPreview, error) {
	task, err := s.beginSheinSubmitLease(ctx, taskID, action, requestID, startedAt)
	if errors.Is(err, errSheinSubmitReplayExisting) {
		preview, previewErr := s.buildTaskPreview(ctx, task, "shein")
		return nil, preview, previewErr
	}
	if errors.Is(err, errSheinSubmitRecoverRemote) {
		preview, previewErr := s.recoverSheinSubmitRemote(ctx, task, action)
		return nil, preview, previewErr
	}
	if errors.Is(err, errSheinSubmitMissingPackage) {
		return nil, nil, fmt.Errorf("%w: shein preview_product is not available", ErrSubmitBlocked)
	}
	if err != nil {
		return nil, nil, err
	}
	return task, nil, nil
}

func (s *service) shouldStartSheinPublishWorkflow(platform, action string) bool {
	return s != nil &&
		s.sheinPublishWorkflowEnabled &&
		s.sheinPublishWorkflowClient != nil &&
		platform == "shein" &&
		action == "publish"
}

func sheinProductAttributesReadyForSubmit(attrs []sheinproduct.ProductAttribute) bool {
	if len(attrs) == 0 {
		return false
	}
	for _, attr := range attrs {
		if attr.AttributeID <= 0 {
			return false
		}
		if attr.AttributeValueID == nil && strings.TrimSpace(attr.AttributeExtraValue) == "" {
			return false
		}
	}
	return true
}

func normalizedSubmitIdempotencyKey(req *SubmitTaskRequest) string {
	if req == nil {
		return ""
	}
	if value := strings.TrimSpace(req.IdempotencyKey); value != "" {
		return value
	}
	return strings.TrimSpace(req.RequestID)
}

func derivedSheinSubmitRequestID(taskID, action string, requestedAt time.Time) string {
	taskID = strings.TrimSpace(taskID)
	action = strings.ToLower(strings.TrimSpace(action))
	if action == "" {
		action = "publish"
	}
	timestamp := requestedAt.UTC().Format("20060102T150405.000000000Z")
	if taskID == "" {
		taskID = "unknown-task"
	}
	return fmt.Sprintf("temporal:%s:%s:%s", taskID, action, timestamp)
}

func shouldReplayStartedTemporalSubmit(err error, requestID string) bool {
	var inProgress *SubmitInProgressError
	return errors.As(err, &inProgress) &&
		inProgress != nil &&
		strings.TrimSpace(inProgress.RequestID) != "" &&
		inProgress.RequestID == strings.TrimSpace(requestID)
}

func (s *service) normalizeSheinSubmitPackage(task *Task, pkg *SheinPackage, req *SubmitTaskRequest, action string) {
	normalizeSheinStudioSubmitSupplierSKUs(task, pkg, normalizedSubmitIdempotencyKey(req))
	if pkg.Pricing == nil || !pkg.Pricing.Ready {
		review := buildSheinDraftBackedPricingReview(pkg, s.currentSheinPricingRule(), nil)
		applySheinPricingReview(pkg, review)
	} else {
		// Submit clones PreviewProduct, so ensure any persisted ready pricing is
		// reapplied after SKU normalization and before submit payload generation.
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

func repairSheinSubmitSaleAttributes(pkg *SheinPackage) {
	if !sheinSubmitSaleAttributesNeedRepair(pkg) {
		return
	}
	sheinpub.ApplySaleAttributeResolution(pkg, pkg.SaleAttributeResolution)
	pkg.PreviewProduct = sheinpub.BuildPreviewProduct(pkg)
}

func sheinSubmitSaleAttributesNeedRepair(pkg *SheinPackage) bool {
	if pkg == nil || pkg.RequestDraft == nil || pkg.SaleAttributeResolution == nil {
		return false
	}
	if strings.TrimSpace(pkg.SaleAttributeResolution.Status) != "resolved" {
		return false
	}
	if len(pkg.RequestDraft.SKCList) == 0 {
		return false
	}
	if len(pkg.SaleAttributeResolution.SKCAttributes) == 0 && len(pkg.SaleAttributeResolution.SKUAttributes) == 0 {
		return false
	}
	for _, skc := range pkg.RequestDraft.SKCList {
		if skc.SaleAttribute == nil || skc.SaleAttribute.AttributeID <= 0 || skc.SaleAttribute.AttributeValueID == nil || *skc.SaleAttribute.AttributeValueID <= 0 {
			return true
		}
		for _, sku := range skc.SKUList {
			if len(pkg.SaleAttributeResolution.SKUAttributes) == 0 {
				continue
			}
			if len(sku.SaleAttributes) == 0 {
				return true
			}
			for _, attr := range sku.SaleAttributes {
				if attr.AttributeID <= 0 || attr.AttributeValueID == nil || *attr.AttributeValueID <= 0 {
					return true
				}
			}
		}
	}
	return false
}

func (s *service) buildSheinSubmitProductAPI(ctx context.Context, task *Task) (sheinproduct.ProductAPI, error) {
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

func (s *service) prepareSheinSubmitProduct(ctx context.Context, task *Task, pkg *SheinPackage, action string) (*sheinproduct.Product, error) {
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
	prepareSheinProductForSubmit(submitProduct, s.resolveSheinSubmitSettings(ctx, task))
	if action == "publish" {
		if err := validateSheinProductPublishPayload(submitProduct); err != nil {
			return nil, err
		}
	}
	return submitProduct, nil
}

func (s *service) uploadSheinSubmitImages(ctx context.Context, task *Task, pkg *SheinPackage, submitProduct *sheinproduct.Product) error {
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

func preValidateSheinSubmitProduct(submitProduct *sheinproduct.Product) error {
	return sheinpub.PreValidateSubmitProduct(submitProduct)
}

func executeSheinSubmitRemote(productAPI sheinproduct.ProductAPI, action string, submitProduct *sheinproduct.Product) (*sheinpub.SubmissionResponse, error) {
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

func isSupportedSubmitAction(action string) bool {
	return action == "publish" || action == "save_draft"
}

func unsupportedSubmitActionError(action string) error {
	return fmt.Errorf("unsupported submit action: %s", action)
}

func sheinSubmitSupplierCode(product *sheinproduct.Product, pkg *SheinPackage) string {
	if product != nil {
		if value := strings.TrimSpace(product.SupplierCode); value != "" {
			return value
		}
		for i := range product.SKCList {
			if product.SKCList[i].SupplierCode == nil {
				continue
			}
			if value := strings.TrimSpace(*product.SKCList[i].SupplierCode); value != "" {
				return value
			}
		}
	}
	if pkg != nil {
		for _, skc := range pkg.SkcList {
			if value := strings.TrimSpace(skc.SupplierCode); value != "" {
				return value
			}
		}
	}
	if product != nil && strings.TrimSpace(product.SPUName) != "" {
		return strings.TrimSpace(product.SPUName)
	}
	return ""
}

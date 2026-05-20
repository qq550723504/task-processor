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
	platform := "shein"
	action := "publish"
	if req != nil {
		if value := strings.ToLower(strings.TrimSpace(req.Platform)); value != "" {
			platform = value
		}
		if value := strings.ToLower(strings.TrimSpace(req.Action)); value != "" {
			action = value
		}
	}
	if platform != "shein" {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedSubmitPlatform, platform)
	}
	if action != "publish" && action != "save_draft" {
		return nil, fmt.Errorf("unsupported submit action: %s", action)
	}

	startedAt := time.Now()
	requestID := normalizedSubmitIdempotencyKey(req)
	if s.shouldStartSheinPublishWorkflow(platform, action) && requestID == "" {
		requestID = derivedSheinSubmitRequestID(taskID, action, startedAt)
	}
	unlockSubmit := s.sheinSubmitLocks.lock(taskID + ":" + action)
	defer unlockSubmit()
	if s.shouldStartSheinPublishWorkflow(platform, action) {
		task, err := s.beginSheinSubmitLease(ctx, taskID, action, requestID, startedAt)
		if errors.Is(err, errSheinSubmitReplayExisting) {
			return buildListingKitPreview(task, "shein")
		}
		if errors.Is(err, errSheinSubmitRecoverRemote) {
			return s.recoverSheinSubmitRemote(ctx, task, action)
		}
		if errors.Is(err, errSheinSubmitMissingPackage) {
			return nil, fmt.Errorf("%w: shein preview_product is not available", ErrSubmitBlocked)
		}
		if err != nil {
			return nil, err
		}
		if err := s.sheinPublishWorkflowClient.StartSheinPublish(ctx, SheinPublishWorkflowStartInput{
			TaskID:         strings.TrimSpace(taskID),
			Platform:       platform,
			Action:         action,
			RequestID:      requestID,
			ConfirmedFinal: req != nil && req.ConfirmedFinal,
			RequestedAt:    startedAt,
		}); err != nil {
			if shouldReplayStartedTemporalSubmit(err, requestID) {
				return buildListingKitPreview(task, "shein")
			}
			failErr := s.recordSheinSubmissionFailureForState(
				ctx,
				taskID,
				task.Result,
				task.Result.Shein,
				action,
				requestID,
				sheinpub.SubmissionPhaseValidate,
				err,
			)
			clearErr := s.clearSheinSubmitLeaseAfterStartFailure(ctx, taskID, action, requestID, err)
			if failErr != nil {
				if clearErr != nil {
					return nil, errors.Join(failErr, clearErr)
				}
				return nil, failErr
			}
			if clearErr != nil {
				return nil, clearErr
			}
			return nil, err
		}
		return s.GetTaskPreview(ctx, taskID, "shein")
	}

	task, err := s.beginSheinSubmitLease(ctx, taskID, action, requestID, startedAt)
	if errors.Is(err, errSheinSubmitReplayExisting) {
		return buildListingKitPreview(task, "shein")
	}
	if errors.Is(err, errSheinSubmitRecoverRemote) {
		return s.recoverSheinSubmitRemote(ctx, task, action)
	}
	if errors.Is(err, errSheinSubmitMissingPackage) {
		return nil, fmt.Errorf("%w: shein preview_product is not available", ErrSubmitBlocked)
	}
	if err != nil {
		return nil, err
	}
	pkg := task.Result.Shein
	s.normalizeSheinSubmitPackage(task, pkg, req, action)

	readiness := buildSheinSubmitReadinessForAction(pkg, action)
	if readiness == nil || !readiness.Ready {
		err := fmt.Errorf("%w: %s", ErrSubmitBlocked, firstSubmitReadinessMessage(readiness))
		if saveErr := s.recordSheinSubmissionFailure(ctx, taskID, task.Result, pkg, action, err); saveErr != nil {
			return nil, saveErr
		}
		return nil, err
	}

	productAPI, err := s.buildSheinSubmitProductAPI(ctx, task)
	if err != nil {
		if saveErr := s.recordSheinSubmissionFailure(ctx, taskID, task.Result, pkg, action, err); saveErr != nil {
			return nil, saveErr
		}
		return nil, err
	}

	if err := s.persistSheinSubmitPhase(ctx, taskID, task.Result, pkg, action, requestID, sheinpub.SubmissionPhasePrepareProduct); err != nil {
		return nil, err
	}
	submitProduct, err := s.prepareSheinSubmitProduct(ctx, task, pkg, action)
	if err != nil {
		if saveErr := s.recordSheinSubmissionFailure(ctx, taskID, task.Result, pkg, action, err); saveErr != nil {
			return nil, saveErr
		}
		return nil, err
	}
	dumpSheinSubmitPayloadForDebug(taskID, action, requestID, "prepared", submitProduct)
	setSheinSubmitSnapshot(pkg, action, requestID, sheinpub.BuildSubmitSnapshot(submitProduct))
	if sheinProductPendingImageUploadCount(submitProduct) > 0 {
		if err := s.persistSheinSubmitPhase(ctx, taskID, task.Result, pkg, action, requestID, sheinpub.SubmissionPhaseUploadImages); err != nil {
			return nil, err
		}
		if err := s.uploadSheinSubmitImages(task, pkg, submitProduct); err != nil {
			if saveErr := s.recordSheinSubmissionFailure(ctx, taskID, task.Result, pkg, action, err); saveErr != nil {
				return nil, saveErr
			}
			return nil, err
		}
		prepareSheinProductForSubmit(submitProduct, s.resolveSheinSubmitSettings(ctx, task))
		dumpSheinSubmitPayloadForDebug(taskID, action, requestID, "uploaded", submitProduct)
	}

	if err := s.persistSheinSubmitPhase(ctx, taskID, task.Result, pkg, action, requestID, sheinpub.SubmissionPhasePreValidate); err != nil {
		return nil, err
	}
	if err := preValidateSheinSubmitProduct(submitProduct); err != nil {
		if saveErr := s.recordSheinSubmissionFailure(ctx, taskID, task.Result, pkg, action, err); saveErr != nil {
			return nil, saveErr
		}
		return nil, err
	}

	supplierCode := sheinSubmitSupplierCode(submitProduct, pkg)
	setSheinSubmitSupplierCode(pkg, action, requestID, supplierCode)
	setSheinSubmitSnapshot(pkg, action, requestID, sheinpub.BuildSubmitSnapshot(submitProduct))
	if err := s.persistSheinSubmitPhase(ctx, taskID, task.Result, pkg, action, requestID, sheinpub.SubmissionPhaseSubmitRemote); err != nil {
		return nil, err
	}
	response, responseErr := executeSheinSubmitRemote(productAPI, action, submitProduct)
	if responseErr == nil {
		responseErr = buildSheinSubmitResponseError(action, response)
	}
	if retryResponse, retryErr, retried := s.retrySheinSensitiveWordSubmit(ctx, taskID, pkg, action, requestID, productAPI, submitProduct, response, responseErr); retried {
		response = retryResponse
		responseErr = retryErr
		setSheinSubmitSnapshot(pkg, action, requestID, sheinpub.BuildSubmitSnapshot(submitProduct))
	}

	if responseErr == nil {
		setSheinSubmitRemoteResponse(pkg, action, requestID, supplierCode, response)
		task.Result.UpdatedAt = time.Now()
		if err := s.repo.SaveTaskResult(ctx, taskID, task.Result); err != nil {
			return nil, err
		}
		if err := s.persistSheinSubmitPhase(ctx, taskID, task.Result, pkg, action, requestID, sheinpub.SubmissionPhasePersistResult); err != nil {
			return nil, err
		}
	}
	record := completeSheinSubmitAttempt(pkg, action, requestID, response, responseErr, time.Now())
	appendSheinSubmissionEvent(pkg, buildSheinSubmissionEvent(taskID, action, record, response, responseErr, startedAt))
	if responseErr == nil {
		s.rememberSheinSubmittedResolution(task, action)
	}
	task.Result.UpdatedAt = time.Now()
	if err := s.repo.SaveTaskResult(ctx, taskID, task.Result); err != nil {
		return nil, err
	}
	if responseErr != nil {
		return nil, responseErr
	}
	return buildListingKitPreview(task, "shein")
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
	storeID, err := s.resolveSheinStoreID(ctx, task)
	if err != nil || storeID <= 0 {
		return nil, fmt.Errorf("shein store id is unavailable for submit")
	}
	productAPI, fallback := s.sheinProductAPIBuilder.BuildProductAPI(storeID)
	if productAPI == nil {
		return nil, fmt.Errorf("shein submit unavailable: %s", fallback)
	}
	return productAPI, nil
}

func (s *service) prepareSheinSubmitProduct(ctx context.Context, task *Task, pkg *SheinPackage, action string) (*sheinproduct.Product, error) {
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
			storeID, resolveErr := s.resolveSheinStoreID(ctx, task)
			if resolveErr == nil && storeID > 0 {
				translateAPI, fallback = s.sheinTranslateAPIBuilder.BuildTranslateAPI(storeID)
			}
			if translateAPI == nil && strings.TrimSpace(fallback) != "" {
				translateAPI = nil
			}
		}
	}
	if err := sheinpub.PrepareSubmitProductContent(ctx, submitProduct, task.Request.Country, s.sheinContentOptimizer, translateAPI); err != nil {
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

func (s *service) uploadSheinSubmitImages(task *Task, pkg *SheinPackage, submitProduct *sheinproduct.Product) error {
	if s.sheinImageAPIBuilder == nil {
		return fmt.Errorf("shein image upload api builder is not configured")
	}
	storeID, err := s.resolveSheinStoreID(context.Background(), task)
	if err != nil || storeID <= 0 {
		return fmt.Errorf("shein store id is unavailable for image upload")
	}
	imageAPI, fallback := s.sheinImageAPIBuilder.BuildImageAPI(storeID)
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
		return nil, fmt.Errorf("unsupported submit action: %s", action)
	}
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

func (s *service) persistSheinSubmitPhase(ctx context.Context, taskID string, result *ListingKitResult, pkg *SheinPackage, action, requestID, phase string) error {
	advanceSheinSubmitPhase(pkg, action, requestID, phase)
	appendSheinSubmissionEvent(pkg, buildSheinPhaseSubmissionEvent(taskID, action, phase, sheinpub.SubmissionStatusRunning, requestID, time.Now(), "", nil))
	if result == nil {
		return nil
	}
	result.UpdatedAt = time.Now()
	return s.repo.SaveTaskResult(ctx, taskID, result)
}

func (s *service) recordSheinSubmissionFailure(ctx context.Context, taskID string, result *ListingKitResult, pkg *SheinPackage, action string, submitErr error) error {
	return s.recordSheinSubmissionFailureForState(ctx, taskID, result, pkg, action, "", "", submitErr)
}

func (s *service) recordSheinSubmissionFailureForState(ctx context.Context, taskID string, result *ListingKitResult, pkg *SheinPackage, action, requestedID, phase string, submitErr error) error {
	requestID := strings.TrimSpace(requestedID)
	phase = strings.TrimSpace(phase)
	if phase == "" {
		phase = sheinpub.SubmissionPhaseValidate
	}
	if pkg != nil && pkg.Submission != nil {
		if requestID == "" {
			requestID = pkg.Submission.CurrentRequestID
		}
		if phase == sheinpub.SubmissionPhaseValidate && pkg.Submission.CurrentPhase != "" {
			phase = pkg.Submission.CurrentPhase
		}
	}
	record := failSheinSubmitAttempt(pkg, action, requestID, phase, submitErr, time.Now())
	startedAt := record.SubmittedAt
	if !record.StartedAt.IsZero() {
		startedAt = record.StartedAt
	}
	appendSheinSubmissionEvent(pkg, buildSheinSubmissionEvent(taskID, action, record, nil, submitErr, startedAt))
	result.UpdatedAt = time.Now()
	return s.repo.SaveTaskResult(ctx, taskID, result)
}

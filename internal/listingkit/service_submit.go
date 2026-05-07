package listingkit

import (
	"context"
	"fmt"
	"strings"
	"time"

	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
	sheintranslateapi "task-processor/internal/shein/api/translate"
	sheinpublish "task-processor/internal/shein/publish"
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
	unlockSubmit := s.sheinSubmitLocks.lock(taskID + ":" + action)
	defer unlockSubmit()

	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task.Result == nil {
		return nil, ErrTaskResultUnavailable
	}
	startedAt := time.Now()
	requestID := normalizedSubmitIdempotencyKey(req)

	pkg := task.Result.Shein
	if pkg == nil || pkg.PreviewProduct == nil {
		return nil, fmt.Errorf("%w: shein preview_product is not available", ErrSubmitBlocked)
	}
	if findSheinSubmissionRecordByRequestID(pkg, action, requestID) != nil {
		return buildListingKitPreview(task, "shein")
	}
	beginSheinSubmitAttempt(pkg, action, requestID, sheinpub.SubmissionPhaseValidate, startedAt)
	if err := s.persistSheinSubmitPhase(ctx, taskID, task.Result, pkg, action, requestID, sheinpub.SubmissionPhaseValidate); err != nil {
		return nil, err
	}
	s.normalizeSheinSubmitPackage(task, pkg, req, action)

	readiness := buildSheinSubmitReadinessForAction(pkg, action)
	if readiness == nil || !readiness.Ready {
		err := fmt.Errorf("%w: %s", ErrSubmitBlocked, firstSubmitReadinessMessage(readiness))
		if saveErr := s.recordSheinSubmissionFailure(ctx, taskID, task.Result, pkg, action, err); saveErr != nil {
			return nil, saveErr
		}
		return nil, err
	}

	productAPI, err := s.buildSheinSubmitProductAPI(task)
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

	if err := s.persistSheinSubmitPhase(ctx, taskID, task.Result, pkg, action, requestID, sheinpub.SubmissionPhaseSubmitRemote); err != nil {
		return nil, err
	}
	response, responseErr := executeSheinSubmitRemote(productAPI, action, submitProduct)
	if responseErr == nil {
		responseErr = buildSheinSubmitResponseError(action, response)
	}

	if responseErr == nil {
		if err := s.persistSheinSubmitPhase(ctx, taskID, task.Result, pkg, action, requestID, sheinpub.SubmissionPhasePersistResult); err != nil {
			return nil, err
		}
	}
	record := completeSheinSubmitAttempt(pkg, action, requestID, response, responseErr, time.Now())
	appendSheinSubmissionEvent(pkg, buildSheinSubmissionEvent(taskID, action, record, response, responseErr, startedAt))
	task.Result.UpdatedAt = time.Now()
	if err := s.repo.SaveTaskResult(ctx, taskID, task.Result); err != nil {
		return nil, err
	}
	if responseErr != nil {
		return nil, responseErr
	}
	return buildListingKitPreview(task, "shein")
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
	return strings.TrimSpace(req.IdempotencyKey)
}

func (s *service) normalizeSheinSubmitPackage(task *Task, pkg *SheinPackage, req *SubmitTaskRequest, action string) {
	normalizeSheinStudioSubmitSupplierSKUs(task, pkg)
	if pkg.Pricing == nil || !pkg.Pricing.Ready {
		review := buildSheinPricingReview(pkg, s.currentSheinPricingRule(), nil)
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
	applySheinFinalImageDraft(pkg)
	applySheinVariantImageCoverageGuard(task, pkg)
}

func (s *service) buildSheinSubmitProductAPI(task *Task) (sheinproduct.ProductAPI, error) {
	if s.sheinProductAPIBuilder == nil {
		return nil, fmt.Errorf("shein product api builder is not configured")
	}
	productAPI, fallback := s.sheinProductAPIBuilder.BuildProductAPI(task.Request.SheinStoreID)
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
	if err := optimizeSheinProductContentForSubmit(ctx, submitProduct, s.sheinContentOptimizer); err != nil {
		return nil, err
	}
	var translateAPI sheintranslateapi.TranslateAPI
	if sheinProductNeedsContentTranslation(submitProduct) {
		if s.sheinTranslateAPIBuilder == nil {
			return nil, fmt.Errorf("shein translate api builder is not configured")
		}
		var fallback string
		translateAPI, fallback = s.sheinTranslateAPIBuilder.BuildTranslateAPI(task.Request.SheinStoreID)
		if translateAPI == nil {
			return nil, fmt.Errorf("shein translate unavailable: %s", fallback)
		}
	}
	if err := translateSheinProductContentForSubmit(submitProduct, translateAPI, task.Request.Country); err != nil {
		return nil, err
	}
	prepareSheinProductForNewSubmit(submitProduct)
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
	imageAPI, fallback := s.sheinImageAPIBuilder.BuildImageAPI(task.Request.SheinStoreID)
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
	validator := sheinpublish.NewPublishProductValidator()
	return validator.PreValidateProductData(nil, &sheinpublish.ValidationInput{
		ProductData: submitProduct,
	})
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

func (s *service) persistSheinSubmitPhase(ctx context.Context, taskID string, result *ListingKitResult, pkg *SheinPackage, action, requestID, phase string) error {
	advanceSheinSubmitPhase(pkg, action, requestID, phase)
	if result == nil {
		return nil
	}
	result.UpdatedAt = time.Now()
	return s.repo.SaveTaskResult(ctx, taskID, result)
}

func (s *service) recordSheinSubmissionFailure(ctx context.Context, taskID string, result *ListingKitResult, pkg *SheinPackage, action string, submitErr error) error {
	requestID := ""
	phase := sheinpub.SubmissionPhaseValidate
	if pkg != nil && pkg.Submission != nil {
		requestID = pkg.Submission.CurrentRequestID
		if pkg.Submission.CurrentPhase != "" {
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

func buildSheinSubmissionRecord(action string, result *sheinpub.SubmissionResponse, submitErr error) *sheinpub.SubmissionRecord {
	record := &sheinpub.SubmissionRecord{
		Action:      action,
		SubmittedAt: time.Now(),
		Result:      result,
	}
	if submitErr != nil {
		record.Status = "failed"
		record.Error = submitErr.Error()
		return record
	}
	if result != nil && (result.Success || saveDraftSucceeded(action, result)) {
		record.Status = "success"
	} else {
		record.Status = "unknown"
	}
	return record
}

func saveDraftSucceeded(action string, result *sheinpub.SubmissionResponse) bool {
	if action != "save_draft" || result == nil {
		return false
	}
	return strings.TrimSpace(result.Code) == "0"
}

func buildSheinSubmitResponseError(action string, result *sheinpub.SubmissionResponse) error {
	if result == nil || result.Success || saveDraftSucceeded(action, result) {
		return nil
	}
	if action != "publish" {
		return nil
	}
	if len(result.ValidationNotes) > 0 {
		return fmt.Errorf("SHEIN publish pre-validation failed: %s", strings.Join(result.ValidationNotes, "; "))
	}
	message := strings.TrimSpace(result.Message)
	if message == "" {
		message = strings.TrimSpace(result.Code)
	}
	if message == "" {
		return fmt.Errorf("SHEIN publish did not complete")
	}
	return fmt.Errorf("SHEIN publish did not complete: %s", message)
}

func applySheinSubmissionRecord(pkg *sheinpub.Package, record *sheinpub.SubmissionRecord) {
	if pkg == nil || record == nil {
		return
	}
	if pkg.Submission == nil {
		pkg.Submission = &sheinpub.SubmissionReport{}
	}
	pkg.Submission.LastAction = record.Action
	pkg.Submission.LastStatus = record.Status
	pkg.Submission.LastError = record.Error
	pkg.Submission.SubmittedAt = &record.SubmittedAt
	pkg.Submission.LastResult = record.Result
	switch record.Action {
	case "save_draft":
		pkg.Submission.SaveDraft = record
	case "publish":
		pkg.Submission.Publish = record
	}
}

func buildSheinSubmissionEvent(taskID, action string, record *sheinpub.SubmissionRecord, response *sheinpub.SubmissionResponse, submitErr error, startedAt time.Time) sheinpub.SubmissionEvent {
	finishedAt := time.Now()
	event := sheinpub.SubmissionEvent{
		TaskID:     taskID,
		Platform:   "shein",
		Action:     action,
		Status:     "unknown",
		StartedAt:  startedAt,
		FinishedAt: &finishedAt,
		Response:   response,
	}
	if record != nil {
		event.Status = record.Status
		event.RequestID = record.RequestID
		if event.Response == nil {
			event.Response = record.Result
		}
	}
	if event.Response != nil {
		event.ValidationNotes = append([]string(nil), event.Response.ValidationNotes...)
	}
	if submitErr != nil {
		event.Status = "failed"
		event.ErrorMessage = submitErr.Error()
	}
	return event
}

func firstSubmitReadinessMessage(readiness *SheinSubmitReadiness) string {
	if readiness == nil {
		return "SHEIN 提交前状态尚未就绪"
	}
	for _, line := range readiness.Summary {
		if value := strings.TrimSpace(line); value != "" {
			return value
		}
	}
	if len(readiness.BlockingItems) > 0 {
		return strings.TrimSpace(readiness.BlockingItems[0].Message)
	}
	return "SHEIN 提交前状态尚未就绪"
}

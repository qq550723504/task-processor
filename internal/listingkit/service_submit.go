package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
	sheintranslateapi "task-processor/internal/shein/api/translate"
	sheinpublish "task-processor/internal/shein/publish"
)

const sheinSubmitInFlightTTL = 15 * time.Minute

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
	unlockSubmit := s.sheinSubmitLocks.lock(taskID + ":" + action)
	defer unlockSubmit()

	startedAt := time.Now()
	requestID := normalizedSubmitIdempotencyKey(req)
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
	supplierCode := sheinSubmitSupplierCode(submitProduct, pkg)
	setSheinSubmitSupplierCode(pkg, action, requestID, supplierCode)
	response, responseErr := executeSheinSubmitRemote(productAPI, action, submitProduct)
	if responseErr == nil {
		responseErr = buildSheinSubmitResponseError(action, response)
	}

	if responseErr == nil {
		setSheinSubmitRemoteResponse(pkg, action, requestID, supplierCode, response)
		if err := s.persistSheinSubmitPhase(ctx, taskID, task.Result, pkg, action, requestID, sheinpub.SubmissionPhasePersistResult); err != nil {
			return nil, err
		}
		if err := s.persistSheinSubmitPhase(ctx, taskID, task.Result, pkg, action, requestID, sheinpub.SubmissionPhaseConfirmRemote); err != nil {
			return nil, err
		}
		remoteEvent := s.confirmSheinSubmitRemote(ctx, taskID, pkg, productAPI, action, requestID, supplierCode, startedAt)
		if remoteEvent != nil {
			appendSheinSubmissionEvent(pkg, *remoteEvent)
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

func (s *service) beginSheinSubmitLease(ctx context.Context, taskID, action, requestID string, startedAt time.Time) (*Task, error) {
	return s.mutateTaskResult(ctx, taskID, func(task *Task) error {
		if task.Result == nil {
			return ErrTaskResultUnavailable
		}
		pkg := task.Result.Shein
		if pkg == nil || pkg.PreviewProduct == nil {
			return errSheinSubmitMissingPackage
		}
		if findSheinSubmissionRecordByRequestID(pkg, action, requestID) != nil {
			return errSheinSubmitReplayExisting
		}
		sameRequestNeedsRecovery := pkg.Submission != nil &&
			pkg.Submission.CurrentRequestID == requestID &&
			pkg.Submission.CurrentPhase != sheinpub.SubmissionPhaseSubmitRemote
		if pkg.Submission != nil && (sameRequestNeedsRecovery || sheinSubmitAttemptNeedsRemoteRecovery(pkg.Submission, action, startedAt)) {
			record := sheinSubmissionRecordForAction(pkg.Submission, action)
			if record != nil && strings.TrimSpace(record.SupplierCode) != "" {
				appendSheinSubmissionEvent(pkg, buildSheinPhaseSubmissionEvent(taskID, action, pkg.Submission.CurrentPhase, sheinpub.SubmissionStatusRunning, requestID, startedAt, "远端可能已收到，正在按供方货号确认", nil))
				return errSheinSubmitRecoverRemote
			}
		}
		if active := findActiveSheinSubmitAttempt(pkg, action, startedAt); active != nil {
			if active.CurrentRequestID == requestID {
				return errSheinSubmitReplayExisting
			}
			return &SubmitInProgressError{
				Platform:       "shein",
				Action:         action,
				Phase:          active.CurrentPhase,
				RequestID:      active.CurrentRequestID,
				LeaseExpiresAt: active.LeaseExpiresAt,
			}
		}
		beginSheinSubmitAttempt(pkg, action, requestID, sheinpub.SubmissionPhaseValidate, startedAt)
		appendSheinSubmissionEvent(pkg, buildSheinPhaseSubmissionEvent(taskID, action, sheinpub.SubmissionPhaseValidate, sheinpub.SubmissionStatusRunning, requestID, startedAt, "", nil))
		task.Result.UpdatedAt = startedAt
		return nil
	})
}

func (s *service) mutateTaskResult(ctx context.Context, taskID string, mutate TaskResultMutation) (*Task, error) {
	if txRepo, ok := s.repo.(TaskResultTransactionRepository); ok {
		return txRepo.MutateTaskResult(ctx, taskID, mutate)
	}
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if mutate != nil {
		if err := mutate(task); err != nil {
			return task, err
		}
	}
	if task.Result != nil {
		if err := s.repo.SaveTaskResult(ctx, taskID, task.Result); err != nil {
			return nil, err
		}
	}
	return task, nil
}

func (s *service) recoverSheinSubmitRemote(ctx context.Context, task *Task, action string) (*ListingKitPreview, error) {
	if task == nil || task.Result == nil || task.Result.Shein == nil || task.Result.Shein.Submission == nil {
		return nil, ErrTaskResultUnavailable
	}
	pkg := task.Result.Shein
	report := pkg.Submission
	record := sheinSubmissionRecordForAction(report, action)
	if record == nil || strings.TrimSpace(record.SupplierCode) == "" {
		return nil, fmt.Errorf("%w: stale SHEIN submit has no supplier code", ErrSubmitInProgress)
	}
	productAPI, err := s.buildSheinSubmitProductAPI(task)
	if err != nil {
		return nil, err
	}
	requestID := report.CurrentRequestID
	now := time.Now()
	advanceSheinSubmitPhase(pkg, action, requestID, sheinpub.SubmissionPhaseConfirmRemote)
	appendSheinSubmissionEvent(pkg, buildSheinPhaseSubmissionEvent(task.ID, action, sheinpub.SubmissionPhaseConfirmRemote, sheinpub.SubmissionStatusRunning, requestID, now, "远端可能已收到，正在按供方货号确认", nil))
	event := s.confirmSheinSubmitRemote(ctx, task.ID, pkg, productAPI, action, requestID, record.SupplierCode, now)
	if event != nil {
		appendSheinSubmissionEvent(pkg, *event)
	}
	response := record.Result
	if response == nil && report.LastResult != nil {
		response = report.LastResult
	}
	record = completeSheinSubmitAttempt(pkg, action, requestID, response, nil, time.Now())
	if record.Result == nil {
		record.Status = sheinpub.SubmissionStatusSuccess
	}
	appendSheinSubmissionEvent(pkg, buildSheinSubmissionEvent(task.ID, action, record, record.Result, nil, record.StartedAt))
	task.Result.UpdatedAt = time.Now()
	if err := s.repo.SaveTaskResult(ctx, task.ID, task.Result); err != nil {
		return nil, err
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

func (s *service) confirmSheinSubmitRemote(ctx context.Context, taskID string, pkg *SheinPackage, productAPI sheinproduct.ProductAPI, action, requestID, supplierCode string, startedAt time.Time) *sheinpub.SubmissionEvent {
	supplierCode = strings.TrimSpace(supplierCode)
	if supplierCode == "" {
		now := time.Now()
		setSheinSubmitRemoteRecord(pkg, action, requestID, sheinpub.SubmissionRemoteStatusPending, nil, now, "missing supplier code")
		return ptrSheinSubmissionEvent(buildSheinPhaseSubmissionEvent(taskID, action, sheinpub.SubmissionPhaseConfirmRemote, sheinpub.SubmissionRemoteStatusPending, requestID, startedAt, "SHEIN submit succeeded, but supplier code is unavailable for remote confirmation", nil))
	}
	codes := []string{supplierCode}
	resp, err := productAPI.Record(&sheinproduct.ProductRecordRequest{
		Language:                  "en",
		OnlyCurrentMonthRecommend: false,
		OnlySpmbCopyProduct:       false,
		QueryTimeOut:              false,
		SearchDiyCustom:           false,
		SupplierCodeList:          &codes,
		SupplierCodeSearchType:    1,
	})
	now := time.Now()
	if err != nil {
		setSheinSubmitRemoteRecord(pkg, action, requestID, sheinpub.SubmissionRemoteStatusFailed, nil, now, err.Error())
		event := buildSheinPhaseSubmissionEvent(taskID, action, sheinpub.SubmissionPhaseConfirmRemote, sheinpub.SubmissionRemoteStatusFailed, requestID, startedAt, "SHEIN remote confirmation failed", err)
		return &event
	}
	if resp == nil || resp.Code != "0" {
		msg := "SHEIN remote confirmation returned no success code"
		if resp != nil && strings.TrimSpace(resp.Msg) != "" {
			msg = resp.Msg
		}
		setSheinSubmitRemoteRecord(pkg, action, requestID, sheinpub.SubmissionRemoteStatusFailed, nil, now, msg)
		event := buildSheinPhaseSubmissionEvent(taskID, action, sheinpub.SubmissionPhaseConfirmRemote, sheinpub.SubmissionRemoteStatusFailed, requestID, startedAt, msg, nil)
		return &event
	}
	if len(resp.Info.Data) == 0 {
		setSheinSubmitRemoteRecord(pkg, action, requestID, sheinpub.SubmissionRemoteStatusPending, nil, now, "record not found")
		event := buildSheinPhaseSubmissionEvent(taskID, action, sheinpub.SubmissionPhaseConfirmRemote, sheinpub.SubmissionRemoteStatusPending, requestID, startedAt, "SHEIN remote record is not visible yet", nil)
		return &event
	}
	item := resp.Info.Data[0]
	setSheinSubmitRemoteRecord(pkg, action, requestID, sheinpub.SubmissionRemoteStatusConfirmed, &item, now, "")
	event := buildSheinPhaseSubmissionEvent(taskID, action, sheinpub.SubmissionPhaseConfirmRemote, sheinpub.SubmissionRemoteStatusConfirmed, requestID, startedAt, "SHEIN remote record confirmed", nil)
	event.RemoteRecordID = item.RecordID
	return &event
}

func ptrSheinSubmissionEvent(event sheinpub.SubmissionEvent) *sheinpub.SubmissionEvent {
	return &event
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
		event.Phase = record.Phase
		event.RemoteRecordID = record.RemoteRecordID
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

func buildSheinPhaseSubmissionEvent(taskID, action, phase, status, requestID string, startedAt time.Time, detail string, err error) sheinpub.SubmissionEvent {
	finishedAt := time.Now()
	event := sheinpub.SubmissionEvent{
		TaskID:     taskID,
		Platform:   "shein",
		Action:     "submit_phase",
		Phase:      phase,
		Status:     status,
		RequestID:  requestID,
		StartedAt:  startedAt,
		FinishedAt: &finishedAt,
		Detail:     detail,
	}
	if event.Status == "" {
		event.Status = sheinpub.SubmissionStatusRunning
	}
	if err != nil {
		event.ErrorMessage = err.Error()
	}
	if event.Detail == "" {
		event.Detail = sheinSubmitPhaseDetail(action, phase)
	}
	return event
}

func sheinSubmitPhaseDetail(action, phase string) string {
	switch phase {
	case sheinpub.SubmissionPhaseValidate:
		return "检查 SHEIN 提交前状态"
	case sheinpub.SubmissionPhasePrepareProduct:
		return "准备 SHEIN 商品载荷"
	case sheinpub.SubmissionPhaseUploadImages:
		return "上传 SHEIN 商品图片"
	case sheinpub.SubmissionPhasePreValidate:
		return "执行 SHEIN 提交前校验"
	case sheinpub.SubmissionPhaseSubmitRemote:
		if action == "save_draft" {
			return "提交 SHEIN 草稿"
		}
		return "提交 SHEIN 发布请求"
	case sheinpub.SubmissionPhasePersistResult:
		return "保存本地提交结果"
	case sheinpub.SubmissionPhaseConfirmRemote:
		return "确认 SHEIN 远端记录"
	default:
		return phase
	}
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

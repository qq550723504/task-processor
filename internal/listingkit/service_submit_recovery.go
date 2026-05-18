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
)

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
			(pkg.Submission.CurrentPhase != sheinpub.SubmissionPhaseSubmitRemote ||
				sheinSubmitRemoteResponsePersisted(pkg, action, requestID))
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

func sheinSubmitRemoteResponsePersisted(pkg *SheinPackage, action, requestID string) bool {
	if pkg == nil || pkg.Submission == nil {
		return false
	}
	record := sheinSubmissionRecordForAction(pkg.Submission, action)
	if record == nil || record.RequestID != requestID {
		return false
	}
	return record.Result != nil
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
	productAPI, err := s.buildSheinSubmitProductAPI(ctx, task)
	if err != nil {
		return nil, err
	}
	requestID := report.CurrentRequestID
	now := time.Now()
	advanceSheinSubmitPhase(pkg, action, requestID, sheinpub.SubmissionPhaseConfirmRemote)
	appendSheinSubmissionEvent(pkg, buildSheinPhaseSubmissionEvent(task.ID, action, sheinpub.SubmissionPhaseConfirmRemote, sheinpub.SubmissionStatusRunning, requestID, now, "远端可能已收到，正在按供方货号确认", nil))
	event, remoteErr := s.confirmSheinSubmitRemote(ctx, task.ID, pkg, productAPI, action, requestID, record.SupplierCode, now)
	if event != nil {
		appendSheinSubmissionEvent(pkg, *event)
	}
	if remoteErr != nil {
		record = failSheinSubmitAttempt(pkg, action, requestID, sheinpub.SubmissionPhaseConfirmRemote, remoteErr, time.Now())
		appendSheinSubmissionEvent(pkg, buildSheinSubmissionEvent(task.ID, action, record, record.Result, remoteErr, record.StartedAt))
		task.Result.UpdatedAt = time.Now()
		if err := s.repo.SaveTaskResult(ctx, task.ID, task.Result); err != nil {
			return nil, err
		}
		return nil, remoteErr
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
	s.rememberSheinSubmittedResolution(task, action)
	task.Result.UpdatedAt = time.Now()
	if err := s.repo.SaveTaskResult(ctx, task.ID, task.Result); err != nil {
		return nil, err
	}
	return buildListingKitPreview(task, "shein")
}

func (s *service) RefreshSubmissionStatus(ctx context.Context, taskID string) (*ListingKitPreview, error) {
	unlockSubmit := s.sheinSubmitLocks.lock(taskID + ":refresh_submission_status")
	defer unlockSubmit()

	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task.Result == nil {
		return nil, ErrTaskResultUnavailable
	}
	pkg := task.Result.Shein
	if pkg == nil || pkg.Submission == nil {
		return nil, fmt.Errorf("%w: shein submission is not available", ErrSubmitBlocked)
	}
	report := pkg.Submission
	action := strings.TrimSpace(report.LastAction)
	if action == "" {
		if report.Publish != nil {
			action = "publish"
		} else if report.SaveDraft != nil {
			action = "save_draft"
		}
	}
	record := sheinSubmissionRecordForAction(report, action)
	if record == nil {
		return nil, fmt.Errorf("%w: shein submission record is not available", ErrSubmitBlocked)
	}
	supplierCode := strings.TrimSpace(record.SupplierCode)
	if supplierCode == "" {
		supplierCode = sheinSubmitSupplierCode(nil, pkg)
	}
	if supplierCode == "" {
		return nil, fmt.Errorf("%w: shein supplier code is not available", ErrSubmitBlocked)
	}
	productAPI, err := s.buildSheinSubmitProductAPI(ctx, task)
	if err != nil {
		return nil, err
	}
	requestID := strings.TrimSpace(record.RequestID)
	startedAt := time.Now()
	lookupCodes := collectSheinRemoteLookupCodes(pkg, supplierCode)
	defaultConfirmed := action == "publish" && sheinRemotePublishAccepted(pkg, action)
	fallbackMessage := "refreshing SHEIN remote record"
	if defaultConfirmed {
		fallbackMessage = "SHEIN accepted publish request; remote record not yet visible"
	}
	confirmation, remoteErr := s.resolveSheinSubmitRemoteStatus(productAPI, action, requestID, lookupCodes, sheinRemoteLookupSPUName(pkg, action), defaultConfirmed, fallbackMessage, startedAt, taskID)
	task, err = s.mutateTaskResult(ctx, taskID, func(task *Task) error {
		if task.Result == nil || task.Result.Shein == nil || task.Result.Shein.Submission == nil {
			return fmt.Errorf("%w: shein submission is not available", ErrSubmitBlocked)
		}
		currentAction := strings.TrimSpace(task.Result.Shein.Submission.LastAction)
		if currentAction == "" {
			currentAction = action
		}
		if currentAction != action {
			return fmt.Errorf("%w: shein submission changed during refresh", ErrSubmitInProgress)
		}
		currentRecord := sheinSubmissionRecordForAction(task.Result.Shein.Submission, action)
		if currentRecord == nil || strings.TrimSpace(currentRecord.RequestID) != requestID {
			return fmt.Errorf("%w: shein submission changed during refresh", ErrSubmitInProgress)
		}
		appendSheinSubmissionEvent(task.Result.Shein, buildSheinPhaseSubmissionEvent(taskID, action, sheinpub.SubmissionPhaseConfirmRemote, sheinpub.SubmissionStatusRunning, requestID, startedAt, "刷新 SHEIN 远端提交状态", nil))
		if confirmation.event != nil {
			appendSheinSubmissionEvent(task.Result.Shein, *confirmation.event)
		}
		setSheinSubmitRemoteRecord(task.Result.Shein, action, requestID, confirmation.remoteStatus, confirmation.record, confirmation.checkedAt, confirmation.message)
		task.Result.UpdatedAt = time.Now()
		return nil
	})
	if err != nil {
		return nil, err
	}
	if remoteErr != nil {
		return nil, remoteErr
	}
	return buildListingKitPreview(task, "shein")
}

func (s *service) confirmSheinSubmitRemote(ctx context.Context, taskID string, pkg *SheinPackage, productAPI sheinproduct.ProductAPI, action, requestID, supplierCode string, startedAt time.Time) (*sheinpub.SubmissionEvent, error) {
	lookupCodes := collectSheinRemoteLookupCodes(pkg, supplierCode)
	defaultConfirmed := action == "publish" && sheinRemotePublishAccepted(pkg, action)
	fallbackMessage := "refreshing SHEIN remote record"
	if defaultConfirmed {
		fallbackMessage = "SHEIN accepted publish request; remote record not yet visible"
	}
	if len(lookupCodes) == 0 {
		now := time.Now()
		remoteStatus := sheinpub.SubmissionRemoteStatusPending
		detail := "SHEIN submit succeeded, but supplier code is unavailable for remote confirmation"
		if defaultConfirmed {
			remoteStatus = sheinpub.SubmissionRemoteStatusConfirmed
			detail = "SHEIN accepted publish request, but supplier code is unavailable for remote confirmation"
		}
		setSheinSubmitRemoteRecord(pkg, action, requestID, remoteStatus, nil, now, detail)
		return ptrSheinSubmissionEvent(buildSheinPhaseSubmissionEvent(taskID, action, sheinpub.SubmissionPhaseConfirmRemote, remoteStatus, requestID, startedAt, detail, nil)), nil
	}
	confirmation, err := s.resolveSheinSubmitRemoteStatus(productAPI, action, requestID, lookupCodes, sheinRemoteLookupSPUName(pkg, action), defaultConfirmed, fallbackMessage, startedAt, taskID)
	setSheinSubmitRemoteRecord(pkg, action, requestID, confirmation.remoteStatus, confirmation.record, confirmation.checkedAt, confirmation.message)
	return confirmation.event, err
}

func ptrSheinSubmissionEvent(event sheinpub.SubmissionEvent) *sheinpub.SubmissionEvent {
	return &event
}

func (s *service) clearSheinSubmitLease(ctx context.Context, taskID, action, requestID string) error {
	_, err := s.mutateTaskResult(ctx, taskID, func(task *Task) error {
		if task.Result == nil || task.Result.Shein == nil || task.Result.Shein.Submission == nil {
			return nil
		}
		clearSheinSubmitInFlight(task.Result.Shein.Submission, action, requestID)
		task.Result.UpdatedAt = time.Now()
		return nil
	})
	return err
}

func (s *service) clearSheinSubmitLeaseAfterStartFailure(ctx context.Context, taskID, action, requestID string, startErr error) error {
	_, err := s.mutateTaskResult(ctx, taskID, func(task *Task) error {
		if task.Result == nil || task.Result.Shein == nil || task.Result.Shein.Submission == nil {
			return nil
		}
		pkg := task.Result.Shein
		record := sheinSubmissionRecordForAction(pkg.Submission, action)
		if record != nil && record.RequestID == requestID && record.Status == sheinpub.SubmissionStatusRunning {
			finishedAt := time.Now()
			record.Status = sheinpub.SubmissionStatusFailed
			record.Phase = sheinpub.SubmissionPhaseValidate
			record.FinishedAt = &finishedAt
			record.SubmittedAt = finishedAt
			if startErr != nil {
				record.Error = startErr.Error()
			}
			listingsubmission.ApplyRecord(pkg, record)
			appendSheinSubmissionEvent(pkg, buildSheinSubmissionEvent(taskID, action, record, nil, startErr, record.StartedAt))
		}
		clearSheinSubmitInFlight(pkg.Submission, action, requestID)
		task.Result.UpdatedAt = time.Now()
		return nil
	})
	return err
}

type sheinRemoteConfirmation struct {
	remoteStatus string
	record       *sheinproduct.RecordItem
	checkedAt    time.Time
	message      string
	event        *sheinpub.SubmissionEvent
}

func (s *service) resolveSheinSubmitRemoteStatus(productAPI sheinproduct.ProductAPI, action, requestID string, lookupCodes []string, spuName string, defaultConfirmed bool, fallbackMessage string, startedAt time.Time, taskID string) (*sheinRemoteConfirmation, error) {
	item, recordErr := lookupSheinRemoteRecord(productAPI, lookupCodes, spuName)
	now := time.Now()
	if recordErr == nil && item != nil {
		remoteStatus, detail, remoteErr := classifySheinRemoteRecord(action, item, defaultConfirmed)
		event := buildSheinPhaseSubmissionEvent(taskID, action, sheinpub.SubmissionPhaseConfirmRemote, remoteStatus, requestID, startedAt, detail, remoteErr)
		event.RemoteRecordID = item.RecordID
		return &sheinRemoteConfirmation{
			remoteStatus: remoteStatus,
			record:       item,
			checkedAt:    now,
			message:      detail,
			event:        &event,
		}, remoteErr
	}
	if action == "publish" {
		if spuName != "" {
			inventoryExists, inventoryErr := lookupSheinRemoteInventory(productAPI, spuName)
			if inventoryErr == nil && inventoryExists {
				detail := fmt.Sprintf("SHEIN remote inventory confirmed for spu_name=%s", spuName)
				event := buildSheinPhaseSubmissionEvent(taskID, action, sheinpub.SubmissionPhaseConfirmRemote, sheinpub.SubmissionRemoteStatusConfirmed, requestID, startedAt, detail, nil)
				return &sheinRemoteConfirmation{
					remoteStatus: sheinpub.SubmissionRemoteStatusConfirmed,
					checkedAt:    now,
					message:      detail,
					event:        &event,
				}, nil
			}
		}
	}
	if defaultConfirmed {
		event := buildSheinPhaseSubmissionEvent(taskID, action, sheinpub.SubmissionPhaseConfirmRemote, sheinpub.SubmissionRemoteStatusConfirmed, requestID, startedAt, fallbackMessage, nil)
		return &sheinRemoteConfirmation{
			remoteStatus: sheinpub.SubmissionRemoteStatusConfirmed,
			checkedAt:    now,
			message:      fallbackMessage,
			event:        &event,
		}, nil
	}
	if recordErr != nil {
		event := buildSheinPhaseSubmissionEvent(taskID, action, sheinpub.SubmissionPhaseConfirmRemote, sheinpub.SubmissionRemoteStatusPending, requestID, startedAt, fallbackMessage, nil)
		return &sheinRemoteConfirmation{
			remoteStatus: sheinpub.SubmissionRemoteStatusPending,
			checkedAt:    now,
			message:      recordErr.Error(),
			event:        &event,
		}, nil
	}
	event := buildSheinPhaseSubmissionEvent(taskID, action, sheinpub.SubmissionPhaseConfirmRemote, sheinpub.SubmissionRemoteStatusPending, requestID, startedAt, fallbackMessage, nil)
	return &sheinRemoteConfirmation{
		remoteStatus: sheinpub.SubmissionRemoteStatusPending,
		checkedAt:    now,
		message:      "record not found",
		event:        &event,
	}, nil
}

func classifySheinRemoteRecord(action string, item *sheinproduct.RecordItem, publishAccepted bool) (string, string, error) {
	if item == nil {
		return sheinpub.SubmissionRemoteStatusPending, "record not found", nil
	}
	if action == "save_draft" {
		return sheinpub.SubmissionRemoteStatusConfirmed, "SHEIN draft record confirmed", nil
	}
	if publishAccepted {
		return sheinpub.SubmissionRemoteStatusConfirmed, fmt.Sprintf("SHEIN publish API reported success (state=%d audit_state=%d)", item.State, item.AuditState), nil
	}
	if sheinRemoteRecordLooksDraft(item) {
		message := fmt.Sprintf("SHEIN publish landed in draft state (state=%d audit_state=%d)", item.State, item.AuditState)
		return sheinpub.SubmissionRemoteStatusFailed, message, errors.New(message)
	}
	if sheinRemoteRecordLooksConfirmed(item) {
		return sheinpub.SubmissionRemoteStatusConfirmed, "SHEIN remote record confirmed", nil
	}
	return sheinpub.SubmissionRemoteStatusPending, fmt.Sprintf("SHEIN remote record is not yet publish-confirmed (state=%d audit_state=%d)", item.State, item.AuditState), nil
}

func sheinRemoteRecordLooksDraft(item *sheinproduct.RecordItem) bool {
	if item == nil {
		return false
	}
	switch item.State {
	case 1:
		return true
	}
	switch item.AuditState {
	case 1, 2:
		return true
	}
	return false
}

func sheinRemoteRecordLooksConfirmed(item *sheinproduct.RecordItem) bool {
	if item == nil {
		return false
	}
	switch item.State {
	case 2, 4:
		return true
	}
	switch item.AuditState {
	case 3, 5:
		return true
	}
	return false
}

func lookupSheinRemoteRecord(productAPI sheinproduct.ProductAPI, codes []string, expectedSPUName string) (*sheinproduct.RecordItem, error) {
	if len(codes) == 0 {
		return nil, nil
	}
	resp, err := productAPI.Record(&sheinproduct.ProductRecordRequest{
		Language:                  "en",
		OnlyCurrentMonthRecommend: false,
		OnlySpmbCopyProduct:       false,
		QueryTimeOut:              false,
		SearchDiyCustom:           false,
		SupplierCodeList:          &codes,
		SupplierCodeSearchType:    1,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.Code != "0" {
		msg := "SHEIN remote record query returned no success code"
		if resp != nil && strings.TrimSpace(resp.Msg) != "" {
			msg = resp.Msg
		}
		return nil, errors.New(msg)
	}
	if len(resp.Info.Data) == 0 {
		return nil, nil
	}
	if expectedSPUName = strings.TrimSpace(expectedSPUName); expectedSPUName != "" {
		for i := range resp.Info.Data {
			if strings.EqualFold(strings.TrimSpace(resp.Info.Data[i].SpuName), expectedSPUName) {
				return &resp.Info.Data[i], nil
			}
		}
	}
	best := resp.Info.Data[0]
	bestTime := parseSheinRemoteRecordTime(best.CreateTime)
	for i := 1; i < len(resp.Info.Data); i++ {
		candidate := resp.Info.Data[i]
		candidateTime := parseSheinRemoteRecordTime(candidate.CreateTime)
		if candidateTime.After(bestTime) {
			best = candidate
			bestTime = candidateTime
		}
	}
	return &best, nil
}

func parseSheinRemoteRecordTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	layouts := []string{
		"2006-01-02 15:04:05",
		time.RFC3339,
	}
	for _, layout := range layouts {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func lookupSheinRemoteInventory(productAPI sheinproduct.ProductAPI, spuName string) (bool, error) {
	spuName = strings.TrimSpace(spuName)
	if spuName == "" {
		return false, nil
	}
	resp, err := productAPI.QueryInventory(spuName)
	if err != nil {
		return false, err
	}
	if resp == nil || strings.TrimSpace(resp.Code) != "0" {
		return false, nil
	}
	return strings.TrimSpace(resp.Info.SpuName) != "", nil
}

func sheinRemoteLookupSPUName(pkg *SheinPackage, action string) string {
	if pkg == nil || pkg.Submission == nil {
		return ""
	}
	record := sheinSubmissionRecordForAction(pkg.Submission, action)
	if record != nil && record.Result != nil {
		if value := strings.TrimSpace(record.Result.SPUName); value != "" {
			return value
		}
	}
	if pkg.Submission.LastResult != nil {
		if value := strings.TrimSpace(pkg.Submission.LastResult.SPUName); value != "" {
			return value
		}
	}
	return ""
}

func sheinRemotePublishAccepted(pkg *SheinPackage, action string) bool {
	if action != "publish" || pkg == nil || pkg.Submission == nil {
		return false
	}
	record := sheinSubmissionRecordForAction(pkg.Submission, action)
	if record != nil && record.Result != nil && record.Result.Success {
		return true
	}
	return pkg.Submission.LastResult != nil && pkg.Submission.LastResult.Success
}

func collectSheinRemoteLookupCodes(pkg *SheinPackage, supplierCode string) []string {
	seen := make(map[string]struct{})
	codes := make([]string, 0, 8)
	appendCode := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		codes = append(codes, value)
	}

	appendCode(supplierCode)
	if pkg == nil || pkg.PreviewProduct == nil {
		return codes
	}
	appendCode(pkg.PreviewProduct.SupplierCode)
	for _, skc := range pkg.PreviewProduct.SKCList {
		if skc.SupplierCode != nil {
			appendCode(*skc.SupplierCode)
		}
		for _, sku := range skc.SKUS {
			appendCode(sku.SupplierSKU)
		}
	}
	return codes
}

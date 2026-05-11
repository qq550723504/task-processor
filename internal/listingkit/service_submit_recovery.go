package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

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
	productAPI, err := s.buildSheinSubmitProductAPI(task)
	if err != nil {
		return nil, err
	}
	requestID := strings.TrimSpace(record.RequestID)
	startedAt := time.Now()
	appendSheinSubmissionEvent(pkg, buildSheinPhaseSubmissionEvent(taskID, action, sheinpub.SubmissionPhaseConfirmRemote, sheinpub.SubmissionStatusRunning, requestID, startedAt, "刷新 SHEIN 远端提交状态", nil))
	event := s.confirmSheinSubmitRemote(ctx, taskID, pkg, productAPI, action, requestID, supplierCode, startedAt)
	if event != nil {
		appendSheinSubmissionEvent(pkg, *event)
	}
	task.Result.UpdatedAt = time.Now()
	if err := s.repo.SaveTaskResult(ctx, taskID, task.Result); err != nil {
		return nil, err
	}
	return buildListingKitPreview(task, "shein")
}

func (s *service) confirmSheinSubmitRemote(ctx context.Context, taskID string, pkg *SheinPackage, productAPI sheinproduct.ProductAPI, action, requestID, supplierCode string, startedAt time.Time) *sheinpub.SubmissionEvent {
	supplierCode = strings.TrimSpace(supplierCode)
	if supplierCode == "" {
		now := time.Now()
		setSheinSubmitRemoteRecord(pkg, action, requestID, sheinpub.SubmissionRemoteStatusPending, nil, now, "missing supplier code")
		return ptrSheinSubmissionEvent(buildSheinPhaseSubmissionEvent(taskID, action, sheinpub.SubmissionPhaseConfirmRemote, sheinpub.SubmissionRemoteStatusPending, requestID, startedAt, "SHEIN submit succeeded, but supplier code is unavailable for remote confirmation", nil))
	}
	return s.confirmSheinRemoteRecordFallback(taskID, pkg, productAPI, action, requestID, supplierCode, startedAt, false, "refreshing SHEIN remote record")
}

func ptrSheinSubmissionEvent(event sheinpub.SubmissionEvent) *sheinpub.SubmissionEvent {
	return &event
}

func (s *service) confirmSheinRemoteRecordFallback(taskID string, pkg *SheinPackage, productAPI sheinproduct.ProductAPI, action, requestID, supplierCode string, startedAt time.Time, defaultConfirmed bool, fallbackMessage string) *sheinpub.SubmissionEvent {
	item, recordErr := lookupSheinRemoteRecord(productAPI, supplierCode)
	now := time.Now()
	if recordErr == nil && item != nil {
		setSheinSubmitRemoteRecord(pkg, action, requestID, sheinpub.SubmissionRemoteStatusConfirmed, item, now, "")
		event := buildSheinPhaseSubmissionEvent(taskID, action, sheinpub.SubmissionPhaseConfirmRemote, sheinpub.SubmissionRemoteStatusConfirmed, requestID, startedAt, "SHEIN remote record confirmed", nil)
		event.RemoteRecordID = item.RecordID
		return &event
	}
	if defaultConfirmed {
		setSheinSubmitRemoteRecord(pkg, action, requestID, sheinpub.SubmissionRemoteStatusConfirmed, nil, now, fallbackMessage)
		event := buildSheinPhaseSubmissionEvent(taskID, action, sheinpub.SubmissionPhaseConfirmRemote, sheinpub.SubmissionRemoteStatusConfirmed, requestID, startedAt, fallbackMessage, nil)
		return &event
	}
	if recordErr != nil {
		setSheinSubmitRemoteRecord(pkg, action, requestID, sheinpub.SubmissionRemoteStatusPending, nil, now, recordErr.Error())
		event := buildSheinPhaseSubmissionEvent(taskID, action, sheinpub.SubmissionPhaseConfirmRemote, sheinpub.SubmissionRemoteStatusPending, requestID, startedAt, fallbackMessage, nil)
		return &event
	}
	setSheinSubmitRemoteRecord(pkg, action, requestID, sheinpub.SubmissionRemoteStatusPending, nil, now, "record not found")
	event := buildSheinPhaseSubmissionEvent(taskID, action, sheinpub.SubmissionPhaseConfirmRemote, sheinpub.SubmissionRemoteStatusPending, requestID, startedAt, fallbackMessage, nil)
	return &event
}

func lookupSheinRemoteRecord(productAPI sheinproduct.ProductAPI, supplierCode string) (*sheinproduct.RecordItem, error) {
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
	return &resp.Info.Data[0], nil
}

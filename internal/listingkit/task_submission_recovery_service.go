package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	listingsubmission "task-processor/internal/listingkit/submission"
	sheinpub "task-processor/internal/publishing/shein"
	sheinother "task-processor/internal/shein/api/other"
	sheinproduct "task-processor/internal/shein/api/product"
)

type taskSubmissionRecoveryServiceConfig struct {
	repo                        Repository
	buildTaskPreview            func(context.Context, *Task, string) (*ListingKitPreview, error)
	buildSheinSubmitProductAPI  func(context.Context, *Task) (sheinproduct.ProductAPI, error)
	buildSheinSubmitOtherAPI    func(context.Context, *Task) (sheinother.OtherAPI, error)
	rememberSheinSubmitted      func(*Task, string)
	persistSuccessfulSubmission func(context.Context, string, *Task, string) error
	resolveRemoteStatusCallback func(sheinproduct.ProductAPI, sheinother.OtherAPI, string, string, []string, string, bool, string, time.Time, string) (*sheinRemoteConfirmation, error)
}

type taskSubmissionRecoveryService struct {
	repo                        Repository
	buildTaskPreview            func(context.Context, *Task, string) (*ListingKitPreview, error)
	buildSheinSubmitProductAPI  func(context.Context, *Task) (sheinproduct.ProductAPI, error)
	buildSheinSubmitOtherAPI    func(context.Context, *Task) (sheinother.OtherAPI, error)
	rememberSheinSubmitted      func(*Task, string)
	persistSuccessfulSubmission func(context.Context, string, *Task, string) error
	resolveRemoteStatusCallback func(sheinproduct.ProductAPI, sheinother.OtherAPI, string, string, []string, string, bool, string, time.Time, string) (*sheinRemoteConfirmation, error)
}

func newTaskSubmissionRecoveryService(config taskSubmissionRecoveryServiceConfig) *taskSubmissionRecoveryService {
	return &taskSubmissionRecoveryService{
		repo:                        config.repo,
		buildTaskPreview:            config.buildTaskPreview,
		buildSheinSubmitProductAPI:  config.buildSheinSubmitProductAPI,
		buildSheinSubmitOtherAPI:    config.buildSheinSubmitOtherAPI,
		rememberSheinSubmitted:      config.rememberSheinSubmitted,
		persistSuccessfulSubmission: config.persistSuccessfulSubmission,
		resolveRemoteStatusCallback: config.resolveRemoteStatusCallback,
	}
}

func (s *taskSubmissionRecoveryService) beginSheinSubmitLease(ctx context.Context, taskID, action, requestID string, startedAt time.Time) (*Task, error) {
	return s.mutateTaskResult(ctx, taskID, func(task *Task) error {
		if task.Result == nil {
			return ErrTaskResultUnavailable
		}
		pkg := sheinpub.NormalizePackageSemanticFields(task.Result.Shein)
		if pkg == nil || pkg.PreviewPayload == nil {
			return errSheinSubmitMissingPackage
		}
		if findSheinSubmissionRecordByRequestID(pkg, action, requestID) != nil {
			return errSheinSubmitReplayExisting
		}
		sameRequestNeedsRecovery := pkg.SubmissionState != nil &&
			pkg.SubmissionState.CurrentRequestID == requestID &&
			(pkg.SubmissionState.CurrentPhase != sheinpub.SubmissionPhaseSubmitRemote ||
				sheinSubmitRemoteResponsePersisted(pkg, action, requestID))
		if pkg.SubmissionState != nil && (sameRequestNeedsRecovery || sheinSubmitAttemptNeedsRemoteRecovery(pkg.SubmissionState, action, startedAt)) {
			record := sheinSubmissionRecordForAction(pkg.SubmissionState, action)
			if record != nil && strings.TrimSpace(record.SupplierCode) != "" {
				appendSheinSubmissionEvent(pkg, listingsubmission.BuildPhaseEvent(taskID, action, pkg.SubmissionState.CurrentPhase, sheinpub.SubmissionStatusRunning, requestID, startedAt, "远端可能已收到，正在刷新诊断状态", nil))
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
		_, event := listingsubmission.BeginAttemptAndBuildEvent(pkg, taskID, action, requestID, sheinpub.SubmissionPhaseValidate, startedAt, sheinSubmitInFlightTTL)
		appendSheinSubmissionEvent(pkg, event)
		task.Result.UpdatedAt = startedAt
		return nil
	})
}

func sheinSubmitRemoteResponsePersisted(pkg *SheinPackage, action, requestID string) bool {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.SubmissionState == nil {
		return false
	}
	record := sheinSubmissionRecordForAction(pkg.SubmissionState, action)
	if record == nil || record.RequestID != requestID {
		return false
	}
	return record.Result != nil
}

func (s *taskSubmissionRecoveryService) mutateTaskResult(ctx context.Context, taskID string, mutate TaskResultMutation) (*Task, error) {
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

func (s *taskSubmissionRecoveryService) recoverSheinSubmitRemote(ctx context.Context, task *Task, action string) (*ListingKitPreview, error) {
	if task == nil || task.Result == nil {
		return nil, ErrTaskResultUnavailable
	}
	pkg := sheinpub.NormalizePackageSemanticFields(task.Result.Shein)
	if pkg == nil || pkg.SubmissionState == nil {
		return nil, ErrTaskResultUnavailable
	}
	report := pkg.SubmissionState
	record := sheinSubmissionRecordForAction(report, action)
	requestID := report.CurrentRequestID
	now := time.Now()
	response := recordResult(record)
	if response == nil {
		response = report.LastResult
	}
	if sheinSubmissionResponseAccepted(response) || (action == "save_draft" && listingsubmission.SaveDraftSucceeded(action, response)) {
		appendSheinSubmissionEvent(pkg, listingsubmission.BuildPhaseEvent(task.ID, action, sheinpub.SubmissionPhasePersistResult, sheinpub.SubmissionStatusRunning, requestID, now, "恢复本地提交完成状态", nil))
		_, completionEvent := listingsubmission.CompleteAttemptAndBuildEvent(pkg, task.ID, action, requestID, response, nil, record.StartedAt, time.Now())
		appendSheinSubmissionEvent(pkg, completionEvent)
		if s.rememberSheinSubmitted != nil {
			s.rememberSheinSubmitted(task, action)
		}
		if err := s.persistSuccessfulSubmission(ctx, task.ID, task, action); err != nil {
			return nil, err
		}
		return s.buildTaskPreview(ctx, task, "shein")
	}
	if record == nil || strings.TrimSpace(record.SupplierCode) == "" {
		return nil, fmt.Errorf("%w: stale SHEIN submit has no supplier code", ErrSubmitInProgress)
	}
	productAPI, err := s.buildSheinSubmitProductAPI(ctx, task)
	if err != nil {
		return nil, err
	}
	appendSheinSubmissionEvent(pkg, listingsubmission.AdvancePhaseAndBuildEvent(pkg, task.ID, action, requestID, sheinpub.SubmissionPhaseConfirmRemote, now, sheinSubmitInFlightTTL))
	event, remoteErr := s.refreshSheinSubmitRemoteStatus(ctx, task, task.ID, pkg, productAPI, action, requestID, record.SupplierCode, now)
	if event != nil {
		appendSheinSubmissionEvent(pkg, *event)
	}
	if remoteErr != nil {
		_, failureEvent := listingsubmission.FailAttemptAndBuildEvent(pkg, task.ID, action, requestID, sheinpub.SubmissionPhaseConfirmRemote, remoteErr, time.Now())
		appendSheinSubmissionEvent(pkg, failureEvent)
		task.Result.UpdatedAt = time.Now()
		if err := s.repo.SaveTaskResult(ctx, task.ID, task.Result); err != nil {
			return nil, err
		}
		return nil, remoteErr
	}
	response = record.Result
	if response == nil && report.LastResult != nil {
		response = report.LastResult
	}
	record, completionEvent := listingsubmission.CompleteAttemptAndBuildEvent(pkg, task.ID, action, requestID, response, nil, record.StartedAt, time.Now())
	if record.Result == nil {
		record.Status = sheinpub.SubmissionStatusSuccess
	}
	appendSheinSubmissionEvent(pkg, completionEvent)
	if s.rememberSheinSubmitted != nil {
		s.rememberSheinSubmitted(task, action)
	}
	if err := s.persistSuccessfulSubmission(ctx, task.ID, task, action); err != nil {
		return nil, err
	}
	return s.buildTaskPreview(ctx, task, "shein")
}

func (s *taskSubmissionRecoveryService) refreshSheinSubmitRemoteStatus(ctx context.Context, task *Task, taskID string, pkg *SheinPackage, productAPI sheinproduct.ProductAPI, action, requestID, supplierCode string, startedAt time.Time) (*sheinpub.SubmissionEvent, error) {
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
		event := listingsubmission.BuildConfirmRemoteEvent(taskID, action, remoteStatus, requestID, startedAt, detail, nil)
		return &event, nil
	}
	var otherAPI sheinother.OtherAPI
	if s.buildSheinSubmitOtherAPI != nil && task != nil {
		otherAPI, _ = s.buildSheinSubmitOtherAPI(ctx, task)
	}
	confirmation, err := s.resolveRemoteStatus(productAPI, otherAPI, action, requestID, lookupCodes, sheinRemoteLookupSPUName(pkg, action), defaultConfirmed, fallbackMessage, startedAt, taskID)
	setSheinSubmitRemoteRecord(pkg, action, requestID, confirmation.remoteStatus, confirmation.record, confirmation.checkedAt, confirmation.message)
	return confirmation.event, err
}

func (s *taskSubmissionRecoveryService) clearSheinSubmitLease(ctx context.Context, taskID, action, requestID string) error {
	_, err := s.mutateTaskResult(ctx, taskID, func(task *Task) error {
		if task.Result == nil {
			return nil
		}
		pkg := sheinpub.NormalizePackageSemanticFields(task.Result.Shein)
		if pkg == nil || pkg.SubmissionState == nil {
			return nil
		}
		clearSheinSubmitInFlight(pkg.SubmissionState, action, requestID)
		task.Result.UpdatedAt = time.Now()
		return nil
	})
	return err
}

func (s *taskSubmissionRecoveryService) clearSheinSubmitLeaseAfterStartFailure(ctx context.Context, taskID, action, requestID string, startErr error) error {
	_, err := s.mutateTaskResult(ctx, taskID, func(task *Task) error {
		if task.Result == nil {
			return nil
		}
		pkg := sheinpub.NormalizePackageSemanticFields(task.Result.Shein)
		if pkg == nil || pkg.SubmissionState == nil {
			return nil
		}
		record := sheinSubmissionRecordForAction(pkg.SubmissionState, action)
		if record != nil && record.RequestID == requestID && record.Status == sheinpub.SubmissionStatusRunning {
			finishedAt := time.Now()
			record.Status = sheinpub.SubmissionStatusFailed
			record.Phase = sheinpub.SubmissionPhaseValidate
			record.FinishedAt = &finishedAt
			if startErr != nil {
				record.Error = startErr.Error()
			}
			listingsubmission.ApplyRecord(pkg, record)
			appendSheinSubmissionEvent(pkg, listingsubmission.BuildEvent(taskID, action, record, nil, startErr, record.StartedAt))
		}
		clearSheinSubmitInFlight(pkg.SubmissionState, action, requestID)
		task.Result.UpdatedAt = time.Now()
		return nil
	})
	return err
}

func (s *taskSubmissionRecoveryService) resolveRemoteStatus(productAPI sheinproduct.ProductAPI, otherAPI sheinother.OtherAPI, action, requestID string, lookupCodes []string, spuName string, defaultConfirmed bool, fallbackMessage string, startedAt time.Time, taskID string) (*sheinRemoteConfirmation, error) {
	if s.resolveRemoteStatusCallback == nil {
		return nil, errors.New("submit remote status resolution is not configured")
	}
	return s.resolveRemoteStatusCallback(productAPI, otherAPI, action, requestID, lookupCodes, spuName, defaultConfirmed, fallbackMessage, startedAt, taskID)
}

type sheinRemoteConfirmation struct {
	remoteStatus string
	record       *sheinproduct.RecordItem
	checkedAt    time.Time
	message      string
	event        *sheinpub.SubmissionEvent
}

func newSheinRemoteConfirmation(parts listingsubmission.ConfirmRemoteParts) *sheinRemoteConfirmation {
	return &sheinRemoteConfirmation{
		remoteStatus: parts.RemoteStatus,
		record:       parts.Record,
		checkedAt:    parts.CheckedAt,
		message:      parts.Message,
		event:        &parts.Event,
	}
}

func (s *taskSubmissionRecoveryService) resolveSheinSubmitRemoteStatus(productAPI sheinproduct.ProductAPI, otherAPI sheinother.OtherAPI, action, requestID string, lookupCodes []string, spuName string, defaultConfirmed bool, fallbackMessage string, startedAt time.Time, taskID string) (*sheinRemoteConfirmation, error) {
	return s.resolveRemoteStatus(productAPI, otherAPI, action, requestID, lookupCodes, spuName, defaultConfirmed, fallbackMessage, startedAt, taskID)
}

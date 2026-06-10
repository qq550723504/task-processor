package listingkit

import (
	"context"
	"errors"
	"time"

	"task-processor/internal/listingkit/submission"
	sheinpub "task-processor/internal/publishing/shein"
	sheinother "task-processor/internal/shein/api/other"
	sheinproduct "task-processor/internal/shein/api/product"
)

func (s *taskSubmissionRecoveryService) refreshSheinSubmitRemoteStatus(ctx context.Context, task *Task, taskID string, pkg *SheinPackage, productAPI sheinproduct.ProductAPI, action, requestID, supplierCode string, startedAt time.Time) (*sheinpub.SubmissionEvent, error) {
	inputs := buildSheinRemoteRefreshState(pkg, action, supplierCode)
	if len(inputs.lookupCodes) == 0 {
		return applyMissingSupplierCodeRemoteConfirmation(pkg, taskID, action, requestID, startedAt, inputs.defaultConfirmed), nil
	}
	var otherAPI sheinother.OtherAPI
	if s.buildSheinSubmitOtherAPI != nil && task != nil {
		otherAPI, _ = s.buildSheinSubmitOtherAPI(ctx, task)
	}
	confirmation, err := s.resolveRemoteStatus(productAPI, otherAPI, action, requestID, inputs.lookupCodes, inputs.spuName, inputs.defaultConfirmed, inputs.fallbackMessage, startedAt, taskID)
	applySheinRemoteRefreshConfirmation(pkg, action, requestID, confirmation)
	return confirmation.event, err
}

type sheinRemoteRefreshState struct {
	lookupCodes      []string
	spuName          string
	defaultConfirmed bool
	fallbackMessage  string
}

func buildSheinRemoteRefreshState(pkg *SheinPackage, action, supplierCode string) sheinRemoteRefreshState {
	defaultConfirmed := action == "publish" && sheinRemotePublishAccepted(pkg, action)
	fallbackMessage := "refreshing SHEIN remote record"
	if defaultConfirmed {
		fallbackMessage = "SHEIN accepted publish request; remote record not yet visible"
	}
	return sheinRemoteRefreshState{
		lookupCodes:      collectSheinRemoteLookupCodes(pkg, supplierCode),
		spuName:          sheinRemoteLookupSPUName(pkg, action),
		defaultConfirmed: defaultConfirmed,
		fallbackMessage:  fallbackMessage,
	}
}

func applySheinRemoteRefreshConfirmation(pkg *SheinPackage, action, requestID string, confirmation *sheinRemoteConfirmation) {
	if pkg == nil || confirmation == nil {
		return
	}
	setSheinSubmitRemoteRecord(pkg, action, requestID, confirmation.remoteStatus, confirmation.record, confirmation.checkedAt, confirmation.message)
}

func applyMissingSupplierCodeRemoteConfirmation(pkg *SheinPackage, taskID, action, requestID string, startedAt time.Time, defaultConfirmed bool) *sheinpub.SubmissionEvent {
	now := time.Now()
	remoteStatus := sheinpub.SubmissionRemoteStatusPending
	detail := "SHEIN submit succeeded, but supplier code is unavailable for remote confirmation"
	if defaultConfirmed {
		remoteStatus = sheinpub.SubmissionRemoteStatusConfirmed
		detail = "SHEIN accepted publish request, but supplier code is unavailable for remote confirmation"
	}
	setSheinSubmitRemoteRecord(pkg, action, requestID, remoteStatus, nil, now, detail)
	event := submission.BuildConfirmRemoteEvent(taskID, action, remoteStatus, requestID, startedAt, detail, nil)
	return &event
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

func newSheinRemoteConfirmation(parts submission.ConfirmRemoteParts) *sheinRemoteConfirmation {
	return &sheinRemoteConfirmation{
		remoteStatus: parts.RemoteStatus,
		record:       parts.Record,
		checkedAt:    parts.CheckedAt,
		message:      parts.Message,
		event:        &parts.Event,
	}
}

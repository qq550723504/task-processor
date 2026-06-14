package listingkit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	sheinmarketpub "task-processor/internal/marketplace/shein/publishing"
	sheinpub "task-processor/internal/publishing/shein"
	sheinother "task-processor/internal/shein/api/other"
	sheinproduct "task-processor/internal/shein/api/product"

	"github.com/sirupsen/logrus"
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

type sheinOnWayDocument = sheinmarketpub.OnWayDocument

func buildSheinRemoteRefreshState(pkg *SheinPackage, action, supplierCode string) sheinRemoteRefreshState {
	policy := sheinmarketpub.BuildRemoteConfirmationPolicy(action, sheinpub.RemotePublishAccepted(pkg, action))
	return sheinRemoteRefreshState{
		lookupCodes:      sheinpub.CollectRemoteLookupCodes(pkg, supplierCode),
		spuName:          sheinpub.RemoteLookupSPUName(pkg, action),
		defaultConfirmed: policy.DefaultConfirmed,
		fallbackMessage:  policy.RefreshFallbackMessage,
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
	policy := sheinmarketpub.BuildRemoteConfirmationPolicy(action, defaultConfirmed)
	remoteStatus := policy.MissingSupplierCodeStatus
	detail := policy.MissingSupplierCodeDetail
	setSheinSubmitRemoteRecord(pkg, action, requestID, remoteStatus, nil, now, detail)
	event := sheinpub.BuildSubmissionConfirmRemoteEvent(taskID, action, remoteStatus, requestID, startedAt, detail, nil)
	return &event
}

func (s *taskSubmissionRecoveryService) resolveRemoteStatus(productAPI sheinproduct.ProductAPI, otherAPI sheinother.OtherAPI, action, requestID string, lookupCodes []string, spuName string, defaultConfirmed bool, fallbackMessage string, startedAt time.Time, taskID string) (*sheinRemoteConfirmation, error) {
	if s.resolveRemoteStatusCallback == nil {
		return nil, errors.New("submit remote status resolution is not configured")
	}
	return s.resolveRemoteStatusCallback(productAPI, otherAPI, action, requestID, lookupCodes, spuName, defaultConfirmed, fallbackMessage, startedAt, taskID)
}

func resolveSheinSubmitRemoteStatus(productAPI sheinproduct.ProductAPI, otherAPI sheinother.OtherAPI, action, requestID string, lookupCodes []string, spuName string, defaultConfirmed bool, fallbackMessage string, startedAt time.Time, taskID string) (*sheinRemoteConfirmation, error) {
	policy := sheinmarketpub.BuildRemoteConfirmationPolicy(action, defaultConfirmed)
	fallbackMessage = policy.ResolveFallbackMessage
	if action == "publish" {
		if onWay, onWayErr := lookupSheinOnWayDocument(otherAPI, spuName); onWayErr == nil && onWay != nil {
			detail := fmt.Sprintf("SHEIN on-way document confirmed for spu_name=%s document_sn=%s", onWay.SpuName, onWay.DocumentSn)
			update := sheinpub.BuildSubmissionConfirmRemoteUpdate(taskID, action, sheinpub.SubmissionRemoteStatusConfirmed, requestID, startedAt, detail, nil)
			return newSheinRemoteConfirmation(update), nil
		}
	}
	item, recordErr := lookupSheinRemoteRecord(productAPI, lookupCodes, spuName)
	if recordErr == nil && item != nil {
		outcome := sheinmarketpub.ClassifyRemoteRecord(action, item, defaultConfirmed)
		update := sheinpub.BuildSubmissionConfirmRemoteUpdateForRecord(taskID, action, outcome.Status, requestID, startedAt, outcome.Detail, outcome.Err, item)
		return newSheinRemoteConfirmation(update), outcome.Err
	}
	if action == "publish" && spuName != "" {
		inventoryExists, inventoryErr := lookupSheinRemoteInventory(productAPI, spuName)
		if inventoryErr == nil && inventoryExists {
			detail := fmt.Sprintf("SHEIN remote inventory confirmed for spu_name=%s", spuName)
			update := sheinpub.BuildSubmissionConfirmRemoteUpdate(taskID, action, sheinpub.SubmissionRemoteStatusConfirmed, requestID, startedAt, detail, nil)
			return newSheinRemoteConfirmation(update), nil
		}
	}
	if recordErr != nil {
		update := sheinpub.BuildSubmissionConfirmRemoteUpdate(taskID, action, sheinpub.SubmissionRemoteStatusPending, requestID, startedAt, fallbackMessage, nil)
		update.Message = recordErr.Error()
		return newSheinRemoteConfirmation(update), nil
	}
	if defaultConfirmed {
		update := sheinpub.BuildSubmissionConfirmRemoteUpdate(taskID, action, sheinpub.SubmissionRemoteStatusConfirmed, requestID, startedAt, fallbackMessage, nil)
		return newSheinRemoteConfirmation(update), nil
	}
	update := sheinpub.BuildSubmissionConfirmRemoteUpdate(taskID, action, sheinpub.SubmissionRemoteStatusPending, requestID, startedAt, fallbackMessage, nil)
	update.Message = "record not found"
	return newSheinRemoteConfirmation(update), nil
}

type sheinRemoteConfirmation struct {
	remoteStatus string
	record       *sheinproduct.RecordItem
	checkedAt    time.Time
	message      string
	event        *sheinpub.SubmissionEvent
}

func newSheinRemoteConfirmation(update sheinpub.SubmissionConfirmRemoteUpdate) *sheinRemoteConfirmation {
	return &sheinRemoteConfirmation{
		remoteStatus: update.RemoteStatus,
		record:       update.Record,
		checkedAt:    update.CheckedAt,
		message:      update.Message,
		event:        update.Event,
	}
}

func lookupSheinOnWayDocument(otherAPI sheinother.OtherAPI, expectedSPUName string) (*sheinOnWayDocument, error) {
	if otherAPI == nil {
		return nil, nil
	}
	expectedSPUName = strings.TrimSpace(expectedSPUName)
	if expectedSPUName == "" {
		return nil, nil
	}
	resp, err := otherAPI.BatchCheckOnWay([]string{expectedSPUName})
	logSheinBatchCheckOnWayResponse(expectedSPUName, resp, err)
	if err != nil {
		return nil, err
	}
	return sheinmarketpub.SelectOnWayDocumentFromResponse(resp, expectedSPUName), nil
}

func logSheinBatchCheckOnWayResponse(expectedSPUName string, resp *sheinother.BatchCheckOnWayResponse, err error) {
	fields := logrus.Fields{
		"expected_spu_name": strings.TrimSpace(expectedSPUName),
	}
	if resp != nil {
		fields["response_code"] = strings.TrimSpace(resp.Code)
		fields["response_msg"] = strings.TrimSpace(resp.Msg)
		if encoded, marshalErr := json.Marshal(resp); marshalErr == nil {
			fields["response_json"] = string(encoded)
		} else {
			fields["response_json_error"] = marshalErr.Error()
		}
	}
	if err != nil {
		fields["error"] = err.Error()
		logrus.WithFields(fields).Warn("listingkit shein batch_check_on_way response error")
		return
	}
	logrus.WithFields(fields).Info("listingkit shein batch_check_on_way response")
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
	return sheinmarketpub.SelectRemoteRecordFromResponse(resp, expectedSPUName)
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
	return sheinmarketpub.InventoryConfirmed(resp), nil
}

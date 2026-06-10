package listingkit

import (
	"context"

	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"task-processor/internal/listingkit/submission"
	"time"

	sheinpub "task-processor/internal/publishing/shein"
	sheinother "task-processor/internal/shein/api/other"
	sheinproduct "task-processor/internal/shein/api/product"

	"github.com/sirupsen/logrus"
)

func (s *service) mutateTaskResult(ctx context.Context, taskID string, mutate TaskResultMutation) (*Task, error) {
	return s.taskSubmissionRecoveryOrDefault().mutateTaskResult(ctx, taskID, mutate)
}

func (s *service) RefreshSubmissionStatus(ctx context.Context, taskID string) (*ListingKitPreview, error) {
	return s.taskSubmissionOrDefault().RefreshSubmissionStatus(ctx, taskID)
}

func (s *service) resolveSheinSubmitRemoteStatus(productAPI sheinproduct.ProductAPI, otherAPI sheinother.OtherAPI, action, requestID string, lookupCodes []string, spuName string, defaultConfirmed bool, fallbackMessage string, startedAt time.Time, taskID string) (*sheinRemoteConfirmation, error) {
	if defaultConfirmed {
		fallbackMessage = "SHEIN accepted publish request; remote confirmation pending"
	}
	if action == "publish" {
		if onWay, onWayErr := lookupSheinOnWayDocument(otherAPI, spuName); onWayErr == nil && onWay != nil {
			detail := fmt.Sprintf("SHEIN on-way document confirmed for spu_name=%s document_sn=%s", onWay.SpuName, onWay.DocumentSn)
			parts := submission.BuildConfirmRemoteParts(taskID, action, sheinpub.SubmissionRemoteStatusConfirmed, requestID, startedAt, detail, nil)
			return newSheinRemoteConfirmation(parts), nil
		}
	}
	item, recordErr := lookupSheinRemoteRecord(productAPI, lookupCodes, spuName)
	if recordErr == nil && item != nil {
		remoteStatus, detail, remoteErr := classifySheinRemoteRecord(action, item, defaultConfirmed)
		parts := submission.BuildConfirmRemotePartsForRecord(taskID, action, remoteStatus, requestID, startedAt, detail, remoteErr, item)
		return newSheinRemoteConfirmation(parts), remoteErr
	}
	if action == "publish" {
		if spuName != "" {
			inventoryExists, inventoryErr := lookupSheinRemoteInventory(productAPI, spuName)
			if inventoryErr == nil && inventoryExists {
				detail := fmt.Sprintf("SHEIN remote inventory confirmed for spu_name=%s", spuName)
				parts := submission.BuildConfirmRemoteParts(taskID, action, sheinpub.SubmissionRemoteStatusConfirmed, requestID, startedAt, detail, nil)
				return newSheinRemoteConfirmation(parts), nil
			}
		}
	}
	if recordErr != nil {
		parts := submission.BuildConfirmRemoteParts(taskID, action, sheinpub.SubmissionRemoteStatusPending, requestID, startedAt, fallbackMessage, nil)
		parts.Message = recordErr.Error()
		return newSheinRemoteConfirmation(parts), nil
	}
	if defaultConfirmed {
		parts := submission.BuildConfirmRemoteParts(taskID, action, sheinpub.SubmissionRemoteStatusConfirmed, requestID, startedAt, fallbackMessage, nil)
		return newSheinRemoteConfirmation(parts), nil
	}
	parts := submission.BuildConfirmRemoteParts(taskID, action, sheinpub.SubmissionRemoteStatusPending, requestID, startedAt, fallbackMessage, nil)
	parts.Message = "record not found"
	return newSheinRemoteConfirmation(parts), nil
}

type sheinOnWayDocument struct {
	SpuName    string
	SkcName    string
	DocumentSn string
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
	if resp == nil || strings.TrimSpace(resp.Code) != "0" {
		return nil, nil
	}
	for _, item := range resp.Info {
		if strings.EqualFold(strings.TrimSpace(item.SpuName), expectedSPUName) && strings.TrimSpace(item.DocumentSn) != "" {
			return &sheinOnWayDocument{
				SpuName:    strings.TrimSpace(item.SpuName),
				SkcName:    strings.TrimSpace(item.SkcName),
				DocumentSn: strings.TrimSpace(item.DocumentSn),
			}, nil
		}
	}
	return nil, nil
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

func (s *service) taskSubmissionRecoveryOrDefault() *taskSubmissionRecoveryService {
	if s.submission.taskSubmissionRecovery != nil {
		return s.submission.taskSubmissionRecovery
	}
	s.submission.taskSubmissionRecovery = newTaskSubmissionRecoveryService(buildTaskSubmissionRecoveryServiceConfig(s))
	return s.submission.taskSubmissionRecovery
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
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.SubmissionState == nil {
		return ""
	}
	record := sheinSubmissionRecordForAction(pkg.SubmissionState, action)
	if record != nil && record.Result != nil {
		if value := strings.TrimSpace(record.Result.SPUName); value != "" {
			return value
		}
	}
	if pkg.SubmissionState.LastResult != nil {
		if value := strings.TrimSpace(pkg.SubmissionState.LastResult.SPUName); value != "" {
			return value
		}
	}
	return ""
}

func sheinRemotePublishAccepted(pkg *SheinPackage, action string) bool {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if action != "publish" || pkg == nil || pkg.SubmissionState == nil {
		return false
	}
	record := sheinSubmissionRecordForAction(pkg.SubmissionState, action)
	if sheinSubmissionResponseAcceptedWithSPU(recordResult(record)) {
		return true
	}
	return sheinSubmissionResponseAcceptedWithSPU(pkg.SubmissionState.LastResult)
}

func sheinSubmissionResponseAccepted(result *sheinpub.SubmissionResponse) bool {
	if result == nil {
		return false
	}
	return result.Success
}

func sheinSubmissionResponseAcceptedWithSPU(result *sheinpub.SubmissionResponse) bool {
	if !sheinSubmissionResponseAccepted(result) {
		return false
	}
	return strings.TrimSpace(result.SPUName) != ""
}

func recordResult(record *sheinpub.SubmissionRecord) *sheinpub.SubmissionResponse {
	if record == nil {
		return nil
	}
	return record.Result
}

func collectSheinRemoteLookupCodes(pkg *SheinPackage, supplierCode string) []string {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
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
	if pkg == nil || pkg.PreviewPayload == nil {
		return codes
	}
	appendCode(pkg.PreviewPayload.SupplierCode)
	for _, skc := range pkg.PreviewPayload.SKCList {
		if skc.SupplierCode != nil {
			appendCode(*skc.SupplierCode)
		}
		for _, sku := range skc.SKUS {
			appendCode(sku.SupplierSKU)
		}
	}
	return codes
}

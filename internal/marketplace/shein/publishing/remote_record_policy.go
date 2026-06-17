package publishing

import (
	"errors"
	"fmt"
	"strings"
	"time"

	sheinother "task-processor/internal/shein/api/other"
	sheinproduct "task-processor/internal/shein/api/product"
)

type RemoteRecordOutcome struct {
	Status string
	Detail string
	Err    error
}

type RemoteConfirmationPolicy struct {
	DefaultConfirmed          bool
	RefreshFallbackMessage    string
	ResolveFallbackMessage    string
	MissingSupplierCodeStatus string
	MissingSupplierCodeDetail string
}

type RemoteConfirmationResolution struct {
	DefaultConfirmed   bool
	FallbackMessage    string
	OnWayDocument      *OnWayDocument
	Record             *sheinproduct.RecordItem
	RecordErr          error
	InventoryConfirmed bool
	SPUName            string
}

type RemoteConfirmationDecision struct {
	Status string
	Detail string
	Err    error
}

const (
	RemoteRecordStatusConfirmed = "confirmed"
	RemoteRecordStatusPending   = "pending"
	RemoteRecordStatusFailed    = "failed"
)

type OnWayDocument struct {
	SpuName    string
	SkcName    string
	DocumentSn string
}

func BuildRemoteConfirmationPolicy(action string, publishAccepted bool) RemoteConfirmationPolicy {
	policy := RemoteConfirmationPolicy{
		DefaultConfirmed:          action == "publish" && publishAccepted,
		RefreshFallbackMessage:    "refreshing SHEIN remote record",
		ResolveFallbackMessage:    "refreshing SHEIN remote record",
		MissingSupplierCodeStatus: RemoteRecordStatusPending,
		MissingSupplierCodeDetail: "SHEIN submit succeeded, but supplier code is unavailable for remote confirmation",
	}
	if !policy.DefaultConfirmed {
		return policy
	}
	policy.RefreshFallbackMessage = "SHEIN accepted publish request; remote record not yet visible"
	policy.ResolveFallbackMessage = "SHEIN accepted publish request; remote confirmation pending"
	policy.MissingSupplierCodeStatus = RemoteRecordStatusConfirmed
	policy.MissingSupplierCodeDetail = "SHEIN accepted publish request, but supplier code is unavailable for remote confirmation"
	return policy
}

func ResolveRemoteConfirmationFallbackMessage(action string, defaultConfirmed bool, fallbackMessage string) string {
	if trimmed := strings.TrimSpace(fallbackMessage); trimmed != "" {
		return trimmed
	}
	return BuildRemoteConfirmationPolicy(action, defaultConfirmed).ResolveFallbackMessage
}

func ResolveRemoteRefreshFallbackMessage(action string, defaultConfirmed bool, fallbackMessage string) string {
	if trimmed := strings.TrimSpace(fallbackMessage); trimmed != "" {
		return trimmed
	}
	return BuildRemoteConfirmationPolicy(action, defaultConfirmed).RefreshFallbackMessage
}

func BuildMissingSupplierCodeDecision(action string, defaultConfirmed bool) RemoteConfirmationDecision {
	policy := BuildRemoteConfirmationPolicy(action, defaultConfirmed)
	return RemoteConfirmationDecision{
		Status: policy.MissingSupplierCodeStatus,
		Detail: policy.MissingSupplierCodeDetail,
	}
}

func ResolveRemoteConfirmationDecision(action string, resolution RemoteConfirmationResolution) RemoteConfirmationDecision {
	fallbackMessage := ResolveRemoteConfirmationFallbackMessage(action, resolution.DefaultConfirmed, resolution.FallbackMessage)
	if action == "publish" && resolution.OnWayDocument != nil {
		return RemoteConfirmationDecision{
			Status: RemoteRecordStatusConfirmed,
			Detail: fmt.Sprintf(
				"SHEIN on-way document confirmed for spu_name=%s document_sn=%s",
				resolution.OnWayDocument.SpuName,
				resolution.OnWayDocument.DocumentSn,
			),
		}
	}
	if resolution.RecordErr == nil && resolution.Record != nil {
		outcome := ClassifyRemoteRecord(action, resolution.Record, resolution.DefaultConfirmed)
		return RemoteConfirmationDecision{
			Status: outcome.Status,
			Detail: outcome.Detail,
			Err:    outcome.Err,
		}
	}
	if action == "publish" && resolution.InventoryConfirmed {
		return RemoteConfirmationDecision{
			Status: RemoteRecordStatusConfirmed,
			Detail: fmt.Sprintf("SHEIN remote inventory confirmed for spu_name=%s", strings.TrimSpace(resolution.SPUName)),
		}
	}
	if resolution.RecordErr != nil {
		return RemoteConfirmationDecision{
			Status: RemoteRecordStatusPending,
			Detail: fallbackMessage,
		}
	}
	if resolution.DefaultConfirmed {
		return RemoteConfirmationDecision{
			Status: RemoteRecordStatusConfirmed,
			Detail: fallbackMessage,
		}
	}
	return RemoteConfirmationDecision{
		Status: RemoteRecordStatusPending,
		Detail: fallbackMessage,
	}
}

func ResolveRemoteConfirmationUpdateMessage(decision RemoteConfirmationDecision, resolution RemoteConfirmationResolution) string {
	if resolution.RecordErr != nil {
		return resolution.RecordErr.Error()
	}
	if decision.Status == RemoteRecordStatusPending && resolution.Record == nil && !resolution.DefaultConfirmed {
		return "record not found"
	}
	return decision.Detail
}

func ClassifyRemoteRecord(action string, item *sheinproduct.RecordItem, publishAccepted bool) RemoteRecordOutcome {
	if item == nil {
		return RemoteRecordOutcome{
			Status: RemoteRecordStatusPending,
			Detail: "record not found",
		}
	}
	if action == "save_draft" {
		return RemoteRecordOutcome{
			Status: RemoteRecordStatusConfirmed,
			Detail: "SHEIN draft record confirmed",
		}
	}
	if publishAccepted {
		return RemoteRecordOutcome{
			Status: RemoteRecordStatusConfirmed,
			Detail: fmt.Sprintf("SHEIN publish API reported success (state=%d audit_state=%d)", item.State, item.AuditState),
		}
	}
	if remoteRecordLooksDraft(item) {
		message := fmt.Sprintf("SHEIN publish landed in draft state (state=%d audit_state=%d)", item.State, item.AuditState)
		return RemoteRecordOutcome{
			Status: RemoteRecordStatusFailed,
			Detail: message,
			Err:    errors.New(message),
		}
	}
	if remoteRecordLooksConfirmed(item) {
		return RemoteRecordOutcome{
			Status: RemoteRecordStatusConfirmed,
			Detail: "SHEIN remote record confirmed",
		}
	}
	return RemoteRecordOutcome{
		Status: RemoteRecordStatusPending,
		Detail: fmt.Sprintf("SHEIN remote record is not yet publish-confirmed (state=%d audit_state=%d)", item.State, item.AuditState),
	}
}

func remoteRecordLooksDraft(item *sheinproduct.RecordItem) bool {
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

func remoteRecordLooksConfirmed(item *sheinproduct.RecordItem) bool {
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

func SelectRemoteRecord(records []sheinproduct.RecordItem, expectedSPUName string) *sheinproduct.RecordItem {
	if len(records) == 0 {
		return nil
	}
	if expectedSPUName = strings.TrimSpace(expectedSPUName); expectedSPUName != "" {
		for i := range records {
			if strings.EqualFold(strings.TrimSpace(records[i].SpuName), expectedSPUName) {
				return &records[i]
			}
		}
	}
	best := records[0]
	bestTime := ParseRemoteRecordTime(best.CreateTime)
	for i := 1; i < len(records); i++ {
		candidate := records[i]
		candidateTime := ParseRemoteRecordTime(candidate.CreateTime)
		if candidateTime.After(bestTime) {
			best = candidate
			bestTime = candidateTime
		}
	}
	return &best
}

func ParseRemoteRecordTime(value string) time.Time {
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

func SelectOnWayDocument(items []struct {
	SpuName    string `json:"spu_name"`
	SkcName    string `json:"skc_name"`
	DocumentSn string `json:"document_sn"`
}, expectedSPUName string) *OnWayDocument {
	expectedSPUName = strings.TrimSpace(expectedSPUName)
	if expectedSPUName == "" {
		return nil
	}
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.SpuName), expectedSPUName) && strings.TrimSpace(item.DocumentSn) != "" {
			return &OnWayDocument{
				SpuName:    strings.TrimSpace(item.SpuName),
				SkcName:    strings.TrimSpace(item.SkcName),
				DocumentSn: strings.TrimSpace(item.DocumentSn),
			}
		}
	}
	return nil
}

func SelectOnWayDocumentFromResponse(resp *sheinother.BatchCheckOnWayResponse, expectedSPUName string) *OnWayDocument {
	if resp == nil || strings.TrimSpace(resp.Code) != "0" {
		return nil
	}
	return SelectOnWayDocument(resp.Info, expectedSPUName)
}

func SelectRemoteRecordFromResponse(resp *sheinproduct.RecordResponse, expectedSPUName string) (*sheinproduct.RecordItem, error) {
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
	return SelectRemoteRecord(resp.Info.Data, expectedSPUName), nil
}

func InventoryConfirmed(resp *sheinproduct.InventoryQueryResponse) bool {
	if resp == nil || strings.TrimSpace(resp.Code) != "0" {
		return false
	}
	return strings.TrimSpace(resp.Info.SpuName) != ""
}

func ResponseAcceptedWithSPU(success bool, spuName string) bool {
	return success && strings.TrimSpace(spuName) != ""
}

func ResponseAcceptedForAction(action string, success bool, code string) bool {
	if success {
		return true
	}
	return strings.TrimSpace(action) == "save_draft" && strings.TrimSpace(code) == "0"
}

func ConfirmedSubmissionMessage(action string) string {
	if strings.TrimSpace(action) == "save_draft" {
		return "save draft confirmed by remote check"
	}
	return "publish confirmed by remote check"
}

func ResolveRemoteLookupSPUName(recordSPUName, lastSPUName string) string {
	if value := strings.TrimSpace(recordSPUName); value != "" {
		return value
	}
	return strings.TrimSpace(lastSPUName)
}

func ResolveRemoteResolutionSPUName(onWay *OnWayDocument, record *sheinproduct.RecordItem, fallbackSPUName string) string {
	if onWay != nil {
		if value := strings.TrimSpace(onWay.SpuName); value != "" {
			return value
		}
	}
	if record != nil {
		if value := strings.TrimSpace(record.SpuName); value != "" {
			return value
		}
	}
	return strings.TrimSpace(fallbackSPUName)
}

func RemotePublishAccepted(action string, recordAccepted, lastAccepted bool) bool {
	return strings.TrimSpace(action) == "publish" && (recordAccepted || lastAccepted)
}

func CollectRemoteLookupCodes(rootSupplierCode, previewSupplierCode string, skcSupplierCodes, skuSupplierCodes []string) []string {
	seen := make(map[string]struct{})
	codes := make([]string, 0, 2+len(skcSupplierCodes)+len(skuSupplierCodes))
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

	appendCode(rootSupplierCode)
	appendCode(previewSupplierCode)
	for _, value := range skcSupplierCodes {
		appendCode(value)
	}
	for _, value := range skuSupplierCodes {
		appendCode(value)
	}
	return codes
}

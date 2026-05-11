package shein

import (
	"time"

	sheinproduct "task-processor/internal/shein/api/product"
)

type SubmissionReport struct {
	LastAction        string              `json:"last_action,omitempty"`
	LastStatus        string              `json:"last_status,omitempty"`
	LastError         string              `json:"last_error,omitempty"`
	SubmittedAt       *time.Time          `json:"submitted_at,omitempty"`
	SaveDraft         *SubmissionRecord   `json:"save_draft,omitempty"`
	Publish           *SubmissionRecord   `json:"publish,omitempty"`
	LastResult        *SubmissionResponse `json:"last_result,omitempty"`
	CurrentAction     string              `json:"current_action,omitempty"`
	CurrentPhase      string              `json:"current_phase,omitempty"`
	CurrentRequestID  string              `json:"current_request_id,omitempty"`
	InFlightStartedAt *time.Time          `json:"in_flight_started_at,omitempty"`
	LeaseExpiresAt    *time.Time          `json:"lease_expires_at,omitempty"`
	RemoteStatus      string              `json:"remote_status,omitempty"`
	RemoteCheckedAt   *time.Time          `json:"remote_checked_at,omitempty"`
	AttemptCount      int                 `json:"attempt_count,omitempty"`
}

type SubmissionRecord struct {
	Action           string              `json:"action,omitempty"`
	Status           string              `json:"status,omitempty"`
	Error            string              `json:"error,omitempty"`
	SubmittedAt      time.Time           `json:"submitted_at"`
	Result           *SubmissionResponse `json:"result,omitempty"`
	SubmitSnapshot   *SubmitSnapshot     `json:"submit_snapshot,omitempty"`
	RequestID        string              `json:"request_id,omitempty"`
	Phase            string              `json:"phase,omitempty"`
	StartedAt        time.Time           `json:"started_at,omitempty"`
	FinishedAt       *time.Time          `json:"finished_at,omitempty"`
	Attempt          int                 `json:"attempt,omitempty"`
	SupplierCode     string              `json:"supplier_code,omitempty"`
	RemoteRecordID   string              `json:"remote_record_id,omitempty"`
	RemoteState      int                 `json:"remote_state,omitempty"`
	RemoteAuditState int                 `json:"remote_audit_state,omitempty"`
	RemoteMessage    string              `json:"remote_message,omitempty"`
	RemoteCheckedAt  *time.Time          `json:"remote_checked_at,omitempty"`
}

const (
	SubmissionPhaseValidate       = "validate"
	SubmissionPhasePrepareProduct = "prepare_product"
	SubmissionPhaseUploadImages   = "upload_images"
	SubmissionPhasePreValidate    = "pre_validate"
	SubmissionPhaseSubmitRemote   = "submit_remote"
	SubmissionPhasePersistResult  = "persist_result"
	SubmissionPhaseConfirmRemote  = "confirm_remote"

	SubmissionStatusRunning = "running"
	SubmissionStatusSuccess = "success"
	SubmissionStatusFailed  = "failed"
	SubmissionStatusBlocked = "blocked"

	SubmissionRemoteStatusConfirmed = "confirmed"
	SubmissionRemoteStatusPending   = "pending"
	SubmissionRemoteStatusFailed    = "failed"
)

type SubmissionEvent struct {
	ID              string              `json:"id,omitempty"`
	TaskID          string              `json:"task_id,omitempty"`
	Platform        string              `json:"platform,omitempty"`
	Action          string              `json:"action,omitempty"`
	Phase           string              `json:"phase,omitempty"`
	Status          string              `json:"status,omitempty"`
	RequestID       string              `json:"request_id,omitempty"`
	StartedAt       time.Time           `json:"started_at"`
	FinishedAt      *time.Time          `json:"finished_at,omitempty"`
	Detail          string              `json:"detail,omitempty"`
	RemoteRecordID  string              `json:"remote_record_id,omitempty"`
	ErrorMessage    string              `json:"error_message,omitempty"`
	ValidationNotes []string            `json:"validation_notes,omitempty"`
	Response        *SubmissionResponse `json:"response,omitempty"`
}

type SubmissionResponse struct {
	Code            string   `json:"code,omitempty"`
	Message         string   `json:"message,omitempty"`
	Success         bool     `json:"success"`
	SPUName         string   `json:"spu_name,omitempty"`
	Version         string   `json:"version,omitempty"`
	ValidationNotes []string `json:"validation_notes,omitempty"`
}

type SubmitSnapshot struct {
	SPUName               string              `json:"spu_name,omitempty"`
	SupplierCode          string              `json:"supplier_code,omitempty"`
	MultiLanguageNameList []LocalizedText     `json:"multi_language_name_list,omitempty"`
	MultiLanguageDescList []LocalizedText     `json:"multi_language_desc_list,omitempty"`
	SKCList               []SubmitSKCSnapshot `json:"skc_list,omitempty"`
	ImageCount            int                 `json:"image_count,omitempty"`
}

type SubmitSKCSnapshot struct {
	SupplierCode          string          `json:"supplier_code,omitempty"`
	PrimaryName           string          `json:"primary_name,omitempty"`
	MultiLanguageNameList []LocalizedText `json:"multi_language_name_list,omitempty"`
}

func BuildSubmissionResponseSummary(resp *sheinproduct.SheinResponse) *SubmissionResponse {
	if resp == nil {
		return nil
	}
	summary := &SubmissionResponse{
		Code:    resp.Code,
		Message: resp.Msg,
	}
	summary.Success = resp.Info.Success
	summary.SPUName = resp.Info.SPUName
	summary.Version = resp.Info.Version
	for _, result := range resp.Info.PreValidResult {
		summary.ValidationNotes = append(summary.ValidationNotes, result.Messages...)
	}
	summary.ValidationNotes = uniqueSubmissionStrings(summary.ValidationNotes)
	return summary
}

func uniqueSubmissionStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

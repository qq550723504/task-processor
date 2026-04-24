package shein

import (
	"time"

	sheinproduct "task-processor/internal/shein/api/product"
)

type SubmissionReport struct {
	LastAction  string              `json:"last_action,omitempty"`
	LastStatus  string              `json:"last_status,omitempty"`
	LastError   string              `json:"last_error,omitempty"`
	SubmittedAt *time.Time          `json:"submitted_at,omitempty"`
	SaveDraft   *SubmissionRecord   `json:"save_draft,omitempty"`
	Publish     *SubmissionRecord   `json:"publish,omitempty"`
	LastResult  *SubmissionResponse `json:"last_result,omitempty"`
}

type SubmissionRecord struct {
	Action      string              `json:"action,omitempty"`
	Status      string              `json:"status,omitempty"`
	Error       string              `json:"error,omitempty"`
	SubmittedAt time.Time           `json:"submitted_at"`
	Result      *SubmissionResponse `json:"result,omitempty"`
}

type SubmissionResponse struct {
	Code            string   `json:"code,omitempty"`
	Message         string   `json:"message,omitempty"`
	Success         bool     `json:"success"`
	SPUName         string   `json:"spu_name,omitempty"`
	Version         string   `json:"version,omitempty"`
	ValidationNotes []string `json:"validation_notes,omitempty"`
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

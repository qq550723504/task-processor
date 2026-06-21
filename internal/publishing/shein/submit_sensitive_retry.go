package shein

import (
	"context"
	"time"

	listingsubmission "task-processor/internal/listing/submission"
	sheinproduct "task-processor/internal/shein/api/product"
)

const sensitiveWordRetryDetail = "检测到敏感词，已自动清理并重试提交"

// SubmitRemoteExecutor executes a SHEIN submit action against the remote API.
type SubmitRemoteExecutor func(sheinproduct.ProductAPI, string, *sheinproduct.Product) (*SubmissionResponse, error)

// RetrySensitiveWordSubmit cleans validation-reported sensitive words and retries a failed publish submit.
func RetrySensitiveWordSubmit(ctx context.Context, taskID string, pkg *Package, action string, requestID string, productAPI sheinproduct.ProductAPI, submitProduct *sheinproduct.Product, response *SubmissionResponse, responseErr error, execute SubmitRemoteExecutor) (*SubmissionResponse, error, bool) {
	if action != listingsubmission.SubmitActionPublish || response == nil || responseErr == nil || len(response.ValidationNotes) == 0 || execute == nil {
		return response, responseErr, false
	}
	if !RetrySensitiveWordCleanup(ctx, submitProduct, response.ValidationNotes) {
		return response, responseErr, false
	}

	AppendSubmissionEvent(pkg, BuildSensitiveWordRetryEvent(taskID, action, requestID, time.Now()))
	retryResponse, retryErr := execute(productAPI, action, submitProduct)
	if retryErr == nil {
		retryErr = listingsubmission.BuildResponseError("SHEIN", action, SubmissionResponseOutcome(retryResponse))
	}
	return retryResponse, retryErr, true
}

// BuildSensitiveWordRetryEvent creates the audit event emitted before retrying a cleaned submit payload.
func BuildSensitiveWordRetryEvent(taskID, action, requestID string, startedAt time.Time) SubmissionEvent {
	finishedAt := time.Now()
	draft := listingsubmission.BuildPhaseEventDraft(
		SubmissionStatusRunning,
		sensitiveWordRetryDetail,
		"",
		nil,
		finishedAt,
	)
	return SubmissionEvent{
		TaskID:       taskID,
		Platform:     "shein",
		Action:       "submit_phase",
		Phase:        SubmissionPhaseSubmitRemote,
		Status:       draft.Status,
		RequestID:    requestID,
		StartedAt:    startedAt,
		FinishedAt:   &draft.FinishedAt,
		Detail:       draft.Detail,
		ErrorMessage: draft.ErrorMessage,
	}
}

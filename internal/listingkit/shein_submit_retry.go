package listingkit

import (
	"context"
	"time"

	submissiondomain "task-processor/internal/listing/submission"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func (s *service) retrySheinSensitiveWordSubmit(ctx context.Context, taskID string, pkg *SheinPackage, action string, requestID string, productAPI sheinproduct.ProductAPI, submitProduct *sheinproduct.Product, response *sheinpub.SubmissionResponse, responseErr error) (*sheinpub.SubmissionResponse, error, bool) {
	if action != "publish" || response == nil || responseErr == nil || len(response.ValidationNotes) == 0 {
		return response, responseErr, false
	}
	if !sheinpub.RetrySensitiveWordCleanup(ctx, submitProduct, response.ValidationNotes) {
		return response, responseErr, false
	}

	sheinpub.AppendSubmissionEvent(pkg, buildSheinSensitiveWordRetryEvent(taskID, action, requestID, time.Now()))
	retryResponse, retryErr := s.taskSubmissionExecutionOrDefault().executeSheinSubmitRemote(productAPI, action, submitProduct)
	if retryErr == nil {
		retryErr = submissiondomain.BuildResponseError("SHEIN", action, sheinpub.SubmissionResponseOutcome(retryResponse))
	}
	return retryResponse, retryErr, true
}

func buildSheinSensitiveWordRetryEvent(taskID, action, requestID string, startedAt time.Time) sheinpub.SubmissionEvent {
	finishedAt := time.Now()
	draft := submissiondomain.BuildPhaseEventDraft(
		sheinpub.SubmissionStatusRunning,
		"检测到敏感词，已自动清理并重试提交",
		"",
		nil,
		finishedAt,
	)
	return sheinpub.SubmissionEvent{
		TaskID:       taskID,
		Platform:     "shein",
		Action:       "submit_phase",
		Phase:        sheinpub.SubmissionPhaseSubmitRemote,
		Status:       draft.Status,
		RequestID:    requestID,
		StartedAt:    startedAt,
		FinishedAt:   &draft.FinishedAt,
		Detail:       draft.Detail,
		ErrorMessage: draft.ErrorMessage,
	}
}

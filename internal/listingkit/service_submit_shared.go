package listingkit

import (
	"fmt"
	"time"

	listingsubmission "task-processor/internal/listing/submission"
)

func normalizedSubmitIdempotencyKey(req *SubmitTaskRequest) string {
	if req == nil {
		return ""
	}
	return listingsubmission.ResolveSubmitRequestID(req.IdempotencyKey, req.RequestID)
}

func derivedSheinSubmitRequestID(taskID, action string, requestedAt time.Time) string {
	return listingsubmission.DeriveWorkflowRequestID(taskID, action, requestedAt)
}

func isSupportedSubmitAction(action string) bool {
	return listingsubmission.IsSupportedSubmitAction(action)
}

func unsupportedSubmitActionError(action string) error {
	return fmt.Errorf("unsupported submit action: %s", action)
}

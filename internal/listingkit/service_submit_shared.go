package listingkit

import listingsubmission "task-processor/internal/listing/submission"

func normalizedSubmitIdempotencyKey(req *SubmitTaskRequest) string {
	if req == nil {
		return ""
	}
	return listingsubmission.ResolveSubmitRequestID(req.IdempotencyKey, req.RequestID)
}

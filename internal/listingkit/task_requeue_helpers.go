package listingkit

import (
	"time"

	listingsubmission "task-processor/internal/listing/submission"
)

func submitTaskWithRetry(submitter TaskSubmitter, taskID string, maxWait time.Duration) error {
	if submitter == nil {
		return ErrTaskRequeueUnavailable
	}
	return listingsubmission.RetryEnqueueSubmit(taskID, maxWait, submitter.Submit)
}

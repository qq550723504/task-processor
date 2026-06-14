package submission

import (
	"time"

	listingsubmission "task-processor/internal/listing/submission"
)

const (
	EnqueueRetryDelay    = listingsubmission.EnqueueRetryDelay
	EnqueueRetryMaxDelay = listingsubmission.EnqueueRetryMaxDelay
)

func RetryEnqueueSubmit(taskID string, maxWait time.Duration, submit func(string) error) error {
	return listingsubmission.RetryEnqueueSubmit(taskID, maxWait, submit)
}

func NextEnqueueRetryDelay(delay time.Duration) time.Duration {
	return listingsubmission.NextEnqueueRetryDelay(delay)
}

func BoundedEnqueueRetryDelay(attempt int) time.Duration {
	return listingsubmission.BoundedEnqueueRetryDelay(attempt)
}

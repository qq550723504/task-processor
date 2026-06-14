package submission

import listingsubmission "task-processor/internal/listing/submission"

type SubmitLockManager = listingsubmission.SubmitLockManager

func NewSubmitLockManager() *SubmitLockManager {
	return listingsubmission.NewSubmitLockManager()
}

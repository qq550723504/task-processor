package submission

import listingsubmission "task-processor/internal/listing/submission"

func SourceFactsReady(metadata map[string]string) (bool, string) {
	return listingsubmission.SourceFactsReady(metadata)
}

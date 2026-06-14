package listingkit

import (
	"context"

	submissiondomain "task-processor/internal/listing/submission"
)

func newSheinSubmissionFailurePersistenceService(
	recordFailure func(context.Context, submissiondomain.FailurePersistenceInput[*ListingKitResult, *SheinPackage]) error,
) *submissiondomain.FailurePersistenceService[*ListingKitResult, *SheinPackage] {
	return submissiondomain.NewFailurePersistenceService(submissiondomain.FailurePersistenceServiceConfig[*ListingKitResult, *SheinPackage]{
		RecordFailure: recordFailure,
	})
}

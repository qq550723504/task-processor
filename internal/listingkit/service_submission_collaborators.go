package listingkit

import listingsubmission "task-processor/internal/listing/submission"

type submissionCollaborators struct {
	recoveryGroup taskSubmitTaskRecoveryCollaborators
	coreGroup     taskSubmissionCoreCollaborators
	managedGroup  taskManagedSubmissionCollaborators
	temporalGroup taskTemporalSubmissionCollaborators

	// Shared submit coordination primitives.
	sheinSubmitLocks *listingsubmission.SubmitLockManager
}

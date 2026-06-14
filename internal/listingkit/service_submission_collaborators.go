package listingkit

import listingsubmission "task-processor/internal/listing/submission"

type submissionCollaborators struct {
	// Task-level retry/recovery collaborators.
	taskRecovery *taskRecoveryService
	taskRequeue  *taskRequeueService

	// Submission state and execution collaborators.
	taskSubmissionState     *taskSubmissionStateService
	taskSubmissionExecution *taskSubmissionExecutionService

	// SHEIN submission orchestrators.
	taskSubmissionRecovery *taskSubmissionRecoveryService
	taskDirectSubmission   *taskDirectSubmissionService
	taskSubmission         *taskSubmissionService
	taskSubmissionRefresh  *taskSubmissionRefreshService

	// Workflow-facing collaborators.
	taskTemporalSubmission            *taskTemporalSubmissionService
	taskTemporalSubmissionLifecycle   *taskTemporalSubmissionLifecycleService
	taskTemporalSubmissionFlow        *taskTemporalSubmissionFlowService
	taskTemporalSubmissionPersistence *taskTemporalSubmissionPersistenceService
	taskTemporalSubmissionRefresh     *taskTemporalSubmissionRefreshService

	// Shared submit coordination primitives.
	sheinSubmitLocks *listingsubmission.SubmitLockManager
}

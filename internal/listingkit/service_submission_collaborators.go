package listingkit

import "task-processor/internal/listingkit/submission"

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

	// Workflow-facing adapter.
	taskTemporalSubmissionAdapter *taskTemporalSubmissionAdapter

	// Shared submit coordination primitives.
	sheinSubmitLocks *submission.SubmitLockManager
}

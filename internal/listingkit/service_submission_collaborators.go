package listingkit

import "task-processor/internal/listingkit/submission"

type submissionCollaborators struct {
	taskRecovery                  *taskRecoveryService
	taskRequeue                   *taskRequeueService
	taskSubmission                *taskSubmissionService
	taskSubmissionRecovery        *taskSubmissionRecoveryService
	taskSubmissionExecution       *taskSubmissionExecutionService
	taskSubmissionState           *taskSubmissionStateService
	taskDirectSubmission          *taskDirectSubmissionService
	taskTemporalSubmissionAdapter *taskTemporalSubmissionAdapter
	sheinSubmitLocks              *submission.SubmitLockManager
}

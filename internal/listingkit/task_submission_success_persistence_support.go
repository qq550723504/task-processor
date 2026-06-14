package listingkit

import (
	"context"
	"time"

	submissiondomain "task-processor/internal/listing/submission"
	sheinpub "task-processor/internal/publishing/shein"
)

func newSheinSubmissionSuccessPersistenceService(
	completeAttempt func(submissiondomain.SuccessPersistenceInput[*Task, *SheinPackage, *sheinpub.SubmissionResponse], time.Time),
	persistResultAndPhase func(context.Context, submissiondomain.SuccessPersistenceInput[*Task, *SheinPackage, *sheinpub.SubmissionResponse]) error,
	rememberSubmitted func(*Task, string),
	persistSuccessfulSubmission func(context.Context, string, *Task, string) error,
) *submissiondomain.SuccessPersistenceService[*Task, *SheinPackage, *sheinpub.SubmissionResponse] {
	return submissiondomain.NewSuccessPersistenceService(submissiondomain.SuccessPersistenceServiceConfig[*Task, *SheinPackage, *sheinpub.SubmissionResponse]{
		PersistResultAndPhase:       persistResultAndPhase,
		CompleteAttempt:             completeAttempt,
		RememberSubmitted:           rememberSubmitted,
		PersistSuccessfulSubmission: persistSuccessfulSubmission,
		CurrentTime:                 time.Now,
	})
}

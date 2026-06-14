package submission

import (
	"time"

	listingsubmission "task-processor/internal/listing/submission"
	sheinpub "task-processor/internal/publishing/shein"
)

func BeginAttemptAndBuildEvent(pkg *sheinpub.Package, taskID, action, requestID, phase string, startedAt time.Time, ttl time.Duration) (*sheinpub.SubmissionRecord, sheinpub.SubmissionEvent) {
	record := BeginAttempt(pkg, action, requestID, phase, startedAt, ttl)
	return record, BuildPhaseEvent(taskID, action, phase, sheinpub.SubmissionStatusRunning, requestID, startedAt, "", nil)
}

func AdvancePhaseAndBuildEvent(pkg *sheinpub.Package, taskID, action, requestID, phase string, now time.Time, ttl time.Duration) sheinpub.SubmissionEvent {
	AdvancePhase(pkg, action, requestID, phase, now, ttl)
	return BuildPhaseEvent(taskID, action, phase, sheinpub.SubmissionStatusRunning, requestID, now, "", nil)
}

func CompleteAttemptAndBuildEvent(pkg *sheinpub.Package, taskID, action, requestID string, response *sheinpub.SubmissionResponse, responseErr error, startedAt, finishedAt time.Time) (*sheinpub.SubmissionRecord, sheinpub.SubmissionEvent) {
	record := CompleteAttempt(pkg, action, requestID, response, responseErr, finishedAt)
	return record, BuildEvent(taskID, action, record, response, responseErr, startedAt)
}

func ResolveFailureState(pkg *sheinpub.Package, requestedID, phase string) (string, string) {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	var currentID, currentPhase string
	if pkg != nil && pkg.SubmissionState != nil {
		currentID = pkg.SubmissionState.CurrentRequestID
		currentPhase = pkg.SubmissionState.CurrentPhase
	}
	return listingsubmission.ResolveFailureState(requestedID, phase, currentID, currentPhase, sheinpub.SubmissionPhaseValidate)
}

func FailAttemptAndBuildEvent(pkg *sheinpub.Package, taskID, action, requestedID, phase string, submitErr error, finishedAt time.Time) (*sheinpub.SubmissionRecord, sheinpub.SubmissionEvent) {
	requestID, resolvedPhase := ResolveFailureState(pkg, requestedID, phase)
	record := FailAttempt(pkg, action, requestID, resolvedPhase, submitErr, finishedAt)
	startedAt := record.SubmittedAt
	if !record.StartedAt.IsZero() {
		startedAt = record.StartedAt
	}
	return record, BuildEvent(taskID, action, record, nil, submitErr, startedAt)
}

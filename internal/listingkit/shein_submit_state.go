package listingkit

import (
	"time"

	sheinpub "task-processor/internal/publishing/shein"
)

func beginSheinSubmitAttempt(pkg *SheinPackage, action, requestID, phase string, startedAt time.Time) *sheinpub.SubmissionRecord {
	return sheinpub.BeginSubmitAttempt(pkg, action, requestID, phase, startedAt, sheinSubmitInFlightTTL)
}

func advanceSheinSubmitPhase(pkg *SheinPackage, action, requestID, phase string) {
	advanceSheinSubmitPhaseAt(pkg, action, requestID, phase, time.Now())
}

func advanceSheinSubmitPhaseAndBuildEvent(pkg *SheinPackage, taskID, action, requestID, phase string, now time.Time) sheinpub.SubmissionEvent {
	return sheinpub.AdvanceSubmitPhaseAndBuildEvent(pkg, taskID, action, requestID, phase, now, sheinSubmitInFlightTTL)
}

func completeSheinSubmitAttempt(pkg *SheinPackage, action, requestID string, response *sheinpub.SubmissionResponse, submitErr error, finishedAt time.Time) *sheinpub.SubmissionRecord {
	return completeSheinSubmitAttemptAt(pkg, action, requestID, response, submitErr, finishedAt)
}

func completeSheinSubmitAttemptAndBuildEvent(pkg *SheinPackage, taskID, action, requestID string, response *sheinpub.SubmissionResponse, responseErr error, startedAt, finishedAt time.Time) (*sheinpub.SubmissionRecord, sheinpub.SubmissionEvent) {
	return sheinpub.CompleteSubmitAttemptAndBuildEvent(pkg, taskID, action, requestID, response, responseErr, startedAt, finishedAt)
}

func failSheinSubmitAttempt(pkg *SheinPackage, action, requestID, phase string, submitErr error, finishedAt time.Time) *sheinpub.SubmissionRecord {
	return failSheinSubmitAttemptAt(pkg, action, requestID, phase, submitErr, finishedAt)
}

func failSheinSubmitAttemptAndBuildEvent(pkg *SheinPackage, taskID, action, requestedID, phase string, submitErr error, finishedAt time.Time) (*sheinpub.SubmissionRecord, sheinpub.SubmissionEvent) {
	return sheinpub.FailSubmitAttemptAndBuildEvent(pkg, taskID, action, requestedID, phase, submitErr, finishedAt)
}

func failSheinSubmitAttemptWithResponseAndBuildEvent(pkg *SheinPackage, taskID, action, requestedID, phase string, response *sheinpub.SubmissionResponse, submitErr error, finishedAt time.Time) (*sheinpub.SubmissionRecord, sheinpub.SubmissionEvent) {
	return sheinpub.FailSubmitAttemptWithResponseAndBuildEvent(pkg, taskID, action, requestedID, phase, response, submitErr, finishedAt)
}

func advanceSheinSubmitPhaseAt(pkg *SheinPackage, action, requestID, phase string, now time.Time) {
	sheinpub.AdvanceSubmitPhaseAt(pkg, action, requestID, phase, now, sheinSubmitInFlightTTL)
}

func completeSheinSubmitAttemptAt(pkg *SheinPackage, action, requestID string, response *sheinpub.SubmissionResponse, submitErr error, finishedAt time.Time) *sheinpub.SubmissionRecord {
	return sheinpub.CompleteSubmitAttemptAt(pkg, action, requestID, response, submitErr, finishedAt)
}

func failSheinSubmitAttemptAt(pkg *SheinPackage, action, requestID, phase string, submitErr error, finishedAt time.Time) *sheinpub.SubmissionRecord {
	return sheinpub.FailSubmitAttemptAt(pkg, action, requestID, phase, submitErr, finishedAt)
}

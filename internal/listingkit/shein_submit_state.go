package listingkit

import (
	"time"

	listingsubmission "task-processor/internal/listing/submission"
	"task-processor/internal/listingkit/submission"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func beginSheinSubmitAttempt(pkg *SheinPackage, action, requestID, phase string, startedAt time.Time) *sheinpub.SubmissionRecord {
	return submission.BeginAttempt(pkg, action, requestID, phase, startedAt, sheinSubmitInFlightTTL)
}

func advanceSheinSubmitPhase(pkg *SheinPackage, action, requestID, phase string) {
	submission.AdvancePhase(pkg, action, requestID, phase, time.Now(), sheinSubmitInFlightTTL)
}

func advanceSheinSubmitPhaseAndBuildEvent(pkg *SheinPackage, taskID, action, requestID, phase string, now time.Time) sheinpub.SubmissionEvent {
	submission.AdvancePhase(pkg, action, requestID, phase, now, sheinSubmitInFlightTTL)
	return sheinpub.BuildSubmissionPhaseEvent(taskID, action, phase, sheinpub.SubmissionStatusRunning, requestID, now, "", nil)
}

func completeSheinSubmitAttempt(pkg *SheinPackage, action, requestID string, response *sheinpub.SubmissionResponse, submitErr error, finishedAt time.Time) *sheinpub.SubmissionRecord {
	return submission.CompleteAttempt(pkg, action, requestID, response, submitErr, finishedAt)
}

func completeSheinSubmitAttemptAndBuildEvent(pkg *SheinPackage, taskID, action, requestID string, response *sheinpub.SubmissionResponse, responseErr error, startedAt, finishedAt time.Time) (*sheinpub.SubmissionRecord, sheinpub.SubmissionEvent) {
	record := submission.CompleteAttempt(pkg, action, requestID, response, responseErr, finishedAt)
	return record, sheinpub.BuildSubmissionAttemptEvent(taskID, action, record, response, responseErr, startedAt)
}

func failSheinSubmitAttempt(pkg *SheinPackage, action, requestID, phase string, submitErr error, finishedAt time.Time) *sheinpub.SubmissionRecord {
	return submission.FailAttempt(pkg, action, requestID, phase, submitErr, finishedAt)
}

func failSheinSubmitAttemptAndBuildEvent(pkg *SheinPackage, taskID, action, requestedID, phase string, submitErr error, finishedAt time.Time) (*sheinpub.SubmissionRecord, sheinpub.SubmissionEvent) {
	requestID, resolvedPhase := resolveSheinSubmitFailureState(pkg, requestedID, phase)
	record := submission.FailAttempt(pkg, action, requestID, resolvedPhase, submitErr, finishedAt)
	startedAt := record.SubmittedAt
	if !record.StartedAt.IsZero() {
		startedAt = record.StartedAt
	}
	return record, sheinpub.BuildSubmissionAttemptEvent(taskID, action, record, nil, submitErr, startedAt)
}

func failSheinSubmitAttemptWithResponseAndBuildEvent(pkg *SheinPackage, taskID, action, requestedID, phase string, response *sheinpub.SubmissionResponse, submitErr error, finishedAt time.Time) (*sheinpub.SubmissionRecord, sheinpub.SubmissionEvent) {
	requestID, resolvedPhase := resolveSheinSubmitFailureState(pkg, requestedID, phase)
	record := submission.FailAttempt(pkg, action, requestID, resolvedPhase, submitErr, finishedAt)
	startedAt := record.SubmittedAt
	if !record.StartedAt.IsZero() {
		startedAt = record.StartedAt
	}
	return record, sheinpub.BuildSubmissionAttemptEvent(taskID, action, record, response, submitErr, startedAt)
}

func resolveSheinSubmitFailureState(pkg *SheinPackage, requestedID, phase string) (string, string) {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	var currentID, currentPhase string
	if pkg != nil && pkg.SubmissionState != nil {
		currentID = pkg.SubmissionState.CurrentRequestID
		currentPhase = pkg.SubmissionState.CurrentPhase
	}
	return listingsubmission.ResolveFailureState(requestedID, phase, currentID, currentPhase, sheinpub.SubmissionPhaseValidate)
}

func findSheinSubmissionRecordByRequestID(pkg *SheinPackage, action, requestID string) *sheinpub.SubmissionRecord {
	return sheinpub.FindCompletedSubmissionRecordByRequestID(pkg, action, requestID)
}

func findActiveSheinSubmitAttempt(pkg *SheinPackage, action string, now time.Time) *sheinpub.SubmissionReport {
	return sheinpub.FindActiveSubmissionAttempt(pkg, action, now, sheinSubmitInFlightTTL)
}

func sheinSubmitAttemptNeedsRemoteRecovery(report *sheinpub.SubmissionReport, action string, now time.Time) bool {
	return sheinpub.SubmissionNeedsRemoteRecovery(report, action, now, sheinSubmitInFlightTTL)
}

func ensureSheinSubmissionReport(pkg *SheinPackage) *sheinpub.SubmissionReport {
	return sheinpub.EnsureSubmissionReport(pkg)
}

func sheinSubmissionRecordForAction(report *sheinpub.SubmissionReport, action string) *sheinpub.SubmissionRecord {
	return sheinpub.SubmissionRecordForAction(report, action)
}

func clearSheinSubmitInFlight(report *sheinpub.SubmissionReport, action, requestID string) {
	sheinpub.ClearSubmissionInFlight(report, action, requestID)
}

func setSheinSubmitRemoteRecord(pkg *SheinPackage, action, requestID, remoteStatus string, item *sheinproduct.RecordItem, checkedAt time.Time, message string) {
	sheinpub.SetSubmissionRemoteRecord(pkg, action, requestID, remoteStatus, item, checkedAt, message)
}

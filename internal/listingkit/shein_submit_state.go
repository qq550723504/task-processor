package listingkit

import (
	"time"

	listingsubmission "task-processor/internal/listing/submission"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func beginSheinSubmitAttempt(pkg *SheinPackage, action, requestID, phase string, startedAt time.Time) *sheinpub.SubmissionRecord {
	if pkg == nil {
		return nil
	}
	report := sheinpub.EnsureSubmissionReport(pkg)
	inFlight := listingsubmission.BeginInFlightState(sheinpub.SubmissionInFlightState(report), action, requestID, phase, startedAt, sheinSubmitInFlightTTL)
	sheinpub.ApplySubmissionInFlightState(report, inFlight)
	record := sheinpub.BuildSubmissionRunningRecord(action, requestID, phase, startedAt, inFlight.AttemptCount)
	sheinpub.ApplySubmissionRecord(pkg, record)
	return record
}

func advanceSheinSubmitPhase(pkg *SheinPackage, action, requestID, phase string) {
	advanceSheinSubmitPhaseAt(pkg, action, requestID, phase, time.Now())
}

func advanceSheinSubmitPhaseAndBuildEvent(pkg *SheinPackage, taskID, action, requestID, phase string, now time.Time) sheinpub.SubmissionEvent {
	advanceSheinSubmitPhaseAt(pkg, action, requestID, phase, now)
	return sheinpub.BuildSubmissionPhaseEvent(taskID, action, phase, sheinpub.SubmissionStatusRunning, requestID, now, "", nil)
}

func completeSheinSubmitAttempt(pkg *SheinPackage, action, requestID string, response *sheinpub.SubmissionResponse, submitErr error, finishedAt time.Time) *sheinpub.SubmissionRecord {
	return completeSheinSubmitAttemptAt(pkg, action, requestID, response, submitErr, finishedAt)
}

func completeSheinSubmitAttemptAndBuildEvent(pkg *SheinPackage, taskID, action, requestID string, response *sheinpub.SubmissionResponse, responseErr error, startedAt, finishedAt time.Time) (*sheinpub.SubmissionRecord, sheinpub.SubmissionEvent) {
	record := completeSheinSubmitAttemptAt(pkg, action, requestID, response, responseErr, finishedAt)
	return record, sheinpub.BuildSubmissionAttemptEvent(taskID, action, record, response, responseErr, startedAt)
}

func failSheinSubmitAttempt(pkg *SheinPackage, action, requestID, phase string, submitErr error, finishedAt time.Time) *sheinpub.SubmissionRecord {
	return failSheinSubmitAttemptAt(pkg, action, requestID, phase, submitErr, finishedAt)
}

func failSheinSubmitAttemptAndBuildEvent(pkg *SheinPackage, taskID, action, requestedID, phase string, submitErr error, finishedAt time.Time) (*sheinpub.SubmissionRecord, sheinpub.SubmissionEvent) {
	requestID, resolvedPhase := resolveSheinSubmitFailureState(pkg, requestedID, phase)
	record := failSheinSubmitAttemptAt(pkg, action, requestID, resolvedPhase, submitErr, finishedAt)
	startedAt := record.SubmittedAt
	if !record.StartedAt.IsZero() {
		startedAt = record.StartedAt
	}
	return record, sheinpub.BuildSubmissionAttemptEvent(taskID, action, record, nil, submitErr, startedAt)
}

func failSheinSubmitAttemptWithResponseAndBuildEvent(pkg *SheinPackage, taskID, action, requestedID, phase string, response *sheinpub.SubmissionResponse, submitErr error, finishedAt time.Time) (*sheinpub.SubmissionRecord, sheinpub.SubmissionEvent) {
	requestID, resolvedPhase := resolveSheinSubmitFailureState(pkg, requestedID, phase)
	record := failSheinSubmitAttemptAt(pkg, action, requestID, resolvedPhase, submitErr, finishedAt)
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

func advanceSheinSubmitPhaseAt(pkg *SheinPackage, action, requestID, phase string, now time.Time) {
	if pkg == nil {
		return
	}
	report := sheinpub.EnsureSubmissionReport(pkg)
	inFlight := listingsubmission.AdvanceInFlightState(sheinpub.SubmissionInFlightState(report), action, requestID, phase, now, sheinSubmitInFlightTTL)
	sheinpub.ApplySubmissionInFlightState(report, inFlight)
	sheinpub.SetSubmissionRecordPhase(report, action, requestID, phase)
}

func completeSheinSubmitAttemptAt(pkg *SheinPackage, action, requestID string, response *sheinpub.SubmissionResponse, submitErr error, finishedAt time.Time) *sheinpub.SubmissionRecord {
	if pkg == nil {
		return nil
	}
	report := sheinpub.EnsureSubmissionReport(pkg)
	record := sheinpub.ResolveSubmissionAttemptRecord(report, action, requestID, listingsubmission.AttemptSeedState{
		AttemptCount:      report.AttemptCount,
		InFlightStartedAt: report.InFlightStartedAt,
	}, finishedAt)
	finalizeState := listingsubmission.ResolveAttemptFinalizeState(action, sheinpub.SubmissionResponseOutcome(response), submitErr, finishedAt)
	sheinpub.ApplySubmissionAttemptFinalizeState(record, response, finalizeState)
	sheinpub.ApplySubmissionRecord(pkg, record)
	sheinpub.ClearSubmissionInFlight(report, action, requestID)
	return record
}

func failSheinSubmitAttemptAt(pkg *SheinPackage, action, requestID, phase string, submitErr error, finishedAt time.Time) *sheinpub.SubmissionRecord {
	if pkg == nil {
		return nil
	}
	report := sheinpub.EnsureSubmissionReport(pkg)
	record := sheinpub.ResolveSubmissionAttemptRecord(report, action, requestID, listingsubmission.AttemptSeedState{
		AttemptCount:      report.AttemptCount,
		InFlightStartedAt: report.InFlightStartedAt,
	}, finishedAt)
	finalizeState := listingsubmission.ResolveAttemptFailureFinalizeState(phase, submitErr, finishedAt)
	sheinpub.ApplySubmissionAttemptFailureState(record, phase, finalizeState)
	sheinpub.ApplySubmissionRecord(pkg, record)
	sheinpub.ClearSubmissionInFlight(report, action, requestID)
	return record
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

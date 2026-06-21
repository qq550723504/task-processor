package shein

import (
	"time"

	listingsubmission "task-processor/internal/listing/submission"
)

// BeginSubmitAttempt records a running submission attempt and refreshes its lease.
func BeginSubmitAttempt(pkg *Package, action, requestID, phase string, startedAt time.Time, ttl time.Duration) *SubmissionRecord {
	if pkg == nil {
		return nil
	}
	report := EnsureSubmissionReport(pkg)
	inFlight := listingsubmission.BeginInFlightState(SubmissionInFlightState(report), action, requestID, phase, startedAt, ttl)
	ApplySubmissionInFlightState(report, inFlight)
	record := BuildSubmissionRunningRecord(action, requestID, phase, startedAt, inFlight.AttemptCount)
	ApplySubmissionRecord(pkg, record)
	return record
}

// AdvanceSubmitPhaseAt moves an active submission attempt to the next phase.
func AdvanceSubmitPhaseAt(pkg *Package, action, requestID, phase string, now time.Time, ttl time.Duration) {
	if pkg == nil {
		return
	}
	report := EnsureSubmissionReport(pkg)
	inFlight := listingsubmission.AdvanceInFlightState(SubmissionInFlightState(report), action, requestID, phase, now, ttl)
	ApplySubmissionInFlightState(report, inFlight)
	SetSubmissionRecordPhase(report, action, requestID, phase)
}

// AdvanceSubmitPhaseAndBuildEvent moves an attempt phase and returns its phase event.
func AdvanceSubmitPhaseAndBuildEvent(pkg *Package, taskID, action, requestID, phase string, now time.Time, ttl time.Duration) SubmissionEvent {
	AdvanceSubmitPhaseAt(pkg, action, requestID, phase, now, ttl)
	return BuildSubmissionPhaseEvent(taskID, action, phase, SubmissionStatusRunning, requestID, now, "", nil)
}

// CompleteSubmitAttemptAt finalizes a submission attempt from its remote response.
func CompleteSubmitAttemptAt(pkg *Package, action, requestID string, response *SubmissionResponse, submitErr error, finishedAt time.Time) *SubmissionRecord {
	if pkg == nil {
		return nil
	}
	report := EnsureSubmissionReport(pkg)
	record := ResolveSubmissionAttemptRecord(report, action, requestID, listingsubmission.AttemptSeedState{
		AttemptCount:      report.AttemptCount,
		InFlightStartedAt: report.InFlightStartedAt,
	}, finishedAt)
	finalizeState := listingsubmission.ResolveAttemptFinalizeState(action, SubmissionResponseOutcome(response), submitErr, finishedAt)
	ApplySubmissionAttemptFinalizeState(record, response, finalizeState)
	ApplySubmissionRecord(pkg, record)
	ClearSubmissionInFlight(report, action, requestID)
	return record
}

// CompleteSubmitAttemptAndBuildEvent finalizes a submission attempt and returns its event.
func CompleteSubmitAttemptAndBuildEvent(pkg *Package, taskID, action, requestID string, response *SubmissionResponse, responseErr error, startedAt, finishedAt time.Time) (*SubmissionRecord, SubmissionEvent) {
	record := CompleteSubmitAttemptAt(pkg, action, requestID, response, responseErr, finishedAt)
	return record, BuildSubmissionAttemptEvent(taskID, action, record, response, responseErr, startedAt)
}

// FailSubmitAttemptAt finalizes a submission attempt as failed.
func FailSubmitAttemptAt(pkg *Package, action, requestID, phase string, submitErr error, finishedAt time.Time) *SubmissionRecord {
	if pkg == nil {
		return nil
	}
	report := EnsureSubmissionReport(pkg)
	record := ResolveSubmissionAttemptRecord(report, action, requestID, listingsubmission.AttemptSeedState{
		AttemptCount:      report.AttemptCount,
		InFlightStartedAt: report.InFlightStartedAt,
	}, finishedAt)
	finalizeState := listingsubmission.ResolveAttemptFailureFinalizeState(phase, submitErr, finishedAt)
	ApplySubmissionAttemptFailureState(record, phase, finalizeState)
	ApplySubmissionRecord(pkg, record)
	ClearSubmissionInFlight(report, action, requestID)
	return record
}

// FailSubmitAttemptAndBuildEvent finalizes a submission attempt as failed and returns its event.
func FailSubmitAttemptAndBuildEvent(pkg *Package, taskID, action, requestedID, phase string, submitErr error, finishedAt time.Time) (*SubmissionRecord, SubmissionEvent) {
	requestID, resolvedPhase := ResolveSubmitFailureState(pkg, requestedID, phase)
	record := FailSubmitAttemptAt(pkg, action, requestID, resolvedPhase, submitErr, finishedAt)
	startedAt := submissionRecordStartedAt(record)
	return record, BuildSubmissionAttemptEvent(taskID, action, record, nil, submitErr, startedAt)
}

// FailSubmitAttemptWithResponseAndBuildEvent finalizes a failed attempt while preserving a remote response payload for the event.
func FailSubmitAttemptWithResponseAndBuildEvent(pkg *Package, taskID, action, requestedID, phase string, response *SubmissionResponse, submitErr error, finishedAt time.Time) (*SubmissionRecord, SubmissionEvent) {
	requestID, resolvedPhase := ResolveSubmitFailureState(pkg, requestedID, phase)
	record := FailSubmitAttemptAt(pkg, action, requestID, resolvedPhase, submitErr, finishedAt)
	startedAt := submissionRecordStartedAt(record)
	return record, BuildSubmissionAttemptEvent(taskID, action, record, response, submitErr, startedAt)
}

// ResolveSubmitFailureState derives the request and phase to use when closing out a failed attempt.
func ResolveSubmitFailureState(pkg *Package, requestedID, phase string) (string, string) {
	pkg = NormalizePackageSemanticFields(pkg)
	var currentID, currentPhase string
	if pkg != nil && pkg.SubmissionState != nil {
		currentID = pkg.SubmissionState.CurrentRequestID
		currentPhase = pkg.SubmissionState.CurrentPhase
	}
	return listingsubmission.ResolveFailureState(requestedID, phase, currentID, currentPhase, SubmissionPhaseValidate)
}

func submissionRecordStartedAt(record *SubmissionRecord) time.Time {
	if record == nil {
		return time.Time{}
	}
	startedAt := record.SubmittedAt
	if !record.StartedAt.IsZero() {
		startedAt = record.StartedAt
	}
	return startedAt
}

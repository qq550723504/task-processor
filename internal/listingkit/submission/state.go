package submission

import (
	"time"

	listingsubmission "task-processor/internal/listing/submission"
	sheinpub "task-processor/internal/publishing/shein"
)

const InFlightTTL = listingsubmission.InFlightTTL

func BeginAttempt(pkg *sheinpub.Package, action, requestID, phase string, startedAt time.Time, ttl time.Duration) *sheinpub.SubmissionRecord {
	if pkg == nil {
		return nil
	}
	report := sheinpub.EnsureSubmissionReport(pkg)
	inFlight := listingsubmission.BeginInFlightState(sheinpub.SubmissionInFlightState(report), action, requestID, phase, startedAt, ttl)
	sheinpub.ApplySubmissionInFlightState(report, inFlight)
	record := sheinpub.BuildSubmissionRunningRecord(action, requestID, phase, startedAt, inFlight.AttemptCount)
	sheinpub.ApplySubmissionRecord(pkg, record)
	return record
}

func AdvancePhase(pkg *sheinpub.Package, action, requestID, phase string, now time.Time, ttl time.Duration) {
	if pkg == nil {
		return
	}
	report := sheinpub.EnsureSubmissionReport(pkg)
	inFlight := listingsubmission.AdvanceInFlightState(sheinpub.SubmissionInFlightState(report), action, requestID, phase, now, ttl)
	sheinpub.ApplySubmissionInFlightState(report, inFlight)
	sheinpub.SetSubmissionRecordPhase(report, action, requestID, phase)
}

func CompleteAttempt(pkg *sheinpub.Package, action, requestID string, response *sheinpub.SubmissionResponse, submitErr error, finishedAt time.Time) *sheinpub.SubmissionRecord {
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

func FailAttempt(pkg *sheinpub.Package, action, requestID, phase string, submitErr error, finishedAt time.Time) *sheinpub.SubmissionRecord {
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

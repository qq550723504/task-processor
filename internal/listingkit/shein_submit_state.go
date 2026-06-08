package listingkit

import (
	"time"

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

func completeSheinSubmitAttempt(pkg *SheinPackage, action, requestID string, response *sheinpub.SubmissionResponse, submitErr error, finishedAt time.Time) *sheinpub.SubmissionRecord {
	return submission.CompleteAttempt(pkg, action, requestID, response, submitErr, finishedAt)
}

func failSheinSubmitAttempt(pkg *SheinPackage, action, requestID, phase string, submitErr error, finishedAt time.Time) *sheinpub.SubmissionRecord {
	return submission.FailAttempt(pkg, action, requestID, phase, submitErr, finishedAt)
}

func findSheinSubmissionRecordByRequestID(pkg *SheinPackage, action, requestID string) *sheinpub.SubmissionRecord {
	return submission.FindRecordByRequestID(pkg, action, requestID)
}

func findActiveSheinSubmitAttempt(pkg *SheinPackage, action string, now time.Time) *sheinpub.SubmissionReport {
	return submission.FindActiveAttempt(pkg, action, now, sheinSubmitInFlightTTL)
}

func sheinSubmitAttemptNeedsRemoteRecovery(report *sheinpub.SubmissionReport, action string, now time.Time) bool {
	return submission.NeedsRemoteRecovery(report, action, now, sheinSubmitInFlightTTL)
}

func ensureSheinSubmissionReport(pkg *SheinPackage) *sheinpub.SubmissionReport {
	return submission.EnsureReport(pkg)
}

func sheinSubmissionRecordForAction(report *sheinpub.SubmissionReport, action string) *sheinpub.SubmissionRecord {
	return submission.RecordForAction(report, action)
}

func clearSheinSubmitInFlight(report *sheinpub.SubmissionReport, action, requestID string) {
	submission.ClearInFlight(report, action, requestID)
}

func setSheinSubmitSupplierCode(pkg *SheinPackage, action, requestID, supplierCode string) {
	submission.SetSupplierCode(pkg, action, requestID, supplierCode)
}

func setSheinSubmitRemoteResponse(pkg *SheinPackage, action, requestID, supplierCode string, response *sheinpub.SubmissionResponse) {
	submission.SetRemoteResponse(pkg, action, requestID, supplierCode, response)
}

func setSheinSubmitSnapshot(pkg *SheinPackage, action, requestID string, snapshot *sheinpub.SubmitSnapshot) {
	submission.SetSubmitSnapshot(pkg, action, requestID, snapshot)
}

func setSheinSubmitRemoteRecord(pkg *SheinPackage, action, requestID, remoteStatus string, item *sheinproduct.RecordItem, checkedAt time.Time, message string) {
	submission.SetRemoteRecord(pkg, action, requestID, remoteStatus, item, checkedAt, message)
}

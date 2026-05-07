package listingkit

import (
	"time"

	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func beginSheinSubmitAttempt(pkg *SheinPackage, action, requestID, phase string, startedAt time.Time) *sheinpub.SubmissionRecord {
	if pkg == nil {
		return nil
	}
	report := ensureSheinSubmissionReport(pkg)
	report.AttemptCount++
	report.CurrentAction = action
	report.CurrentPhase = phase
	report.CurrentRequestID = requestID
	report.InFlightStartedAt = &startedAt
	leaseExpiresAt := startedAt.Add(sheinSubmitInFlightTTL)
	report.LeaseExpiresAt = &leaseExpiresAt

	record := &sheinpub.SubmissionRecord{
		Action:      action,
		Status:      sheinpub.SubmissionStatusRunning,
		SubmittedAt: startedAt,
		RequestID:   requestID,
		Phase:       phase,
		StartedAt:   startedAt,
		Attempt:     report.AttemptCount,
	}
	applySheinSubmissionRecord(pkg, record)
	return record
}

func advanceSheinSubmitPhase(pkg *SheinPackage, action, requestID, phase string) {
	if pkg == nil {
		return
	}
	report := ensureSheinSubmissionReport(pkg)
	report.CurrentAction = action
	report.CurrentPhase = phase
	report.CurrentRequestID = requestID
	leaseExpiresAt := time.Now().Add(sheinSubmitInFlightTTL)
	report.LeaseExpiresAt = &leaseExpiresAt
	record := sheinSubmissionRecordForAction(report, action)
	if record == nil || record.RequestID != requestID {
		return
	}
	record.Phase = phase
}

func completeSheinSubmitAttempt(pkg *SheinPackage, action, requestID string, response *sheinpub.SubmissionResponse, submitErr error, finishedAt time.Time) *sheinpub.SubmissionRecord {
	if pkg == nil {
		return nil
	}
	report := ensureSheinSubmissionReport(pkg)
	record := sheinSubmissionRecordForAction(report, action)
	if record == nil || record.RequestID != requestID {
		startedAt := finishedAt
		if report.InFlightStartedAt != nil {
			startedAt = *report.InFlightStartedAt
		}
		record = &sheinpub.SubmissionRecord{
			Action:      action,
			SubmittedAt: startedAt,
			RequestID:   requestID,
			StartedAt:   startedAt,
			Attempt:     report.AttemptCount,
		}
	}
	record.Result = response
	record.FinishedAt = &finishedAt
	record.SubmittedAt = finishedAt
	if submitErr != nil {
		record.Status = sheinpub.SubmissionStatusFailed
		record.Error = submitErr.Error()
	} else if response != nil && (response.Success || saveDraftSucceeded(action, response)) {
		record.Status = sheinpub.SubmissionStatusSuccess
		record.Error = ""
	} else {
		record.Status = "unknown"
		record.Error = ""
	}
	applySheinSubmissionRecord(pkg, record)
	clearSheinSubmitInFlight(report, action, requestID)
	return record
}

func failSheinSubmitAttempt(pkg *SheinPackage, action, requestID, phase string, submitErr error, finishedAt time.Time) *sheinpub.SubmissionRecord {
	if pkg == nil {
		return nil
	}
	report := ensureSheinSubmissionReport(pkg)
	startedAt := finishedAt
	if report.InFlightStartedAt != nil {
		startedAt = *report.InFlightStartedAt
	}
	record := sheinSubmissionRecordForAction(report, action)
	if record == nil || record.RequestID != requestID {
		record = &sheinpub.SubmissionRecord{
			Action:      action,
			SubmittedAt: startedAt,
			RequestID:   requestID,
			StartedAt:   startedAt,
			Attempt:     report.AttemptCount,
		}
	}
	record.Status = sheinpub.SubmissionStatusFailed
	record.Phase = phase
	record.FinishedAt = &finishedAt
	record.SubmittedAt = finishedAt
	if submitErr != nil {
		record.Error = submitErr.Error()
	}
	applySheinSubmissionRecord(pkg, record)
	clearSheinSubmitInFlight(report, action, requestID)
	return record
}

func findSheinSubmissionRecordByRequestID(pkg *SheinPackage, action, requestID string) *sheinpub.SubmissionRecord {
	if pkg == nil || pkg.Submission == nil || requestID == "" {
		return nil
	}
	record := sheinSubmissionRecordForAction(pkg.Submission, action)
	if record == nil || record.RequestID != requestID || record.FinishedAt == nil {
		return nil
	}
	return record
}

func findActiveSheinSubmitAttempt(pkg *SheinPackage, action string, now time.Time) *sheinpub.SubmissionReport {
	if pkg == nil || pkg.Submission == nil {
		return nil
	}
	report := pkg.Submission
	if report.CurrentAction != action || report.CurrentRequestID == "" || report.CurrentPhase == "" || report.InFlightStartedAt == nil {
		return nil
	}
	if report.LeaseExpiresAt != nil {
		if now.After(*report.LeaseExpiresAt) {
			return nil
		}
		return report
	}
	if now.Sub(*report.InFlightStartedAt) > sheinSubmitInFlightTTL {
		return nil
	}
	return report
}

func sheinSubmitAttemptNeedsRemoteRecovery(report *sheinpub.SubmissionReport, action string, now time.Time) bool {
	if report == nil || report.CurrentAction != action || report.CurrentRequestID == "" {
		return false
	}
	switch report.CurrentPhase {
	case sheinpub.SubmissionPhaseSubmitRemote, sheinpub.SubmissionPhasePersistResult, sheinpub.SubmissionPhaseConfirmRemote:
	default:
		return false
	}
	if report.LeaseExpiresAt != nil {
		return now.After(*report.LeaseExpiresAt)
	}
	if report.InFlightStartedAt == nil {
		return true
	}
	return now.Sub(*report.InFlightStartedAt) > sheinSubmitInFlightTTL
}

func ensureSheinSubmissionReport(pkg *SheinPackage) *sheinpub.SubmissionReport {
	if pkg.Submission == nil {
		pkg.Submission = &sheinpub.SubmissionReport{}
	}
	return pkg.Submission
}

func sheinSubmissionRecordForAction(report *sheinpub.SubmissionReport, action string) *sheinpub.SubmissionRecord {
	if report == nil {
		return nil
	}
	switch action {
	case "save_draft":
		return report.SaveDraft
	case "publish":
		return report.Publish
	default:
		return nil
	}
}

func clearSheinSubmitInFlight(report *sheinpub.SubmissionReport, action, requestID string) {
	if report == nil {
		return
	}
	if report.CurrentAction != action || report.CurrentRequestID != requestID {
		return
	}
	report.CurrentAction = ""
	report.CurrentPhase = ""
	report.CurrentRequestID = ""
	report.InFlightStartedAt = nil
	report.LeaseExpiresAt = nil
}

func setSheinSubmitSupplierCode(pkg *SheinPackage, action, requestID, supplierCode string) {
	if pkg == nil || pkg.Submission == nil || supplierCode == "" {
		return
	}
	record := sheinSubmissionRecordForAction(pkg.Submission, action)
	if record == nil || record.RequestID != requestID {
		return
	}
	record.SupplierCode = supplierCode
}

func setSheinSubmitRemoteResponse(pkg *SheinPackage, action, requestID, supplierCode string, response *sheinpub.SubmissionResponse) {
	if pkg == nil || pkg.Submission == nil {
		return
	}
	record := sheinSubmissionRecordForAction(pkg.Submission, action)
	if record == nil || record.RequestID != requestID {
		return
	}
	if supplierCode != "" {
		record.SupplierCode = supplierCode
	}
	record.Result = response
	pkg.Submission.LastResult = response
}

func setSheinSubmitRemoteRecord(pkg *SheinPackage, action, requestID, remoteStatus string, item *sheinproduct.RecordItem, checkedAt time.Time, message string) {
	if pkg == nil {
		return
	}
	report := ensureSheinSubmissionReport(pkg)
	report.RemoteStatus = remoteStatus
	report.RemoteCheckedAt = &checkedAt
	record := sheinSubmissionRecordForAction(report, action)
	if record == nil || record.RequestID != requestID {
		return
	}
	record.RemoteCheckedAt = &checkedAt
	record.RemoteMessage = message
	if item != nil {
		record.RemoteRecordID = item.RecordID
		record.RemoteState = item.State
		record.RemoteAuditState = item.AuditState
		if record.SupplierCode == "" {
			record.SupplierCode = item.SupplierCode
		}
	}
}

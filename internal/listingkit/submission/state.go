package submission

import (
	"time"

	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

const InFlightTTL = 15 * time.Minute

func BeginAttempt(pkg *sheinpub.Package, action, requestID, phase string, startedAt time.Time, ttl time.Duration) *sheinpub.SubmissionRecord {
	if pkg == nil {
		return nil
	}
	report := EnsureReport(pkg)
	report.AttemptCount++
	report.CurrentAction = action
	report.CurrentPhase = phase
	report.CurrentRequestID = requestID
	report.InFlightStartedAt = &startedAt
	leaseExpiresAt := startedAt.Add(ttl)
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
	ApplyRecord(pkg, record)
	return record
}

func AdvancePhase(pkg *sheinpub.Package, action, requestID, phase string, now time.Time, ttl time.Duration) {
	if pkg == nil {
		return
	}
	report := EnsureReport(pkg)
	report.CurrentAction = action
	report.CurrentPhase = phase
	report.CurrentRequestID = requestID
	leaseExpiresAt := now.Add(ttl)
	report.LeaseExpiresAt = &leaseExpiresAt
	record := RecordForAction(report, action)
	if record == nil || record.RequestID != requestID {
		return
	}
	record.Phase = phase
}

func CompleteAttempt(pkg *sheinpub.Package, action, requestID string, response *sheinpub.SubmissionResponse, submitErr error, finishedAt time.Time) *sheinpub.SubmissionRecord {
	if pkg == nil {
		return nil
	}
	report := EnsureReport(pkg)
	record := RecordForAction(report, action)
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
	if submitErr != nil {
		record.Status = sheinpub.SubmissionStatusFailed
		record.Error = submitErr.Error()
	} else if response != nil && (response.Success || SaveDraftSucceeded(action, response)) {
		record.Status = sheinpub.SubmissionStatusSuccess
		record.Error = ""
	} else {
		record.Status = "unknown"
		record.Error = ""
	}
	ApplyRecord(pkg, record)
	ClearInFlight(report, action, requestID)
	return record
}

func FailAttempt(pkg *sheinpub.Package, action, requestID, phase string, submitErr error, finishedAt time.Time) *sheinpub.SubmissionRecord {
	if pkg == nil {
		return nil
	}
	report := EnsureReport(pkg)
	startedAt := finishedAt
	if report.InFlightStartedAt != nil {
		startedAt = *report.InFlightStartedAt
	}
	record := RecordForAction(report, action)
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
	if submitErr != nil {
		record.Error = submitErr.Error()
	}
	ApplyRecord(pkg, record)
	ClearInFlight(report, action, requestID)
	return record
}

func FindRecordByRequestID(pkg *sheinpub.Package, action, requestID string) *sheinpub.SubmissionRecord {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.SubmissionState == nil || requestID == "" {
		return nil
	}
	record := RecordForAction(pkg.SubmissionState, action)
	if record == nil || record.RequestID != requestID || record.FinishedAt == nil {
		return nil
	}
	return record
}

func FindActiveAttempt(pkg *sheinpub.Package, action string, now time.Time, ttl time.Duration) *sheinpub.SubmissionReport {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.SubmissionState == nil {
		return nil
	}
	report := pkg.SubmissionState
	if report.CurrentAction != action || report.CurrentRequestID == "" || report.CurrentPhase == "" || report.InFlightStartedAt == nil {
		return nil
	}
	if report.LeaseExpiresAt != nil {
		if now.After(*report.LeaseExpiresAt) {
			return nil
		}
		return report
	}
	if now.Sub(*report.InFlightStartedAt) > ttl {
		return nil
	}
	return report
}

func NeedsRemoteRecovery(report *sheinpub.SubmissionReport, action string, now time.Time, ttl time.Duration) bool {
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
	return now.Sub(*report.InFlightStartedAt) > ttl
}

func EnsureReport(pkg *sheinpub.Package) *sheinpub.SubmissionReport {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg.SubmissionState == nil {
		pkg.SubmissionState = &sheinpub.SubmissionReport{}
	}
	return pkg.SubmissionState
}

func ApplyRecord(pkg *sheinpub.Package, record *sheinpub.SubmissionRecord) {
	if pkg == nil || record == nil {
		return
	}
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg.SubmissionState == nil {
		pkg.SubmissionState = &sheinpub.SubmissionReport{}
	}
	pkg.SubmissionState.LastAction = record.Action
	pkg.SubmissionState.LastStatus = record.Status
	pkg.SubmissionState.LastError = record.Error
	pkg.SubmissionState.SubmittedAt = &record.SubmittedAt
	pkg.SubmissionState.LastResult = record.Result
	switch record.Action {
	case "save_draft":
		pkg.SubmissionState.SaveDraft = record
	case "publish":
		pkg.SubmissionState.Publish = record
	}
}

func RecordForAction(report *sheinpub.SubmissionReport, action string) *sheinpub.SubmissionRecord {
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

func ClearInFlight(report *sheinpub.SubmissionReport, action, requestID string) {
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

func SetSupplierCode(pkg *sheinpub.Package, action, requestID, supplierCode string) {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.SubmissionState == nil || supplierCode == "" {
		return
	}
	record := RecordForAction(pkg.SubmissionState, action)
	if record == nil || record.RequestID != requestID {
		return
	}
	record.SupplierCode = supplierCode
}

func SetRemoteResponse(pkg *sheinpub.Package, action, requestID, supplierCode string, response *sheinpub.SubmissionResponse) {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.SubmissionState == nil {
		return
	}
	record := RecordForAction(pkg.SubmissionState, action)
	if record == nil || record.RequestID != requestID {
		return
	}
	if supplierCode != "" {
		record.SupplierCode = supplierCode
	}
	record.Result = response
	pkg.SubmissionState.LastResult = response
}

func SetSubmitSnapshot(pkg *sheinpub.Package, action, requestID string, snapshot *sheinpub.SubmitSnapshot) {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.SubmissionState == nil || snapshot == nil {
		return
	}
	record := RecordForAction(pkg.SubmissionState, action)
	if record == nil || record.RequestID != requestID {
		return
	}
	record.SubmitSnapshot = snapshot
}

func SetRemoteRecord(pkg *sheinpub.Package, action, requestID, remoteStatus string, item *sheinproduct.RecordItem, checkedAt time.Time, message string) {
	if pkg == nil {
		return
	}
	report := EnsureReport(pkg)
	report.RemoteStatus = remoteStatus
	report.RemoteCheckedAt = &checkedAt
	record := RecordForAction(report, action)
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

func SaveDraftSucceeded(action string, result *sheinpub.SubmissionResponse) bool {
	if action != "save_draft" || result == nil {
		return false
	}
	return result.Success || result.Code == "0"
}

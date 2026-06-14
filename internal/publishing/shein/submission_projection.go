package shein

import "time"

type SubmissionProjection struct {
	WorkflowStatus  string
	LatestStatus    string
	LatestError     string
	RemoteStatus    string
	RemoteCheckedAt *time.Time
	RemoteRecordID  string
}

func LatestSubmissionOutcomeEvent(pkg *Package) *SubmissionEvent {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || len(pkg.SubmissionEvents) == 0 {
		return nil
	}
	for i := range pkg.SubmissionEvents {
		event := &pkg.SubmissionEvents[i]
		if event.Action != "submit_phase" {
			return event
		}
	}
	return nil
}

func PrimarySubmissionRecord(report *SubmissionReport) *SubmissionRecord {
	if report == nil {
		return nil
	}
	record := SubmissionRecordForAction(report, report.LastAction)
	if record != nil {
		return record
	}
	if report.Publish != nil {
		return report.Publish
	}
	return report.SaveDraft
}

func SubmissionWorkflowStatus(pkg *Package, ready bool) string {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil {
		return ""
	}
	if latest := LatestSubmissionOutcomeEvent(pkg); latest != nil {
		if latest.Action == "publish" && latest.Status == SubmissionStatusSuccess {
			return "published"
		}
		if latest.Action == "save_draft" && latest.Status == SubmissionStatusSuccess {
			return "draft_saved"
		}
		if latest.Status == SubmissionStatusFailed {
			return "publish_failed"
		}
	}
	if pkg.SubmissionState != nil {
		if pkg.SubmissionState.Publish != nil && pkg.SubmissionState.Publish.Status == SubmissionStatusSuccess {
			return "published"
		}
		if pkg.SubmissionState.SaveDraft != nil && pkg.SubmissionState.SaveDraft.Status == SubmissionStatusSuccess {
			return "draft_saved"
		}
		if pkg.SubmissionState.LastStatus == SubmissionStatusFailed {
			return "publish_failed"
		}
	}
	if ready {
		return "ready_to_submit"
	}
	return "pending_confirmation"
}

func ResolveSubmissionProjection(pkg *Package, ready bool) SubmissionProjection {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil {
		return SubmissionProjection{}
	}

	projection := SubmissionProjection{
		WorkflowStatus: SubmissionWorkflowStatus(pkg, ready),
	}
	if latest := LatestSubmissionOutcomeEvent(pkg); latest != nil {
		projection.LatestStatus = latest.Status
		projection.LatestError = latest.ErrorMessage
	} else if pkg.SubmissionState != nil {
		projection.LatestStatus = pkg.SubmissionState.LastStatus
		projection.LatestError = pkg.SubmissionState.LastError
	}
	if pkg.SubmissionState == nil {
		return projection
	}

	projection.RemoteStatus = pkg.SubmissionState.RemoteStatus
	projection.RemoteCheckedAt = pkg.SubmissionState.RemoteCheckedAt

	record := PrimarySubmissionRecord(pkg.SubmissionState)
	if record == nil {
		return projection
	}
	projection.RemoteRecordID = record.RemoteRecordID
	if projection.RemoteCheckedAt == nil {
		projection.RemoteCheckedAt = record.RemoteCheckedAt
	}
	return projection
}

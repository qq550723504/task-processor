package shein

import (
	"time"

	listingsubmission "task-processor/internal/listing/submission"
	sheinmarketpub "task-processor/internal/marketplace/shein/publishing"
)

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
	latest := listingsubmission.LatestOutcomeEvent(submissionProjectionEvents(pkg))
	if latest == nil {
		return nil
	}
	for i := range pkg.SubmissionEvents {
		if pkg.SubmissionEvents[i].Action == latest.Action &&
			pkg.SubmissionEvents[i].Status == latest.Status &&
			pkg.SubmissionEvents[i].ErrorMessage == latest.ErrorMessage {
			return &pkg.SubmissionEvents[i]
		}
	}
	return nil
}

func PrimarySubmissionRecord(report *SubmissionReport) *SubmissionRecord {
	if report == nil {
		return nil
	}
	primary := listingsubmission.PrimaryActionRecord(submissionProjectionReport(report))
	if primary == nil {
		return nil
	}
	switch primary.Action {
	case listingsubmission.SubmitActionPublish:
		return report.Publish
	case listingsubmission.SubmitActionSaveDraft:
		return report.SaveDraft
	default:
		return nil
	}
}

func SubmissionWorkflowStatus(pkg *Package, ready bool) string {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil {
		return ""
	}
	return listingsubmission.WorkflowStatus(
		submissionProjectionEvents(pkg),
		submissionProjectionReport(pkg.SubmissionState),
		ready,
		sheinSubmissionProjectionPolicy(),
	)
}

func ResolveSubmissionProjection(pkg *Package, ready bool) SubmissionProjection {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil {
		return SubmissionProjection{}
	}

	generic := listingsubmission.ResolveProjection(
		submissionProjectionEvents(pkg),
		submissionProjectionReport(pkg.SubmissionState),
		ready,
		sheinSubmissionProjectionPolicy(),
	)
	return SubmissionProjection{
		WorkflowStatus:  generic.WorkflowStatus,
		LatestStatus:    generic.LatestStatus,
		LatestError:     generic.LatestError,
		RemoteStatus:    generic.RemoteStatus,
		RemoteCheckedAt: generic.RemoteCheckedAt,
		RemoteRecordID:  generic.RemoteRecordID,
	}
}

func sheinSubmissionProjectionPolicy() listingsubmission.ProjectionWorkflowPolicy {
	return sheinmarketpub.SubmissionProjectionWorkflowPolicy(SubmissionStatusSuccess, SubmissionStatusFailed)
}

func submissionProjectionEvents(pkg *Package) []listingsubmission.Event {
	if pkg == nil || len(pkg.SubmissionEvents) == 0 {
		return nil
	}
	events := make([]listingsubmission.Event, 0, len(pkg.SubmissionEvents))
	for _, event := range pkg.SubmissionEvents {
		events = append(events, listingsubmission.Event{
			Action:       event.Action,
			Status:       event.Status,
			ErrorMessage: event.ErrorMessage,
		})
	}
	return events
}

func submissionProjectionReport(report *SubmissionReport) *listingsubmission.Report {
	if report == nil {
		return nil
	}
	return &listingsubmission.Report{
		LastAction:      report.LastAction,
		LastStatus:      report.LastStatus,
		LastError:       report.LastError,
		RemoteStatus:    report.RemoteStatus,
		RemoteCheckedAt: report.RemoteCheckedAt,
		SaveDraft:       submissionProjectionRecord(report.SaveDraft, listingsubmission.SubmitActionSaveDraft),
		Publish:         submissionProjectionRecord(report.Publish, listingsubmission.SubmitActionPublish),
	}
}

func submissionProjectionRecord(record *SubmissionRecord, fallbackAction string) *listingsubmission.ActionRecord {
	if record == nil {
		return nil
	}
	return &listingsubmission.ActionRecord{
		Action:          submissionRecordAction(record, fallbackAction),
		Status:          record.Status,
		RemoteRecordID:  record.RemoteRecordID,
		RemoteCheckedAt: record.RemoteCheckedAt,
	}
}

func submissionRecordAction(record *SubmissionRecord, fallback string) string {
	if record == nil {
		return ""
	}
	if record.Action != "" {
		return record.Action
	}
	return fallback
}

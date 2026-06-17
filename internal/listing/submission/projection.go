package submission

import "time"

const SubmitActionPhase = "submit_phase"

type ProjectionWorkflowPolicy struct {
	SuccessStatus            string
	FailedStatus             string
	PublishedWorkflowStatus  string
	DraftSavedWorkflowStatus string
	FailedWorkflowStatus     string
	ReadyWorkflowStatus      string
	PendingWorkflowStatus    string
}

type Event struct {
	Action       string
	Status       string
	ErrorMessage string
}

type ActionRecord struct {
	Action          string
	Status          string
	RemoteRecordID  string
	RemoteCheckedAt *time.Time
}

type Report struct {
	LastAction      string
	LastStatus      string
	LastError       string
	RemoteStatus    string
	RemoteCheckedAt *time.Time
	SaveDraft       *ActionRecord
	Publish         *ActionRecord
}

type Projection struct {
	WorkflowStatus  string
	LatestStatus    string
	LatestError     string
	RemoteStatus    string
	RemoteCheckedAt *time.Time
	RemoteRecordID  string
}

func LatestOutcomeEvent(events []Event) *Event {
	for i := range events {
		if events[i].Action != SubmitActionPhase {
			return &events[i]
		}
	}
	return nil
}

func PrimaryActionRecord(report *Report) *ActionRecord {
	if report == nil {
		return nil
	}
	if record := actionRecordForAction(report, report.LastAction); record != nil {
		return record
	}
	if report.Publish != nil {
		return report.Publish
	}
	return report.SaveDraft
}

func WorkflowStatus(events []Event, report *Report, ready bool, policy ProjectionWorkflowPolicy) string {
	if latest := LatestOutcomeEvent(events); latest != nil {
		if latest.Action == SubmitActionPublish && latest.Status == policy.SuccessStatus {
			return policy.PublishedWorkflowStatus
		}
		if latest.Action == SubmitActionSaveDraft && latest.Status == policy.SuccessStatus {
			return policy.DraftSavedWorkflowStatus
		}
		if latest.Status == policy.FailedStatus {
			return policy.FailedWorkflowStatus
		}
	}
	if report != nil {
		if report.Publish != nil && report.Publish.Status == policy.SuccessStatus {
			return policy.PublishedWorkflowStatus
		}
		if report.SaveDraft != nil && report.SaveDraft.Status == policy.SuccessStatus {
			return policy.DraftSavedWorkflowStatus
		}
		if report.LastStatus == policy.FailedStatus {
			return policy.FailedWorkflowStatus
		}
	}
	if ready {
		return policy.ReadyWorkflowStatus
	}
	return policy.PendingWorkflowStatus
}

func ResolveProjection(events []Event, report *Report, ready bool, policy ProjectionWorkflowPolicy) Projection {
	projection := Projection{
		WorkflowStatus: WorkflowStatus(events, report, ready, policy),
	}
	if latest := LatestOutcomeEvent(events); latest != nil {
		projection.LatestStatus = latest.Status
		projection.LatestError = latest.ErrorMessage
	} else if report != nil {
		projection.LatestStatus = report.LastStatus
		projection.LatestError = report.LastError
	}
	if report == nil {
		return projection
	}

	projection.RemoteStatus = report.RemoteStatus
	projection.RemoteCheckedAt = report.RemoteCheckedAt
	record := PrimaryActionRecord(report)
	if record == nil {
		return projection
	}
	projection.RemoteRecordID = record.RemoteRecordID
	if projection.RemoteCheckedAt == nil {
		projection.RemoteCheckedAt = record.RemoteCheckedAt
	}
	return projection
}

func actionRecordForAction(report *Report, action string) *ActionRecord {
	if report == nil {
		return nil
	}
	switch action {
	case SubmitActionPublish:
		return report.Publish
	case SubmitActionSaveDraft:
		return report.SaveDraft
	default:
		return nil
	}
}

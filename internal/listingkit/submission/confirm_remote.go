package submission

import (
	"time"

	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

type ConfirmRemoteParts struct {
	RemoteStatus string
	Record       *sheinproduct.RecordItem
	CheckedAt    time.Time
	Message      string
	Event        sheinpub.SubmissionEvent
}

func BuildConfirmRemoteParts(taskID, action, status, requestID string, startedAt time.Time, detail string, err error) ConfirmRemoteParts {
	return ConfirmRemoteParts{
		RemoteStatus: status,
		CheckedAt:    time.Now(),
		Message:      detail,
		Event:        BuildConfirmRemoteEvent(taskID, action, status, requestID, startedAt, detail, err),
	}
}

func BuildConfirmRemotePartsForRecord(taskID, action, status, requestID string, startedAt time.Time, detail string, err error, record *sheinproduct.RecordItem) ConfirmRemoteParts {
	parts := BuildConfirmRemoteParts(taskID, action, status, requestID, startedAt, detail, err)
	parts.Record = record
	if record != nil {
		parts.Event.RemoteRecordID = record.RecordID
	}
	return parts
}

func BuildRefreshConfirmRemoteRunningEvent(taskID, action, requestID string, startedAt time.Time) sheinpub.SubmissionEvent {
	return BuildConfirmRemoteEvent(taskID, action, sheinpub.SubmissionStatusRunning, requestID, startedAt, "刷新 SHEIN 远端提交状态", nil)
}

func ApplyConfirmRemoteParts(pkg *sheinpub.Package, action, requestID string, parts ConfirmRemoteParts) {
	if pkg == nil {
		return
	}
	AppendEvent(pkg, parts.Event)
	SetRemoteRecord(pkg, action, requestID, parts.RemoteStatus, parts.Record, parts.CheckedAt, parts.Message)
}

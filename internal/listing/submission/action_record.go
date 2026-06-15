package submission

import (
	"strings"
	"time"
)

type ActionRecordSlots[Record any] struct {
	SaveDraft *Record
	Publish   *Record
}

type ReportState[Record any, Result any] struct {
	LastAction  string
	LastStatus  string
	LastError   string
	SubmittedAt *time.Time
	LastResult  *Result
	Slots       ActionRecordSlots[Record]
}

type ReportRecordState[Result any] struct {
	Action      string
	Status      string
	Error       string
	SubmittedAt time.Time
	Result      *Result
}

type ActionRecordView struct {
	RequestID  string
	Status     string
	FinishedAt *time.Time
}

func RecordForAction[Record any](slots ActionRecordSlots[Record], action string) *Record {
	switch action {
	case "save_draft":
		return slots.SaveDraft
	case "publish":
		return slots.Publish
	default:
		return nil
	}
}

func SetRecordForAction[Record any](slots *ActionRecordSlots[Record], action string, record *Record) {
	if slots == nil {
		return
	}
	switch action {
	case "save_draft":
		slots.SaveDraft = record
	case "publish":
		slots.Publish = record
	}
}

func ApplyRecordState[Record any, Result any](report *ReportState[Record, Result], record *Record, state ReportRecordState[Result]) {
	if report == nil || record == nil {
		return
	}
	report.LastAction = state.Action
	report.LastStatus = state.Status
	report.LastError = state.Error
	report.SubmittedAt = &state.SubmittedAt
	report.LastResult = state.Result
	SetRecordForAction(&report.Slots, state.Action, record)
}

func ActionSucceeded[Record any](slots ActionRecordSlots[Record], action string, view func(*Record) ActionRecordView, successStatus string) bool {
	record := RecordForAction(slots, action)
	if record == nil {
		return false
	}
	return view(record).Status == successStatus
}

func FindRecordByRequestID[Record any](slots ActionRecordSlots[Record], action, requestID string, view func(*Record) ActionRecordView) *Record {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return nil
	}
	record := RecordForAction(slots, action)
	if record == nil {
		return nil
	}
	recordView := view(record)
	if strings.TrimSpace(recordView.RequestID) != requestID {
		return nil
	}
	return record
}

func FindRecordByRequestIDAndStatus[Record any](slots ActionRecordSlots[Record], action, requestID string, view func(*Record) ActionRecordView, status string) *Record {
	record := FindRecordByRequestID(slots, action, requestID, view)
	if record == nil {
		return nil
	}
	if view(record).Status != status {
		return nil
	}
	return record
}

func FindCompletedRecordByRequestID[Record any](slots ActionRecordSlots[Record], action, requestID string, view func(*Record) ActionRecordView) *Record {
	record := FindRecordByRequestID(slots, action, requestID, view)
	if record == nil {
		return nil
	}
	if view(record).FinishedAt == nil {
		return nil
	}
	return record
}

func MutateMatchingRecord[Record any](slots ActionRecordSlots[Record], action, requestID string, view func(*Record) ActionRecordView, mutate func(*Record)) bool {
	if mutate == nil {
		return false
	}
	record := RecordForAction(slots, action)
	if record == nil {
		return false
	}
	recordView := view(record)
	if recordView.RequestID != requestID {
		return false
	}
	mutate(record)
	return true
}

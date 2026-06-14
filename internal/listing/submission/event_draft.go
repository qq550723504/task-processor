package submission

import "time"

type AttemptEventDraft struct {
	Status          string
	RequestID       string
	Phase           string
	RemoteRecordID  string
	ErrorMessage    string
	ValidationNotes []string
	FinishedAt      time.Time
}

func BuildAttemptEventDraft(recordState *EventRecordState, outcome *ResponseOutcome, fallbackOutcome *ResponseOutcome, submitErr error, finishedAt time.Time) AttemptEventDraft {
	resolved := ResolveEventOutcome(recordState, outcome, fallbackOutcome, submitErr)
	return AttemptEventDraft{
		Status:          resolved.Status,
		RequestID:       resolved.RequestID,
		Phase:           resolved.Phase,
		RemoteRecordID:  resolved.RemoteRecordID,
		ErrorMessage:    resolved.ErrorMessage,
		ValidationNotes: append([]string(nil), resolved.ValidationNotes...),
		FinishedAt:      finishedAt,
	}
}

type PhaseEventDraft struct {
	Status       string
	Detail       string
	ErrorMessage string
	FinishedAt   time.Time
}

func BuildPhaseEventDraft(status, detail, fallbackDetail string, err error, finishedAt time.Time) PhaseEventDraft {
	state := ResolvePhaseEventState(status, detail, fallbackDetail, err)
	return PhaseEventDraft{
		Status:       state.Status,
		Detail:       state.Detail,
		ErrorMessage: state.ErrorMessage,
		FinishedAt:   finishedAt,
	}
}

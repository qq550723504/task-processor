package submission

import "time"

type AttemptRecordDraft struct {
	Action      string
	SubmittedAt time.Time
	Status      string
	Error       string
}

func BuildAttemptRecordDraft(action string, outcome *ResponseOutcome, submitErr error, submittedAt time.Time) AttemptRecordDraft {
	state := ResolveAttemptResultState(action, outcome, submitErr)
	return AttemptRecordDraft{
		Action:      action,
		SubmittedAt: submittedAt,
		Status:      state.Status,
		Error:       state.ErrorMessage,
	}
}

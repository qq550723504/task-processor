package submission

import "time"

type AttemptFinalizeState struct {
	Status       string
	ErrorMessage string
	FinishedAt   time.Time
}

func ResolveAttemptFinalizeState(action string, outcome *ResponseOutcome, submitErr error, finishedAt time.Time) AttemptFinalizeState {
	result := ResolveAttemptResultState(action, outcome, submitErr)
	return AttemptFinalizeState{
		Status:       result.Status,
		ErrorMessage: result.ErrorMessage,
		FinishedAt:   finishedAt,
	}
}

func ResolveAttemptFailureFinalizeState(phase string, submitErr error, finishedAt time.Time) AttemptFinalizeState {
	state := AttemptFinalizeState{
		Status:     "failed",
		FinishedAt: finishedAt,
	}
	if submitErr != nil {
		state.ErrorMessage = submitErr.Error()
	}
	return state
}

package submission

type AttemptResultState struct {
	Status       string
	ErrorMessage string
}

func ResolveAttemptResultState(action string, outcome *ResponseOutcome, submitErr error) AttemptResultState {
	if submitErr != nil {
		return AttemptResultState{
			Status:       "failed",
			ErrorMessage: submitErr.Error(),
		}
	}
	if outcome != nil && (outcome.Success || SaveDraftSucceeded(action, outcome)) {
		return AttemptResultState{Status: "success"}
	}
	return AttemptResultState{Status: "unknown"}
}

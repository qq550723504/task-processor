package submission

type PhaseEventState struct {
	Status       string
	Detail       string
	ErrorMessage string
}

func ResolvePhaseEventState(status, detail, defaultDetail string, err error) PhaseEventState {
	out := PhaseEventState{
		Status: status,
		Detail: detail,
	}
	if out.Status == "" {
		out.Status = "running"
	}
	if out.Detail == "" {
		out.Detail = defaultDetail
	}
	if err != nil {
		out.ErrorMessage = err.Error()
	}
	return out
}

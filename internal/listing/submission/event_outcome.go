package submission

type EventRecordState struct {
	Status         string
	RequestID      string
	Phase          string
	RemoteRecordID string
}

type EventOutcome struct {
	Status          string
	RequestID       string
	Phase           string
	RemoteRecordID  string
	ErrorMessage    string
	ValidationNotes []string
}

func ResolveEventOutcome(record *EventRecordState, explicitOutcome, fallbackOutcome *ResponseOutcome, submitErr error) EventOutcome {
	out := EventOutcome{Status: "unknown"}
	if record != nil {
		out.Status = record.Status
		out.RequestID = record.RequestID
		out.Phase = record.Phase
		out.RemoteRecordID = record.RemoteRecordID
	}

	outcome := explicitOutcome
	if outcome == nil {
		outcome = fallbackOutcome
	}
	if outcome != nil {
		out.ValidationNotes = append([]string(nil), outcome.ValidationNotes...)
	}
	if submitErr != nil {
		out.Status = "failed"
		out.ErrorMessage = submitErr.Error()
	}
	return out
}

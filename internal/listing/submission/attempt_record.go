package submission

import "time"

type AttemptSeedState struct {
	AttemptCount      int
	InFlightStartedAt *time.Time
}

type AttemptRecordSeed struct {
	Action      string
	RequestID   string
	SubmittedAt time.Time
	StartedAt   time.Time
	Attempt     int
}

func ResolveAttemptStartedAt(fallback time.Time, inFlightStartedAt *time.Time) time.Time {
	if inFlightStartedAt != nil {
		return *inFlightStartedAt
	}
	return fallback
}

func ResolveAttemptRecordForRequest[Record any](
	slots ActionRecordSlots[Record],
	action string,
	requestID string,
	view func(*Record) ActionRecordView,
	build func(AttemptRecordSeed) *Record,
	seedState AttemptSeedState,
	fallbackStartedAt time.Time,
) *Record {
	record := RecordForAction(slots, action)
	if record != nil && view(record).RequestID == requestID {
		return record
	}
	if build == nil {
		return nil
	}
	startedAt := ResolveAttemptStartedAt(fallbackStartedAt, seedState.InFlightStartedAt)
	return build(AttemptRecordSeed{
		Action:      action,
		RequestID:   requestID,
		SubmittedAt: startedAt,
		StartedAt:   startedAt,
		Attempt:     seedState.AttemptCount,
	})
}

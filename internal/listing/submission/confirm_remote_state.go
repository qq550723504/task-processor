package submission

import "time"

type ConfirmRemoteState struct {
	CheckedAt           time.Time
	Message             string
	EventRemoteRecordID string
}

func BuildConfirmRemoteState(detail, eventRemoteRecordID, recordRemoteRecordID string, checkedAt time.Time) ConfirmRemoteState {
	return ConfirmRemoteState{
		CheckedAt:           checkedAt,
		Message:             detail,
		EventRemoteRecordID: ResolveRemoteRecordID(eventRemoteRecordID, recordRemoteRecordID),
	}
}

package submission

import "time"

type ActionRemoteSyncState struct {
	RemoteStatus string
	CheckedAt    time.Time
}

func ApplyRemoteSync[Record any](
	slots ActionRecordSlots[Record],
	action string,
	requestID string,
	view func(*Record) ActionRecordView,
	state ActionRemoteSyncState,
	applyReport func(ActionRemoteSyncState),
	mutateRecord func(*Record, ActionRemoteSyncState),
) bool {
	if applyReport != nil {
		applyReport(state)
	}
	if mutateRecord == nil {
		return false
	}
	return MutateMatchingRecord(
		slots,
		action,
		requestID,
		view,
		func(record *Record) {
			mutateRecord(record, state)
		},
	)
}

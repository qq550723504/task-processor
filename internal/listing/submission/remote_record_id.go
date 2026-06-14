package submission

func ResolveRemoteRecordID(eventRemoteRecordID, recordRemoteRecordID string) string {
	if eventRemoteRecordID != "" {
		return eventRemoteRecordID
	}
	return recordRemoteRecordID
}

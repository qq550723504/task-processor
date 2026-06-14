package shein

import sheinmarketplace "task-processor/internal/marketplace/shein/workspace"

type HistoryDetail[Record any, Payload any] = sheinmarketplace.HistoryDetail[Record, Payload]

func BuildHistoryDetail[Record any, Payload any](
	taskID string,
	record *Record,
	navigation *HistoryNavigation,
	restorePayload *Payload,
	historyIndex int,
	totalRecords int,
	isTruncated bool,
	maxRecords int,
) *HistoryDetail[Record, Payload] {
	return sheinmarketplace.BuildHistoryDetail(taskID, record, navigation, restorePayload, historyIndex, totalRecords, isTruncated, maxRecords)
}

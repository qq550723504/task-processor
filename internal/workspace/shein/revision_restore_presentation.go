package shein

import sheinmarketplace "task-processor/internal/marketplace/shein/workspace"

type HistoryRestoreRecommendedView = sheinmarketplace.HistoryRestoreRecommendedView
type HistoryRestorePresentationData = sheinmarketplace.HistoryRestorePresentationData

func BuildHistoryRestorePresentationData(
	record *HistoryRestoreRecordInput,
	context *HistoryRestoreContext,
	safety *HistoryRestoreSafety,
	compare *HistoryRestoreCompareInput,
) *HistoryRestorePresentationData {
	return sheinmarketplace.BuildHistoryRestorePresentationData(record, context, safety, compare)
}

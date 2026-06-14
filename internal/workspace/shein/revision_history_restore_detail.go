package shein

import sheinmarketplace "task-processor/internal/marketplace/shein/workspace"

type HistoryRestoreTimeline = sheinmarketplace.HistoryRestoreTimeline
type HistoryRestoreRecordInput = sheinmarketplace.HistoryRestoreRecordInput
type HistoryRestoreCompareInput = sheinmarketplace.HistoryRestoreCompareInput
type HistoryRestoreStateInput = sheinmarketplace.HistoryRestoreStateInput
type HistoryRestoreContext = sheinmarketplace.HistoryRestoreContext
type HistoryRestoreSafety = sheinmarketplace.HistoryRestoreSafety
type HistoryRestoreOverview = sheinmarketplace.HistoryRestoreOverview
type HistoryRestoreMessages = sheinmarketplace.HistoryRestoreMessages

func BuildHistoryRestoreContext(record *HistoryRestoreRecordInput, executionMode, platform, reason string, compare *HistoryRestoreCompareInput) *HistoryRestoreContext {
	return sheinmarketplace.BuildHistoryRestoreContext(record, executionMode, platform, reason, compare)
}

func BuildHistoryRestoreSafety(state *HistoryRestoreStateInput, record *HistoryRestoreRecordInput, draft *EditorRevisionSkeleton, compare *HistoryRestoreCompareInput) *HistoryRestoreSafety {
	return sheinmarketplace.BuildHistoryRestoreSafety(state, record, draft, compare)
}

func BuildHistoryRestoreOverview(record *HistoryRestoreRecordInput, safety *HistoryRestoreSafety, compare *HistoryRestoreCompareInput) *HistoryRestoreOverview {
	return sheinmarketplace.BuildHistoryRestoreOverview(record, safety, compare)
}

func BuildHistoryRestoreMessages(context *HistoryRestoreContext, safety *HistoryRestoreSafety, overview *HistoryRestoreOverview) *HistoryRestoreMessages {
	return sheinmarketplace.BuildHistoryRestoreMessages(context, safety, overview)
}

func BuildHistoryRestoreNextActions(record *HistoryRestoreRecordInput, safety *HistoryRestoreSafety, compare *HistoryRestoreCompareInput) []string {
	return sheinmarketplace.BuildHistoryRestoreNextActions(record, safety, compare)
}

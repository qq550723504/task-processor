package shein

import sheinmarketplace "task-processor/internal/marketplace/shein/workspace"

type HistoryRestoreDetailData[Req any, Compare any] = sheinmarketplace.HistoryRestoreDetailData[Req, Compare]

func BuildHistoryRestoreDetailData[Req any, Compare any](
	record *HistoryRestoreRecordInput,
	state *HistoryRestoreStateInput,
	draft *EditorRevisionSkeleton,
	revisionPayload *Req,
	executionMode string,
	platform string,
	reason string,
	compareInput *HistoryRestoreCompareInput,
	compareValue *Compare,
) *HistoryRestoreDetailData[Req, Compare] {
	return sheinmarketplace.BuildHistoryRestoreDetailData(record, state, draft, revisionPayload, executionMode, platform, reason, compareInput, compareValue)
}

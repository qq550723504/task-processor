package shein

import sheinmarketplace "task-processor/internal/marketplace/shein/workspace"

type HistoryCompareRecord = sheinmarketplace.HistoryCompareRecord
type HistoryComparePreview = sheinmarketplace.HistoryComparePreview
type HistoryNavigation = sheinmarketplace.HistoryNavigation
type HistoryDetail[Record any, Payload any] = sheinmarketplace.HistoryDetail[Record, Payload]
type HistoryRestoreTimeline = sheinmarketplace.HistoryRestoreTimeline
type HistoryRestoreRecordInput = sheinmarketplace.HistoryRestoreRecordInput
type HistoryRestoreCompareInput = sheinmarketplace.HistoryRestoreCompareInput
type HistoryRestoreStateInput = sheinmarketplace.HistoryRestoreStateInput
type HistoryRestoreContext = sheinmarketplace.HistoryRestoreContext
type HistoryRestoreSafety = sheinmarketplace.HistoryRestoreSafety
type HistoryRestoreOverview = sheinmarketplace.HistoryRestoreOverview
type HistoryRestoreMessages = sheinmarketplace.HistoryRestoreMessages
type HistoryRestoreRecommendedView = sheinmarketplace.HistoryRestoreRecommendedView
type HistoryRestorePresentationData = sheinmarketplace.HistoryRestorePresentationData
type RestorePreviewCoreData[Req any, Ctx any, Safety any, Compare any] = sheinmarketplace.RestorePreviewCoreData[Req, Ctx, Safety, Compare]
type RestorePreviewPayload[Req any, Ctx any, Safety any, Compare any, Pres any] = sheinmarketplace.RestorePreviewPayload[Req, Ctx, Safety, Compare, Pres]
type HistoryRestoreDetailData[Req any, Compare any] = sheinmarketplace.HistoryRestoreDetailData[Req, Compare]

func ResolveHistoryCompareTarget(records []HistoryCompareRecord, currentIndex int, compareTo string) (HistoryCompareRecord, int, string, bool) {
	return sheinmarketplace.ResolveHistoryCompareTarget(records, currentIndex, compareTo)
}

func BuildHistoryComparePreview(current *HistoryCompareRecord, compareTo string, compare *HistoryCompareRecord, relationLabel string) *HistoryComparePreview {
	return sheinmarketplace.BuildHistoryComparePreview(current, compareTo, compare, relationLabel)
}

func BuildCurrentHistoryComparePreview(record *HistoryCompareRecord, currentDraft *EditorRevisionSkeleton) *HistoryComparePreview {
	return sheinmarketplace.BuildCurrentHistoryComparePreview(record, currentDraft)
}

func BuildHistoryNavigation(prevRevisionID, nextRevisionID string) *HistoryNavigation {
	return sheinmarketplace.BuildHistoryNavigation(prevRevisionID, nextRevisionID)
}

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

func BuildHistoryRestorePresentationData(
	record *HistoryRestoreRecordInput,
	context *HistoryRestoreContext,
	safety *HistoryRestoreSafety,
	compare *HistoryRestoreCompareInput,
) *HistoryRestorePresentationData {
	return sheinmarketplace.BuildHistoryRestorePresentationData(record, context, safety, compare)
}

func BuildRestorePreviewPayload[Req any, Ctx any, Safety any, Compare any, Pres any](
	draft *EditorRevisionSkeleton,
	revisionPayload *Req,
	context *Ctx,
	safety *Safety,
	compare *Compare,
	presentation *Pres,
) *RestorePreviewPayload[Req, Ctx, Safety, Compare, Pres] {
	return sheinmarketplace.BuildRestorePreviewPayload(draft, revisionPayload, context, safety, compare, presentation)
}

func RebuildRestorePreviewPayload[Req any, Ctx any, Safety any, Compare any, Pres any](
	src *RestorePreviewPayload[Req, Ctx, Safety, Compare, Pres],
	compare *Compare,
) *RestorePreviewPayload[Req, Ctx, Safety, Compare, Pres] {
	return sheinmarketplace.RebuildRestorePreviewPayload(src, compare)
}

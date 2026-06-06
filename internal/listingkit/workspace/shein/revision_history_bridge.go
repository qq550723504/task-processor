package shein

import sheinworkspace "task-processor/internal/workspace/shein"

type HistoryCompareRecord = sheinworkspace.HistoryCompareRecord
type HistoryComparePreview = sheinworkspace.HistoryComparePreview
type HistoryNavigation = sheinworkspace.HistoryNavigation
type HistoryDetail[Record any, Payload any] = sheinworkspace.HistoryDetail[Record, Payload]
type HistoryRestoreTimeline = sheinworkspace.HistoryRestoreTimeline
type HistoryRestoreRecordInput = sheinworkspace.HistoryRestoreRecordInput
type HistoryRestoreCompareInput = sheinworkspace.HistoryRestoreCompareInput
type HistoryRestoreStateInput = sheinworkspace.HistoryRestoreStateInput
type HistoryRestoreContext = sheinworkspace.HistoryRestoreContext
type HistoryRestoreSafety = sheinworkspace.HistoryRestoreSafety
type HistoryRestoreOverview = sheinworkspace.HistoryRestoreOverview
type HistoryRestoreMessages = sheinworkspace.HistoryRestoreMessages
type HistoryRestoreRecommendedView = sheinworkspace.HistoryRestoreRecommendedView
type HistoryRestorePresentationData = sheinworkspace.HistoryRestorePresentationData
type RestorePreviewCoreData[Req any, Ctx any, Safety any, Compare any] = sheinworkspace.RestorePreviewCoreData[Req, Ctx, Safety, Compare]
type RestorePreviewPayload[Req any, Ctx any, Safety any, Compare any, Pres any] = sheinworkspace.RestorePreviewPayload[Req, Ctx, Safety, Compare, Pres]
type HistoryRestoreDetailData[Req any, Compare any] = sheinworkspace.HistoryRestoreDetailData[Req, Compare]

func ResolveHistoryCompareTarget(records []HistoryCompareRecord, currentIndex int, compareTo string) (HistoryCompareRecord, int, string, bool) {
	return sheinworkspace.ResolveHistoryCompareTarget(records, currentIndex, compareTo)
}

func BuildHistoryComparePreview(current *HistoryCompareRecord, compareTo string, compare *HistoryCompareRecord, relationLabel string) *HistoryComparePreview {
	return sheinworkspace.BuildHistoryComparePreview(current, compareTo, compare, relationLabel)
}

func BuildCurrentHistoryComparePreview(record *HistoryCompareRecord, currentDraft *EditorRevisionSkeleton) *HistoryComparePreview {
	return sheinworkspace.BuildCurrentHistoryComparePreview(record, currentDraft)
}

func BuildHistoryNavigation(prevRevisionID, nextRevisionID string) *HistoryNavigation {
	return sheinworkspace.BuildHistoryNavigation(prevRevisionID, nextRevisionID)
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
	return sheinworkspace.BuildHistoryDetail(taskID, record, navigation, restorePayload, historyIndex, totalRecords, isTruncated, maxRecords)
}

func BuildHistoryRestoreContext(record *HistoryRestoreRecordInput, executionMode, platform, reason string, compare *HistoryRestoreCompareInput) *HistoryRestoreContext {
	return sheinworkspace.BuildHistoryRestoreContext(record, executionMode, platform, reason, compare)
}

func BuildHistoryRestoreSafety(state *HistoryRestoreStateInput, record *HistoryRestoreRecordInput, draft *EditorRevisionSkeleton, compare *HistoryRestoreCompareInput) *HistoryRestoreSafety {
	return sheinworkspace.BuildHistoryRestoreSafety(state, record, draft, compare)
}

func BuildHistoryRestoreOverview(record *HistoryRestoreRecordInput, safety *HistoryRestoreSafety, compare *HistoryRestoreCompareInput) *HistoryRestoreOverview {
	return sheinworkspace.BuildHistoryRestoreOverview(record, safety, compare)
}

func BuildHistoryRestoreMessages(context *HistoryRestoreContext, safety *HistoryRestoreSafety, overview *HistoryRestoreOverview) *HistoryRestoreMessages {
	return sheinworkspace.BuildHistoryRestoreMessages(context, safety, overview)
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
	return sheinworkspace.BuildHistoryRestoreDetailData(record, state, draft, revisionPayload, executionMode, platform, reason, compareInput, compareValue)
}

func BuildHistoryRestorePresentationData(
	record *HistoryRestoreRecordInput,
	context *HistoryRestoreContext,
	safety *HistoryRestoreSafety,
	compare *HistoryRestoreCompareInput,
) *HistoryRestorePresentationData {
	return sheinworkspace.BuildHistoryRestorePresentationData(record, context, safety, compare)
}

func BuildRestorePreviewPayload[Req any, Ctx any, Safety any, Compare any, Pres any](
	draft *EditorRevisionSkeleton,
	revisionPayload *Req,
	context *Ctx,
	safety *Safety,
	compare *Compare,
	presentation *Pres,
) *RestorePreviewPayload[Req, Ctx, Safety, Compare, Pres] {
	return sheinworkspace.BuildRestorePreviewPayload(draft, revisionPayload, context, safety, compare, presentation)
}

func RebuildRestorePreviewPayload[Req any, Ctx any, Safety any, Compare any, Pres any](
	src *RestorePreviewPayload[Req, Ctx, Safety, Compare, Pres],
	compare *Compare,
) *RestorePreviewPayload[Req, Ctx, Safety, Compare, Pres] {
	return sheinworkspace.RebuildRestorePreviewPayload(src, compare)
}

package listingkit

import (
	sheinworkspace "task-processor/internal/listingkit/workspace/shein"
	sheindiff "task-processor/internal/marketplace/shein/workspace"
)

type RevisionHistoryNavigation = sheinworkspace.HistoryNavigation
type RevisionHistoryRestoreContext = sheinworkspace.HistoryRestoreContext
type RevisionHistoryRestoreSafety = sheinworkspace.HistoryRestoreSafety
type RevisionHistoryRestoreMessages = sheinworkspace.HistoryRestoreMessages
type RevisionHistoryRestoreOverview = sheinworkspace.HistoryRestoreOverview
type RevisionRestorePreviewCoreData = sheinworkspace.RestorePreviewCoreData[ApplyRevisionRequest, RevisionHistoryRestoreContext, RevisionHistoryRestoreSafety, RevisionHistoryComparePreview]
type RevisionRestorePreviewPayload = sheinworkspace.RestorePreviewPayload[ApplyRevisionRequest, RevisionHistoryRestoreContext, RevisionHistoryRestoreSafety, RevisionHistoryComparePreview, RevisionInteractionPresentation]
type revisionHistoryRestoreDetailData = sheinworkspace.HistoryRestoreDetailData[ApplyRevisionRequest, RevisionHistoryComparePreview]

func buildRevisionHistoryNavigation(prevRevisionID, nextRevisionID string) *RevisionHistoryNavigation {
	return sheinworkspace.BuildHistoryNavigation(prevRevisionID, nextRevisionID)
}

func buildRevisionHistoryRestoreContext(record *ListingKitRevisionRecord, payload *ApplyRevisionRequest, comparePreview *RevisionHistoryComparePreview) *RevisionHistoryRestoreContext {
	return sheinworkspace.BuildHistoryRestoreContext(
		buildRevisionHistoryRestoreRecordInput(record),
		"restore_from_revision_id",
		revisionPayloadPlatform(payload),
		revisionPayloadReason(payload),
		buildRevisionHistoryRestoreCompareInput(comparePreview),
	)
}

func buildRevisionHistoryRestoreSafety(result *ListingKitResult, record *ListingKitRevisionRecord, restoreDraft *SheinEditorRevisionSkeleton, comparePreview *RevisionHistoryComparePreview) *RevisionHistoryRestoreSafety {
	return sheinworkspace.BuildHistoryRestoreSafety(
		buildRevisionHistoryRestoreStateInput(result),
		buildRevisionHistoryRestoreRecordInput(record),
		restoreDraft,
		buildRevisionHistoryRestoreCompareInput(comparePreview),
	)
}

func buildRevisionHistoryRestoreOverview(record *ListingKitRevisionRecord, safety *RevisionHistoryRestoreSafety, comparePreview *RevisionHistoryComparePreview) *RevisionHistoryRestoreOverview {
	return sheinworkspace.BuildHistoryRestoreOverview(
		buildRevisionHistoryRestoreRecordInput(record),
		safety,
		buildRevisionHistoryRestoreCompareInput(comparePreview),
	)
}

func buildRevisionHistoryRestoreMessages(record *ListingKitRevisionRecord, context *RevisionHistoryRestoreContext, safety *RevisionHistoryRestoreSafety, overview *RevisionHistoryRestoreOverview) *RevisionHistoryRestoreMessages {
	_ = record
	return sheinworkspace.BuildHistoryRestoreMessages(context, safety, overview)
}

func buildRevisionHistoryRestoreDetailData(
	result *ListingKitResult,
	record *ListingKitRevisionRecord,
	draft *SheinEditorRevisionSkeleton,
	revisionPayload *ApplyRevisionRequest,
	comparePreview *RevisionHistoryComparePreview,
) *revisionHistoryRestoreDetailData {
	return sheinworkspace.BuildHistoryRestoreDetailData(
		buildRevisionHistoryRestoreRecordInput(record),
		buildRevisionHistoryRestoreStateInput(result),
		draft,
		revisionPayload,
		"restore_from_revision_id",
		revisionPayloadPlatform(revisionPayload),
		revisionPayloadReason(revisionPayload),
		buildRevisionHistoryRestoreCompareInput(comparePreview),
		comparePreview,
	)
}

func buildRevisionHistoryDetailRestorePayload(
	record *ListingKitRevisionRecord,
	draft *SheinEditorRevisionSkeleton,
	revisionPayload *ApplyRevisionRequest,
	context *RevisionHistoryRestoreContext,
	safety *RevisionHistoryRestoreSafety,
	presentation *RevisionInteractionPresentation,
	compare *RevisionHistoryComparePreview,
) *RevisionRestorePreviewPayload {
	_ = record
	return sheinworkspace.BuildRestorePreviewPayload(draft, revisionPayload, context, safety, compare, presentation)
}

func buildRevisionRestorePreviewFromDetail(detail *ListingKitRevisionHistoryDetail) *RevisionRestorePreviewPayload {
	if detail == nil || detail.RestorePayload == nil {
		return nil
	}
	compare := detail.RestorePayload.Core.Compare
	if compare == nil && detail.Record != nil {
		compare = &RevisionHistoryComparePreview{
			CompareTo:         "current",
			CompareRevisionID: "current",
			RelationLabel:     "当前版本",
			DiffPreview:       sheindiff.BuildRevisionDiffPreviewFromInput(detail.RestorePayload.Core.Draft),
		}
	}
	return sheinworkspace.RebuildRestorePreviewPayload(detail.RestorePayload, compare)
}

func buildRevisionHistoryRestorePresentationData(
	record *ListingKitRevisionRecord,
	context *RevisionHistoryRestoreContext,
	safety *RevisionHistoryRestoreSafety,
	comparePreview *RevisionHistoryComparePreview,
) *sheinworkspace.HistoryRestorePresentationData {
	return sheinworkspace.BuildHistoryRestorePresentationData(
		buildRevisionHistoryRestoreRecordInput(record),
		context,
		safety,
		buildRevisionHistoryRestoreCompareInput(comparePreview),
	)
}

func convertHistoryRestoreRecommendedView(view *sheinworkspace.HistoryRestoreRecommendedView) *RevisionRecommendedView {
	if view == nil {
		return nil
	}
	return &RevisionRecommendedView{
		View:   view.View,
		Reason: view.Reason,
	}
}

func buildRevisionHistoryRestoreRecordInput(record *ListingKitRevisionRecord) *sheinworkspace.HistoryRestoreRecordInput {
	if record == nil {
		return nil
	}
	input := &sheinworkspace.HistoryRestoreRecordInput{
		RevisionID:             record.RevisionID,
		Platform:               record.Platform,
		ActionType:             record.ActionType,
		RestoredFromRevisionID: record.RestoredFromRevisionID,
	}
	if record.Timeline != nil {
		input.Timeline = &sheinworkspace.HistoryRestoreTimeline{
			Headline:     record.Timeline.Headline,
			RelationText: record.Timeline.RelationText,
		}
	}
	return input
}

func buildRevisionHistoryRestoreCompareInput(compare *RevisionHistoryComparePreview) *sheinworkspace.HistoryRestoreCompareInput {
	if compare == nil {
		return nil
	}
	input := &sheinworkspace.HistoryRestoreCompareInput{
		CompareTo:         compare.CompareTo,
		CompareRevisionID: compare.CompareRevisionID,
		RelationLabel:     compare.RelationLabel,
	}
	if compare.DiffPreview != nil {
		input.ChangeCount = compare.DiffPreview.ChangeCount
	}
	return input
}

func buildRevisionHistoryRestoreStateInput(result *ListingKitResult) *sheinworkspace.HistoryRestoreStateInput {
	if result == nil || result.Shein == nil {
		return &sheinworkspace.HistoryRestoreStateInput{}
	}
	return &sheinworkspace.HistoryRestoreStateInput{
		HasCurrentPackage:     true,
		CategoryResolved:      isSheinCategoryResolved(result.Shein),
		AttributeResolved:     isSheinAttributeResolved(result.Shein),
		SaleAttributeResolved: isSheinSaleAttributeResolved(result.Shein),
		ManualReviewNotes:     filterManualSheinReviewNotes(result.Shein.ReviewNotes),
	}
}

func revisionPayloadPlatform(payload *ApplyRevisionRequest) string {
	if payload == nil {
		return ""
	}
	return payload.Platform
}

func revisionPayloadReason(payload *ApplyRevisionRequest) string {
	if payload == nil {
		return ""
	}
	return payload.Reason
}

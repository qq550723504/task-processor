package listingkit

import (
	"errors"
	"strings"

	sheinworkspace "task-processor/internal/listingkit/workspace/shein"
)

var ErrRevisionHistoryRecordNotFound = errors.New("revision history record not found")

type ListingKitRevisionHistoryDetail = sheinworkspace.HistoryDetail[ListingKitRevisionRecord, RevisionRestorePreviewPayload]

type RevisionHistoryDetailQuery struct {
	CompareTo string `form:"compare_to"`
}

func buildRevisionHistoryDetail(result *ListingKitResult, revisionID string, query *RevisionHistoryDetailQuery) (*ListingKitRevisionHistoryDetail, error) {
	if result == nil {
		return nil, ErrTaskResultUnavailable
	}
	normalized := normalizeRevisionHistoryDetailQuery(query)

	for i, record := range result.RevisionHistory {
		recordWithID := withRevisionHistoryRecordID(record, i)
		if recordWithID.RevisionID != revisionID {
			continue
		}
		total := result.RevisionHistoryTotal
		if total == 0 && len(result.RevisionHistory) > 0 {
			total = len(result.RevisionHistory)
		}
		restoreDraft := buildRevisionHistoryRestoreDraft(&recordWithID)
		restoreRevisionPayload := buildRevisionHistoryRestorePayload(&recordWithID)
		comparePreview, err := buildRevisionHistoryComparePreview(result, i, normalized.CompareTo)
		if err != nil {
			return nil, err
		}
		restoreDetail := buildRevisionHistoryRestoreDetailData(result, &recordWithID, restoreDraft, restoreRevisionPayload, comparePreview)
		restorePresentation := buildRevisionRestorePreviewPresentation(
			&recordWithID,
			restoreDetailContextValue(restoreDetail),
			restoreDetailSafetyValue(restoreDetail),
			comparePreview,
		)
		return sheinworkspace.BuildHistoryDetail(
			result.TaskID,
			&recordWithID,
			buildRevisionHistoryNavigation(
				buildAdjacentRevisionID(result.RevisionHistory, i-1),
				buildAdjacentRevisionID(result.RevisionHistory, i+1),
			),
			buildRevisionHistoryDetailRestorePayload(
				&recordWithID,
				restoreDraft,
				restoreRevisionPayload,
				restoreDetailContextValue(restoreDetail),
				restoreDetailSafetyValue(restoreDetail),
				restorePresentation,
				comparePreview,
			),
			i,
			total,
			total > len(result.RevisionHistory),
			maxRevisionHistoryRecords,
		), nil
	}

	return nil, ErrRevisionHistoryRecordNotFound
}

func buildRevisionHistoryRestorePayload(record *ListingKitRevisionRecord) *ApplyRevisionRequest {
	if record == nil {
		return nil
	}
	revisionID := record.RevisionID
	if revisionID == "" {
		return nil
	}
	return &ApplyRevisionRequest{
		Platform:              firstNonEmpty(record.Platform, "shein"),
		Actor:                 "desktop-client",
		Reason:                buildRevisionHistoryRestoreReason(record),
		RestoreFromRevisionID: revisionID,
	}
}

func buildRevisionRestorePreviewPresentation(
	record *ListingKitRevisionRecord,
	context *RevisionHistoryRestoreContext,
	safety *RevisionHistoryRestoreSafety,
	comparePreview *RevisionHistoryComparePreview,
) *RevisionInteractionPresentation {
	data := buildRevisionHistoryRestorePresentationData(record, context, safety, comparePreview)
	if data == nil {
		return nil
	}

	summaryCard := &RevisionSuccessSummaryCard{
		Status:        data.Status,
		Title:         data.Headline,
		Subtitle:      data.Subheadline,
		PrimaryAction: data.PrimaryAction,
		Highlights:    append([]string(nil), data.Highlights...),
	}

	return sheinworkspace.BuildSuccessPresentation(
		revisionPresentationSceneRestorePreview,
		append([]string(nil), data.NextActions...),
		&RevisionResultMessages{
			Title:            data.Title,
			Description:      data.Description,
			SuccessLabel:     data.ConfirmLabel,
			WarningTitle:     data.WarningTitle,
			WarningSummaries: append([]string(nil), data.WarningSummaries...),
		},
		convertHistoryRestoreRecommendedView(data.RecommendedView),
		summaryCard,
	)
}

func normalizeRevisionHistoryDetailQuery(query *RevisionHistoryDetailQuery) RevisionHistoryDetailQuery {
	if query == nil {
		return RevisionHistoryDetailQuery{}
	}
	return RevisionHistoryDetailQuery{
		CompareTo: strings.TrimSpace(query.CompareTo),
	}
}

func restoreDetailContextValue(detail *revisionHistoryRestoreDetailData) *RevisionHistoryRestoreContext {
	if detail == nil {
		return nil
	}
	return detail.Context
}

func restoreDetailSafetyValue(detail *revisionHistoryRestoreDetailData) *RevisionHistoryRestoreSafety {
	if detail == nil {
		return nil
	}
	return detail.Safety
}

func buildAdjacentRevisionID(records []ListingKitRevisionRecord, index int) string {
	if index < 0 || index >= len(records) {
		return ""
	}
	return withRevisionHistoryRecordID(records[index], index).RevisionID
}

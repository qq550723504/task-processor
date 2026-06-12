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
	records := buildRevisionHistoryRecords(result.RevisionHistory)

	for i, recordWithID := range records {
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
				buildAdjacentRevisionID(records, i-1),
				buildAdjacentRevisionID(records, i+1),
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
	return buildRevisionRestorePreviewPresentationValue(record, context, safety, comparePreview)
}

func normalizeRevisionHistoryDetailQuery(query *RevisionHistoryDetailQuery) RevisionHistoryDetailQuery {
	if query == nil {
		return RevisionHistoryDetailQuery{}
	}
	return RevisionHistoryDetailQuery{
		CompareTo: strings.TrimSpace(query.CompareTo),
	}
}

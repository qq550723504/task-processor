package listingkit

import (
	"errors"

	sheinworkspace "task-processor/internal/workspace/shein"
)

var ErrRevisionHistoryRecordNotFound = errors.New("revision history record not found")

type ListingKitRevisionHistoryDetail = sheinworkspace.HistoryDetail[ListingKitRevisionRecord, RevisionRestorePreviewPayload]

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

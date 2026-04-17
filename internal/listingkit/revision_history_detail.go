package listingkit

import "errors"

var ErrRevisionHistoryRecordNotFound = errors.New("revision history record not found")

type ListingKitRevisionHistoryDetail struct {
	TaskID         string                         `json:"task_id"`
	Record         *ListingKitRevisionRecord      `json:"record,omitempty"`
	Navigation     *RevisionHistoryNavigation     `json:"navigation,omitempty"`
	RestorePayload *RevisionRestorePreviewPayload `json:"restore_payload,omitempty"`
	HistoryIndex   int                            `json:"history_index"`
	TotalRecords   int                            `json:"total_records"`
	IsTruncated    bool                           `json:"is_truncated"`
	MaxRecords     int                            `json:"max_records"`
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
		restoreSafety := buildRevisionHistoryRestoreSafety(result, &recordWithID, restoreDraft, comparePreview)
		restoreContext := buildRevisionHistoryRestoreContext(&recordWithID, restoreRevisionPayload, comparePreview)
		restorePresentation := buildRevisionRestorePreviewPresentation(&recordWithID, restoreContext, restoreSafety, comparePreview)
		return &ListingKitRevisionHistoryDetail{
			TaskID: result.TaskID,
			Record: &recordWithID,
			Navigation: buildRevisionHistoryNavigation(
				buildAdjacentRevisionID(result.RevisionHistory, i-1),
				buildAdjacentRevisionID(result.RevisionHistory, i+1),
			),
			RestorePayload: buildRevisionHistoryDetailRestorePayload(
				&recordWithID,
				restoreDraft,
				restoreRevisionPayload,
				restoreContext,
				restoreSafety,
				restorePresentation,
				comparePreview,
			),
			HistoryIndex: i,
			TotalRecords: total,
			IsTruncated:  total > len(result.RevisionHistory),
			MaxRecords:   maxRevisionHistoryRecords,
		}, nil
	}

	return nil, ErrRevisionHistoryRecordNotFound
}

func buildAdjacentRevisionID(records []ListingKitRevisionRecord, index int) string {
	if index < 0 || index >= len(records) {
		return ""
	}
	return withRevisionHistoryRecordID(records[index], index).RevisionID
}

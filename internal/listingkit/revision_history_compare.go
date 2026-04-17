package listingkit

import "errors"

var ErrRevisionHistoryCompareTargetNotFound = errors.New("revision history compare target not found")

type RevisionHistoryComparePreview struct {
	CompareTo         string               `json:"compare_to,omitempty"`
	CompareRevisionID string               `json:"compare_revision_id,omitempty"`
	RelationLabel     string               `json:"relation_label,omitempty"`
	DiffPreview       *RevisionDiffPreview `json:"diff_preview,omitempty"`
}

func buildRevisionHistoryComparePreview(result *ListingKitResult, currentIndex int, compareTo string) (*RevisionHistoryComparePreview, error) {
	if result == nil || compareTo == "" {
		return nil, nil
	}

	currentRecord := withRevisionHistoryRecordID(result.RevisionHistory[currentIndex], currentIndex)
	if compareTo == "current" {
		diff := buildRevisionHistoryCurrentDiff(currentRecord, result.Shein)
		return &RevisionHistoryComparePreview{
			CompareTo:         compareTo,
			CompareRevisionID: "current",
			RelationLabel:     "当前版本",
			DiffPreview:       diff,
		}, nil
	}

	compareRecord, _, relationLabel, err := resolveRevisionHistoryCompareTarget(result.RevisionHistory, currentIndex, compareTo)
	if err != nil {
		return nil, err
	}
	diff := buildRevisionHistoryRecordDiff(compareRecord, currentRecord)

	return &RevisionHistoryComparePreview{
		CompareTo:         compareTo,
		CompareRevisionID: compareRecord.RevisionID,
		RelationLabel:     relationLabel,
		DiffPreview:       diff,
	}, nil
}

func resolveRevisionHistoryCompareTarget(records []ListingKitRevisionRecord, currentIndex int, compareTo string) (ListingKitRevisionRecord, int, string, error) {
	switch compareTo {
	case "prev":
		index := currentIndex - 1
		if index < 0 {
			return ListingKitRevisionRecord{}, -1, "", ErrRevisionHistoryCompareTargetNotFound
		}
		return withRevisionHistoryRecordID(records[index], index), index, "上一条", nil
	case "next":
		index := currentIndex + 1
		if index >= len(records) {
			return ListingKitRevisionRecord{}, -1, "", ErrRevisionHistoryCompareTargetNotFound
		}
		return withRevisionHistoryRecordID(records[index], index), index, "下一条", nil
	default:
		for i, record := range records {
			hydrated := withRevisionHistoryRecordID(record, i)
			if hydrated.RevisionID == compareTo {
				return hydrated, i, "指定记录", nil
			}
		}
		return ListingKitRevisionRecord{}, -1, "", ErrRevisionHistoryCompareTargetNotFound
	}
}

func buildRevisionHistoryRecordDiff(base, target ListingKitRevisionRecord) *RevisionDiffPreview {
	if base.Platform != "shein" || target.Platform != "shein" {
		return nil
	}
	baseDraft := buildRevisionHistoryRestoreDraft(&base)
	targetDraft := buildRevisionHistoryRestoreDraft(&target)
	return buildSheinRevisionDiffBetweenRevisions(baseDraft, targetDraft)
}

func buildRevisionHistoryCurrentDiff(record ListingKitRevisionRecord, pkg *SheinPackage) *RevisionDiffPreview {
	if record.Platform != "shein" || pkg == nil {
		return nil
	}
	baseDraft := buildRevisionHistoryRestoreDraft(&record)
	currentDraft := buildSheinEditorRevisionSkeleton(pkg)
	return buildSheinRevisionDiffBetweenRevisions(baseDraft, currentDraft)
}

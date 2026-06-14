package workspace

type HistoryCompareRecord struct {
	RevisionID string
	Draft      *EditorRevisionSkeleton
}

type HistoryComparePreview struct {
	CompareTo         string               `json:"compare_to,omitempty"`
	CompareRevisionID string               `json:"compare_revision_id,omitempty"`
	RelationLabel     string               `json:"relation_label,omitempty"`
	DiffPreview       *RevisionDiffPreview `json:"diff_preview,omitempty"`
}

func ResolveHistoryCompareTarget(records []HistoryCompareRecord, currentIndex int, compareTo string) (HistoryCompareRecord, int, string, bool) {
	switch compareTo {
	case "prev":
		index := currentIndex - 1
		if index < 0 {
			return HistoryCompareRecord{}, -1, "", false
		}
		return records[index], index, "上一条", true
	case "next":
		index := currentIndex + 1
		if index >= len(records) {
			return HistoryCompareRecord{}, -1, "", false
		}
		return records[index], index, "下一条", true
	default:
		for i, record := range records {
			if record.RevisionID == compareTo {
				return record, i, "指定记录", true
			}
		}
		return HistoryCompareRecord{}, -1, "", false
	}
}

func BuildHistoryComparePreview(current *HistoryCompareRecord, compareTo string, compare *HistoryCompareRecord, relationLabel string) *HistoryComparePreview {
	if current == nil || compare == nil || compareTo == "" {
		return nil
	}
	return &HistoryComparePreview{
		CompareTo:         compareTo,
		CompareRevisionID: compare.RevisionID,
		RelationLabel:     relationLabel,
		DiffPreview:       BuildRevisionDiffBetweenRevisions(compare.Draft, current.Draft),
	}
}

func BuildCurrentHistoryComparePreview(record *HistoryCompareRecord, currentDraft *EditorRevisionSkeleton) *HistoryComparePreview {
	if record == nil || currentDraft == nil {
		return nil
	}
	return &HistoryComparePreview{
		CompareTo:         "current",
		CompareRevisionID: "current",
		RelationLabel:     "当前版本",
		DiffPreview:       BuildRevisionDiffBetweenRevisions(record.Draft, currentDraft),
	}
}

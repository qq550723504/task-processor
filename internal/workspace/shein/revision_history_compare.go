package shein

import sheinmarketplace "task-processor/internal/marketplace/shein/workspace"

type HistoryCompareRecord = sheinmarketplace.HistoryCompareRecord
type HistoryComparePreview = sheinmarketplace.HistoryComparePreview

func ResolveHistoryCompareTarget(records []HistoryCompareRecord, currentIndex int, compareTo string) (HistoryCompareRecord, int, string, bool) {
	return sheinmarketplace.ResolveHistoryCompareTarget(records, currentIndex, compareTo)
}

func BuildHistoryComparePreview(current *HistoryCompareRecord, compareTo string, compare *HistoryCompareRecord, relationLabel string) *HistoryComparePreview {
	return sheinmarketplace.BuildHistoryComparePreview(current, compareTo, compare, relationLabel)
}

func BuildCurrentHistoryComparePreview(record *HistoryCompareRecord, currentDraft *EditorRevisionSkeleton) *HistoryComparePreview {
	return sheinmarketplace.BuildCurrentHistoryComparePreview(record, currentDraft)
}

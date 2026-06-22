package listingkit

import (
	"errors"

	sheinworkspace "task-processor/internal/marketplace/shein/workspace"
)

var ErrRevisionHistoryCompareTargetNotFound = errors.New("revision history compare target not found")

type RevisionHistoryComparePreview = sheinworkspace.HistoryComparePreview

func buildRevisionHistoryComparePreview(result *ListingKitResult, currentIndex int, compareTo string) (*RevisionHistoryComparePreview, error) {
	if result == nil || compareTo == "" {
		return nil, nil
	}

	currentRecord := buildHistoryCompareRecord(result.RevisionHistory[currentIndex], currentIndex)
	if compareTo == "current" {
		return sheinworkspace.BuildCurrentHistoryComparePreview(&currentRecord, buildCurrentHistoryCompareDraft(result.Shein)), nil
	}

	records := buildHistoryCompareRecords(result.RevisionHistory)
	compareRecord, _, relationLabel, ok := sheinworkspace.ResolveHistoryCompareTarget(records, currentIndex, compareTo)
	if !ok {
		return nil, ErrRevisionHistoryCompareTargetNotFound
	}
	return sheinworkspace.BuildHistoryComparePreview(&currentRecord, compareTo, &compareRecord, relationLabel), nil
}

func buildHistoryCompareRecords(records []ListingKitRevisionRecord) []sheinworkspace.HistoryCompareRecord {
	if len(records) == 0 {
		return nil
	}
	items := make([]sheinworkspace.HistoryCompareRecord, 0, len(records))
	for i, record := range records {
		items = append(items, buildHistoryCompareRecord(record, i))
	}
	return items
}

func buildHistoryCompareRecord(record ListingKitRevisionRecord, index int) sheinworkspace.HistoryCompareRecord {
	hydrated := withRevisionHistoryRecordID(record, index)
	return sheinworkspace.HistoryCompareRecord{
		RevisionID: hydrated.RevisionID,
		Draft:      buildRevisionHistoryRestoreDraft(&hydrated),
	}
}

func buildCurrentHistoryCompareDraft(pkg *SheinPackage) *SheinEditorRevisionSkeleton {
	if pkg == nil {
		return nil
	}
	return buildSheinEditorRevisionSkeleton(pkg)
}

package listingkit

import "time"

func buildRevisionHistoryPage(result *ListingKitResult, query *RevisionHistoryQuery) (*ListingKitRevisionHistoryPage, error) {
	if result == nil {
		return nil, ErrTaskResultUnavailable
	}

	normalized := normalizeRevisionHistoryQuery(query)
	before, err := parseRevisionHistoryBefore(normalized.Before)
	if err != nil {
		return nil, err
	}
	if err := validateRevisionHistoryActionType(normalized.ActionType); err != nil {
		return nil, err
	}

	items := make([]ListingKitRevisionRecord, 0, normalized.Limit)
	records := result.RevisionHistory
	hasMore := false
	for i := len(records) - 1; i >= 0; i-- {
		record := withRevisionHistoryRecordID(records[i], i)
		if normalized.ActionType != "" && record.ActionType != normalized.ActionType {
			continue
		}
		if before != nil && !record.UpdatedAt.Before(*before) {
			continue
		}
		if len(items) >= normalized.Limit {
			hasMore = true
			break
		}
		items = append(items, record)
	}

	meta := buildRevisionHistoryPageMeta(result, items, hasMore, normalized.ActionType)
	return &ListingKitRevisionHistoryPage{
		TaskID: result.TaskID,
		Items:  items,
		Meta:   meta,
	}, nil
}

func buildRevisionHistoryPageMeta(result *ListingKitResult, items []ListingKitRevisionRecord, hasMore bool, actionType string) *ListingKitRevisionHistoryPageMeta {
	if result == nil {
		return nil
	}
	total := result.RevisionHistoryTotal
	if total == 0 && len(result.RevisionHistory) > 0 {
		total = len(result.RevisionHistory)
	}
	meta := &ListingKitRevisionHistoryPageMeta{
		TotalRecords:    total,
		ReturnedRecords: len(items),
		HasMore:         hasMore,
		IsTruncated:     total > len(result.RevisionHistory),
		MaxRecords:      maxRevisionHistoryRecords,
		ActionType:      actionType,
		Counts:          buildRevisionHistoryCounts(result.RevisionHistory),
	}
	if hasMore && len(items) > 0 {
		meta.NextBefore = items[len(items)-1].UpdatedAt.Format(time.RFC3339)
	}
	return meta
}

func buildRevisionHistoryCounts(records []ListingKitRevisionRecord) *ListingKitRevisionHistoryCounts {
	if len(records) == 0 {
		return &ListingKitRevisionHistoryCounts{}
	}
	counts := &ListingKitRevisionHistoryCounts{}
	for i, record := range records {
		hydrated := withRevisionHistoryRecordID(record, i)
		counts.All++
		switch hydrated.ActionType {
		case RevisionActionTypeRestore:
			counts.Restore++
		default:
			counts.Edit++
		}
	}
	return counts
}

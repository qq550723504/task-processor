package listingkit

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const defaultRevisionHistoryPageSize = 10

var ErrInvalidRevisionHistoryCursor = errors.New("invalid revision history cursor")
var ErrInvalidRevisionHistoryActionType = errors.New("invalid revision history action type")

type RevisionHistoryQuery struct {
	Limit      int    `form:"limit"`
	Before     string `form:"before"`
	ActionType string `form:"action_type"`
}

type ListingKitRevisionHistoryPage struct {
	TaskID string                             `json:"task_id"`
	Items  []ListingKitRevisionRecord         `json:"items,omitempty"`
	Meta   *ListingKitRevisionHistoryPageMeta `json:"meta,omitempty"`
}

type ListingKitRevisionHistoryPageMeta struct {
	TotalRecords    int                              `json:"total_records"`
	ReturnedRecords int                              `json:"returned_records"`
	HasMore         bool                             `json:"has_more"`
	IsTruncated     bool                             `json:"is_truncated"`
	MaxRecords      int                              `json:"max_records"`
	NextBefore      string                           `json:"next_before,omitempty"`
	ActionType      string                           `json:"action_type,omitempty"`
	Counts          *ListingKitRevisionHistoryCounts `json:"counts,omitempty"`
}

type ListingKitRevisionHistoryCounts struct {
	All     int `json:"all"`
	Edit    int `json:"edit"`
	Restore int `json:"restore"`
}

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
	records := buildRevisionHistoryRecords(result.RevisionHistory)
	hasMore := false
	for i := len(records) - 1; i >= 0; i-- {
		record := records[i]
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
	for _, hydrated := range buildRevisionHistoryRecords(records) {
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

func normalizeRevisionHistoryQuery(query *RevisionHistoryQuery) RevisionHistoryQuery {
	if query == nil {
		return RevisionHistoryQuery{Limit: defaultRevisionHistoryPageSize}
	}
	normalized := *query
	if normalized.Limit <= 0 {
		normalized.Limit = defaultRevisionHistoryPageSize
	}
	if normalized.Limit > maxRevisionHistoryRecords {
		normalized.Limit = maxRevisionHistoryRecords
	}
	normalized.Before = strings.TrimSpace(normalized.Before)
	normalized.ActionType = strings.ToLower(strings.TrimSpace(normalized.ActionType))
	return normalized
}

func parseRevisionHistoryBefore(before string) (*time.Time, error) {
	before = strings.TrimSpace(before)
	if before == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, before)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRevisionHistoryCursor, err)
	}
	return &parsed, nil
}

func validateRevisionHistoryActionType(actionType string) error {
	switch strings.TrimSpace(actionType) {
	case "", RevisionActionTypeEdit, RevisionActionTypeRestore:
		return nil
	default:
		return fmt.Errorf("%w: %s", ErrInvalidRevisionHistoryActionType, actionType)
	}
}

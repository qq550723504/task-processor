package listingkit

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestGetTaskRevisionHistoryReturnsNewestFirstPage(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	repo := &stubApplyRevisionRepo{
		task: &Task{
			ID: "task-history-1",
			Result: &ListingKitResult{
				TaskID:               "task-history-1",
				RevisionHistoryTotal: 4,
				RevisionHistory: []ListingKitRevisionRecord{
					{Platform: "shein", UpdatedAt: now.Add(-4 * time.Minute)},
					{Platform: "shein", UpdatedAt: now.Add(-3 * time.Minute)},
					{Platform: "shein", UpdatedAt: now.Add(-2 * time.Minute)},
					{Platform: "shein", UpdatedAt: now.Add(-1 * time.Minute)},
				},
			},
		},
	}
	svc := &service{repo: repo}

	page, err := svc.GetTaskRevisionHistory(context.Background(), "task-history-1", &RevisionHistoryQuery{Limit: 2})
	if err != nil {
		t.Fatalf("get revision history: %v", err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(page.Items))
	}
	if !page.Items[0].UpdatedAt.After(page.Items[1].UpdatedAt) {
		t.Fatalf("items not newest first: %+v", page.Items)
	}
	if page.Meta == nil || !page.Meta.HasMore || page.Meta.NextBefore == "" {
		t.Fatalf("meta = %+v", page.Meta)
	}
}

func TestGetTaskRevisionHistoryFiltersBeforeCursor(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	repo := &stubApplyRevisionRepo{
		task: &Task{
			ID: "task-history-2",
			Result: &ListingKitResult{
				TaskID: "task-history-2",
				RevisionHistory: []ListingKitRevisionRecord{
					{Platform: "shein", UpdatedAt: now.Add(-3 * time.Minute)},
					{Platform: "shein", UpdatedAt: now.Add(-2 * time.Minute)},
					{Platform: "shein", UpdatedAt: now.Add(-1 * time.Minute)},
				},
				RevisionHistoryTotal: 3,
			},
		},
	}
	svc := &service{repo: repo}

	page, err := svc.GetTaskRevisionHistory(context.Background(), "task-history-2", &RevisionHistoryQuery{
		Limit:  2,
		Before: now.Add(-90 * time.Second).Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("get revision history: %v", err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(page.Items))
	}
	if !page.Items[0].UpdatedAt.Before(now.Add(-90 * time.Second)) {
		t.Fatalf("unexpected first item: %+v", page.Items[0])
	}
}

func TestGetTaskRevisionHistoryRejectsInvalidCursor(t *testing.T) {
	t.Parallel()

	repo := &stubApplyRevisionRepo{
		task: &Task{
			ID:     "task-history-3",
			Result: &ListingKitResult{TaskID: "task-history-3"},
		},
	}
	svc := &service{repo: repo}

	_, err := svc.GetTaskRevisionHistory(context.Background(), "task-history-3", &RevisionHistoryQuery{Before: "bad"})
	if err == nil {
		t.Fatal("expected invalid cursor error")
	}
	if err != nil && !errors.Is(err, ErrInvalidRevisionHistoryCursor) {
		t.Fatalf("error = %v, want invalid cursor", err)
	}
}

func TestGetTaskRevisionHistoryFiltersActionType(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	repo := &stubApplyRevisionRepo{
		task: &Task{
			ID: "task-history-4",
			Result: &ListingKitResult{
				TaskID: "task-history-4",
				RevisionHistory: []ListingKitRevisionRecord{
					{Platform: "shein", ActionType: RevisionActionTypeEdit, UpdatedAt: now.Add(-3 * time.Minute)},
					{Platform: "shein", ActionType: RevisionActionTypeRestore, RestoredFromRevisionID: "rev-1", UpdatedAt: now.Add(-2 * time.Minute)},
					{Platform: "shein", ActionType: RevisionActionTypeEdit, UpdatedAt: now.Add(-1 * time.Minute)},
				},
				RevisionHistoryTotal: 3,
			},
		},
	}
	svc := &service{repo: repo}

	page, err := svc.GetTaskRevisionHistory(context.Background(), "task-history-4", &RevisionHistoryQuery{
		Limit:      10,
		ActionType: RevisionActionTypeRestore,
	})
	if err != nil {
		t.Fatalf("get revision history: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("items = %+v, want single restore item", page.Items)
	}
	if page.Items[0].ActionType != RevisionActionTypeRestore {
		t.Fatalf("item = %+v", page.Items[0])
	}
	if page.Meta == nil || page.Meta.ActionType != RevisionActionTypeRestore {
		t.Fatalf("meta = %+v", page.Meta)
	}
	if page.Meta.Counts == nil || page.Meta.Counts.All != 3 || page.Meta.Counts.Edit != 2 || page.Meta.Counts.Restore != 1 {
		t.Fatalf("counts = %+v", page.Meta)
	}
}

func TestGetTaskRevisionHistoryRejectsInvalidActionType(t *testing.T) {
	t.Parallel()

	repo := &stubApplyRevisionRepo{
		task: &Task{
			ID:     "task-history-5",
			Result: &ListingKitResult{TaskID: "task-history-5"},
		},
	}
	svc := &service{repo: repo}

	_, err := svc.GetTaskRevisionHistory(context.Background(), "task-history-5", &RevisionHistoryQuery{ActionType: "archive"})
	if err == nil {
		t.Fatal("expected invalid action_type error")
	}
	if !errors.Is(err, ErrInvalidRevisionHistoryActionType) {
		t.Fatalf("error = %v, want %v", err, ErrInvalidRevisionHistoryActionType)
	}
}

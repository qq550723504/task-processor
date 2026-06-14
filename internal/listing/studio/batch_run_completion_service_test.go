package studio

import (
	"context"
	"errors"
	"testing"
	"time"
)

type completionItemStub struct {
	ID         string
	Status     string
	FinishedAt *time.Time
	UpdatedAt  time.Time
}

func TestBatchRunCompletionServiceCancelUnfinishedItems(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 14, 10, 0, 0, 0, time.UTC)
	var updated []completionItemStub
	service := NewBatchRunCompletionService(BatchRunCompletionServiceConfig[completionItemStub, string]{
		UpdateItem: func(_ context.Context, item *completionItemStub) error {
			updated = append(updated, *item)
			return nil
		},
		ItemStatus: func(item *completionItemStub) string {
			if item == nil {
				return ""
			}
			return item.Status
		},
		MarkCancelled: func(item *completionItemStub, at time.Time) {
			item.Status = "cancelled"
			item.FinishedAt = &at
			item.UpdatedAt = at
		},
		Now:                      func() time.Time { return now },
		SucceededStatus:          "succeeded",
		FailedStatus:             "failed",
		CancelledStatus:          "cancelled",
		PartiallySucceededStatus: "partially_succeeded",
	})

	items := []completionItemStub{
		{ID: "pending", Status: "pending"},
		{ID: "succeeded", Status: "succeeded"},
		{ID: "failed", Status: "failed"},
	}
	if err := service.CancelUnfinishedItems(context.Background(), items); err != nil {
		t.Fatalf("CancelUnfinishedItems() error = %v", err)
	}
	if len(updated) != 1 {
		t.Fatalf("len(updated) = %d, want 1", len(updated))
	}
	if updated[0].ID != "pending" || updated[0].Status != "cancelled" || !updated[0].UpdatedAt.Equal(now) {
		t.Fatalf("updated[0] = %+v, want cancelled pending item", updated[0])
	}
}

func TestBatchRunCompletionServiceCancelUnfinishedItemsPropagatesUpdateError(t *testing.T) {
	t.Parallel()

	updateErr := errors.New("update failed")
	service := NewBatchRunCompletionService(BatchRunCompletionServiceConfig[completionItemStub, string]{
		UpdateItem: func(context.Context, *completionItemStub) error { return updateErr },
		ItemStatus: func(item *completionItemStub) string {
			return item.Status
		},
		MarkCancelled:            func(item *completionItemStub, _ time.Time) { item.Status = "cancelled" },
		SucceededStatus:          "succeeded",
		FailedStatus:             "failed",
		CancelledStatus:          "cancelled",
		PartiallySucceededStatus: "partially_succeeded",
	})

	err := service.CancelUnfinishedItems(context.Background(), []completionItemStub{{ID: "pending", Status: "pending"}})
	if !errors.Is(err, updateErr) {
		t.Fatalf("CancelUnfinishedItems() error = %v, want %v", err, updateErr)
	}
}

func TestBatchRunCompletionServiceResolveFinalStatus(t *testing.T) {
	t.Parallel()

	service := NewBatchRunCompletionService(BatchRunCompletionServiceConfig[completionItemStub, string]{
		ItemStatus: func(item *completionItemStub) string {
			if item == nil {
				return ""
			}
			return item.Status
		},
		SucceededStatus:          "succeeded",
		FailedStatus:             "failed",
		CancelledStatus:          "cancelled",
		PartiallySucceededStatus: "partially_succeeded",
	})

	tests := []struct {
		name            string
		cancelRequested bool
		items           []completionItemStub
		want            string
	}{
		{name: "cancelled", cancelRequested: true, want: "cancelled"},
		{name: "all succeeded", items: []completionItemStub{{Status: "succeeded"}}, want: "succeeded"},
		{name: "all failed", items: []completionItemStub{{Status: "failed"}}, want: "failed"},
		{name: "mixed", items: []completionItemStub{{Status: "succeeded"}, {Status: "failed"}}, want: "partially_succeeded"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := service.ResolveFinalStatus(tt.cancelRequested, tt.items); got != tt.want {
				t.Fatalf("ResolveFinalStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBatchRunCompletionServiceCountItems(t *testing.T) {
	t.Parallel()

	service := NewBatchRunCompletionService(BatchRunCompletionServiceConfig[completionItemStub, string]{
		ItemStatus: func(item *completionItemStub) string {
			if item == nil {
				return ""
			}
			return item.Status
		},
		SucceededStatus:          "succeeded",
		FailedStatus:             "failed",
		CancelledStatus:          "cancelled",
		PartiallySucceededStatus: "partially_succeeded",
	})

	got := service.CountItems([]completionItemStub{
		{Status: "succeeded"},
		{Status: "failed"},
		{Status: "cancelled"},
		{Status: "pending"},
	})
	want := BatchRunItemCounters{Total: 4, Completed: 3, Succeeded: 1, Failed: 2}
	if got != want {
		t.Fatalf("CountItems() = %+v, want %+v", got, want)
	}
}

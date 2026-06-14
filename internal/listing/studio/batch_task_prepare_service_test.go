package studio

import (
	"context"
	"testing"
	"time"
)

func TestBatchTaskPrepareServicePrepareTaskCreation(t *testing.T) {
	t.Parallel()

	type session struct {
		pending []string
		failed  []string
		status  string
		updated time.Time
	}
	type batch struct {
		status  string
		updated time.Time
	}
	type result struct {
		id string
	}

	now := time.Date(2026, 6, 15, 1, 0, 0, 0, time.UTC)
	var updatedSession *session
	var updatedBatch *batch
	service := NewBatchTaskPrepareService(BatchTaskPrepareServiceConfig[session, batch, result]{
		UpdateSession: func(_ context.Context, got *session) error {
			updatedSession = got
			return nil
		},
		SetPendingDesignIDs: func(s *session, designIDs []string) {
			s.pending = append([]string(nil), designIDs...)
		},
		ClearFailedTasks: func(s *session) {
			s.failed = nil
		},
		SetSessionCreating: func(s *session) {
			s.status = "tasks_creating"
		},
		SetSessionUpdatedAt: func(s *session, updatedAt time.Time) {
			s.updated = updatedAt
		},
		UpdateBatch: func(_ context.Context, got *batch) error {
			updatedBatch = got
			return nil
		},
		SetBatchCreating: func(b *batch) {
			b.status = "tasks_creating"
		},
		SetBatchUpdatedAt: func(b *batch, updatedAt time.Time) {
			b.updated = updatedAt
		},
		LoadResult: func(context.Context, string) (*result, error) {
			return &result{id: "batch-1"}, nil
		},
		CurrentTime: func() time.Time { return now },
	})

	got, err := service.PrepareTaskCreation(context.Background(), "batch-1", BatchTaskPrepareState[session, batch]{
		Session:   &session{failed: []string{"old"}},
		Batch:     &batch{},
		DesignIDs: []string{"design-1"},
	})
	if err != nil {
		t.Fatalf("PrepareTaskCreation() error = %v", err)
	}
	if got == nil || got.id != "batch-1" {
		t.Fatalf("PrepareTaskCreation() = %+v, want batch-1", got)
	}
	if updatedSession == nil || updatedSession.status != "tasks_creating" || len(updatedSession.pending) != 1 || updatedSession.pending[0] != "design-1" || !updatedSession.updated.Equal(now) {
		t.Fatalf("updatedSession = %+v, want tasks_creating with pending design", updatedSession)
	}
	if updatedBatch == nil || updatedBatch.status != "tasks_creating" || !updatedBatch.updated.Equal(now) {
		t.Fatalf("updatedBatch = %+v, want tasks_creating batch", updatedBatch)
	}
}

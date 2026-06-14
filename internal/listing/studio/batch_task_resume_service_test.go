package studio

import (
	"context"
	"testing"
	"time"
)

func TestBatchTaskResumeFinalizeServiceFinalizeTaskCreation(t *testing.T) {
	t.Parallel()

	type session struct {
		pending    []string
		created    []string
		failed     []string
		status     string
		updatedAt  time.Time
		createdIDs []string
	}
	type batch struct {
		status    string
		updatedAt time.Time
	}
	type result struct {
		id string
	}

	now := time.Date(2026, 6, 15, 2, 0, 0, 0, time.UTC)
	var updatedSession *session
	var updatedBatch *batch
	service := NewBatchTaskResumeFinalizeService(BatchTaskResumeFinalizeServiceConfig[
		session,
		batch,
		result,
		string,
		string,
	]{
		UpdateSession: func(_ context.Context, got *session) error {
			updatedSession = got
			return nil
		},
		ClearPendingTasks: func(s *session) {
			s.pending = nil
		},
		SetCreatedTasks: func(s *session, created []string) {
			s.created = append([]string(nil), created...)
			s.createdIDs = append([]string(nil), created...)
		},
		SetFailedTasks: func(s *session, failed []string) {
			s.failed = append([]string(nil), failed...)
		},
		SetSessionDone: func(s *session) {
			s.status = "tasks_created"
		},
		SetSessionUpdated: func(s *session, updatedAt time.Time) {
			s.updatedAt = updatedAt
		},
		UpdateBatch: func(_ context.Context, got *batch) error {
			updatedBatch = got
			return nil
		},
		SetBatchDone: func(b *batch) {
			b.status = "tasks_created"
		},
		SetBatchUpdated: func(b *batch, updatedAt time.Time) {
			b.updatedAt = updatedAt
		},
		LoadResult: func(context.Context, string) (*result, error) {
			return &result{id: "batch-1"}, nil
		},
		CurrentTime: func() time.Time { return now },
	})

	got, err := service.FinalizeTaskCreation(context.Background(), "batch-1", BatchTaskResumeFinalizeState[
		session,
		batch,
		string,
		string,
	]{
		Session:      &session{pending: []string{"design-1"}},
		Batch:        &batch{},
		CreatedTasks: []string{"task-1"},
		FailedTasks:  []string{"design-2"},
	})
	if err != nil {
		t.Fatalf("FinalizeTaskCreation() error = %v", err)
	}
	if got == nil || got.id != "batch-1" {
		t.Fatalf("FinalizeTaskCreation() = %+v, want batch-1", got)
	}
	if updatedSession == nil || updatedSession.status != "tasks_created" || len(updatedSession.pending) != 0 || len(updatedSession.created) != 1 || updatedSession.created[0] != "task-1" || len(updatedSession.failed) != 1 || updatedSession.failed[0] != "design-2" || !updatedSession.updatedAt.Equal(now) {
		t.Fatalf("updatedSession = %+v, want finalized session", updatedSession)
	}
	if updatedBatch == nil || updatedBatch.status != "tasks_created" || !updatedBatch.updatedAt.Equal(now) {
		t.Fatalf("updatedBatch = %+v, want finalized batch", updatedBatch)
	}
}

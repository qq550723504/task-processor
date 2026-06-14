package studio

import (
	"context"
	"errors"
	"testing"
)

type draftSessionStub struct {
	ID           string
	SavedAsBatch bool
	Name         string
}

type draftRepoStub struct {
	sessions       map[string]*draftSessionStub
	designs        map[string][]string
	gallery        []string
	deletedSession string
}

func (r *draftRepoStub) GetSession(_ context.Context, sessionID string) (*draftSessionStub, error) {
	if r.sessions == nil {
		return nil, nil
	}
	session := r.sessions[sessionID]
	if session == nil {
		return nil, nil
	}
	cloned := *session
	return &cloned, nil
}

func (r *draftRepoStub) DeleteSession(_ context.Context, sessionID string) error {
	r.deletedSession = sessionID
	return nil
}

func (r *draftRepoStub) ListSessionDesigns(_ context.Context, sessionID string) ([]string, error) {
	return append([]string(nil), r.designs[sessionID]...), nil
}

func (r *draftRepoStub) CountSessionDesignsBySessionIDs(_ context.Context, sessionIDs []string) (map[string]int, error) {
	counts := make(map[string]int, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		counts[sessionID] = len(r.designs[sessionID])
	}
	return counts, nil
}

func (r *draftRepoStub) ListGalleryItems(context.Context, int) ([]string, error) {
	return append([]string(nil), r.gallery...), nil
}

func (r *draftRepoStub) ListBatchSessions(context.Context, int) ([]draftSessionStub, error) {
	items := make([]draftSessionStub, 0, len(r.sessions))
	for _, session := range r.sessions {
		items = append(items, *session)
	}
	return items, nil
}

func newDraftServiceForTest(repo *draftRepoStub) *BatchDraftService[draftSessionStub, string, string, string] {
	return NewBatchDraftService(BatchDraftServiceConfig[draftSessionStub, string, string, string]{
		Repo:         repo,
		IsSavedBatch: func(session *draftSessionStub) bool { return session != nil && session.SavedAsBatch },
		SessionID:    func(session *draftSessionStub) string { return session.ID },
		MapBatchListItem: func(session *draftSessionStub, count int) string {
			return session.Name
		},
	})
}

func TestBatchDraftServiceListSessionGallery(t *testing.T) {
	service := newDraftServiceForTest(&draftRepoStub{gallery: []string{"a", "b"}})

	result, err := service.ListSessionGallery(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListSessionGallery() error = %v", err)
	}
	if result.Total != 2 || len(result.Items) != 2 {
		t.Fatalf("result = %+v, want 2 items", result)
	}
}

func TestBatchDraftServiceListBatches(t *testing.T) {
	service := newDraftServiceForTest(&draftRepoStub{
		sessions: map[string]*draftSessionStub{
			"batch-1": {ID: "batch-1", SavedAsBatch: true, Name: "Batch 1"},
		},
		designs: map[string][]string{
			"batch-1": {"design-1"},
		},
	})

	result, err := service.ListBatches(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListBatches() error = %v", err)
	}
	if result.Total != 1 || len(result.Items) != 1 || result.Items[0] != "Batch 1" {
		t.Fatalf("result = %+v, want one mapped batch", result)
	}
}

func TestBatchDraftServiceGetBatch(t *testing.T) {
	service := newDraftServiceForTest(&draftRepoStub{
		sessions: map[string]*draftSessionStub{
			"batch-1": {ID: "batch-1", SavedAsBatch: true, Name: "Batch 1"},
		},
		designs: map[string][]string{
			"batch-1": {"design-1"},
		},
	})

	result, err := service.GetBatch(context.Background(), "batch-1")
	if err != nil {
		t.Fatalf("GetBatch() error = %v", err)
	}
	if result.Batch == nil || result.Batch.ID != "batch-1" || len(result.Designs) != 1 {
		t.Fatalf("result = %+v, want batch-1 with one design", result)
	}
}

func TestBatchDraftServiceGetBatchReturnsNotFoundForUnsavedSession(t *testing.T) {
	service := newDraftServiceForTest(&draftRepoStub{
		sessions: map[string]*draftSessionStub{
			"batch-1": {ID: "batch-1", SavedAsBatch: false, Name: "Batch 1"},
		},
	})

	_, err := service.GetBatch(context.Background(), "batch-1")
	if !errors.Is(err, ErrBatchDraftNotFound) {
		t.Fatalf("GetBatch() error = %v, want ErrBatchDraftNotFound", err)
	}
}

func TestBatchDraftServiceDeleteBatchIsIdempotent(t *testing.T) {
	repo := &draftRepoStub{
		sessions: map[string]*draftSessionStub{
			"batch-1": {ID: "batch-1", SavedAsBatch: true, Name: "Batch 1"},
		},
	}
	service := newDraftServiceForTest(repo)

	if err := service.DeleteBatch(context.Background(), "batch-1"); err != nil {
		t.Fatalf("DeleteBatch() error = %v", err)
	}
	if repo.deletedSession != "batch-1" {
		t.Fatalf("deletedSession = %q, want batch-1", repo.deletedSession)
	}
}

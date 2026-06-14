package studio

import (
	"context"
	"errors"
	"testing"
)

type stubBatchRunRepo struct {
	run       *BatchRunRecord
	items     []BatchRunItemRecord
	updateRun *BatchRunRecord
	createErr error
	getErr    error
	listErr   error
	updateErr error
}

func (r *stubBatchRunRepo) CreateBatchRun(_ context.Context, run *BatchRunRecord, items []BatchRunItemRecord) error {
	if r.createErr != nil {
		return r.createErr
	}
	clonedRun := *run
	r.run = &clonedRun
	r.items = append([]BatchRunItemRecord(nil), items...)
	return nil
}

func (r *stubBatchRunRepo) GetBatchRun(context.Context, string) (*BatchRunRecord, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	if r.run == nil {
		return nil, nil
	}
	cloned := *r.run
	return &cloned, nil
}

func (r *stubBatchRunRepo) ListBatchRunItems(context.Context, string) ([]BatchRunItemRecord, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	return append([]BatchRunItemRecord(nil), r.items...), nil
}

func (r *stubBatchRunRepo) UpdateBatchRun(_ context.Context, run *BatchRunRecord) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	cloned := *run
	r.updateRun = &cloned
	r.run = &cloned
	return nil
}

type stubSessionRepo struct {
	sessions map[string]*BatchSeedSession
	err      error
}

func (r *stubSessionRepo) GetSession(_ context.Context, batchID string) (*BatchSeedSession, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.sessions[batchID], nil
}

func TestBatchRunServiceCreateBatchRun(t *testing.T) {
	t.Parallel()

	repo := &stubBatchRunRepo{}
	sessionRepo := &stubSessionRepo{sessions: map[string]*BatchSeedSession{
		"batch-1": {SavedAsBatch: true},
		"batch-2": {SavedAsBatch: true},
	}}
	var startedRunID string
	service := NewBatchRunService(BatchRunServiceConfig{
		Repo:        repo,
		SessionRepo: sessionRepo,
		StartRun: func(_ context.Context, runID string) error {
			startedRunID = runID
			return nil
		},
		NewRunID:      func() string { return "run-1" },
		RequestUserID: func(context.Context) string { return "user-1" },
	})

	run, items, err := service.CreateBatchRun(context.Background(), &CreateBatchRunRequest{
		BatchIDs: []string{" batch-1 ", "batch-2"},
	})
	if err != nil {
		t.Fatalf("CreateBatchRun() error = %v", err)
	}
	if run == nil || run.ID != "run-1" || run.UserID != "user-1" || run.TotalBatches != 2 {
		t.Fatalf("run = %+v", run)
	}
	if startedRunID != "run-1" {
		t.Fatalf("startedRunID = %q, want run-1", startedRunID)
	}
	if len(items) != 2 || items[0].BatchID != "batch-1" || items[1].BatchID != "batch-2" {
		t.Fatalf("items = %+v", items)
	}
}

func TestBatchRunServiceCreateRejectsDuplicateBatchIDs(t *testing.T) {
	t.Parallel()

	service := NewBatchRunService(BatchRunServiceConfig{
		Repo:        &stubBatchRunRepo{},
		SessionRepo: &stubSessionRepo{sessions: map[string]*BatchSeedSession{"batch-1": {SavedAsBatch: true}}},
		StartRun:    func(context.Context, string) error { return nil },
		NewRunID:    func() string { return "run-1" },
	})

	if _, _, err := service.CreateBatchRun(context.Background(), &CreateBatchRunRequest{BatchIDs: []string{"batch-1", "batch-1"}}); err == nil {
		t.Fatal("CreateBatchRun() error = nil, want duplicate batch_id error")
	}
}

func TestBatchRunServiceCreateReturnsMissingBatchError(t *testing.T) {
	t.Parallel()

	service := NewBatchRunService(BatchRunServiceConfig{
		Repo:        &stubBatchRunRepo{},
		SessionRepo: &stubSessionRepo{sessions: map[string]*BatchSeedSession{"batch-1": nil}},
		StartRun:    func(context.Context, string) error { return nil },
		NewRunID:    func() string { return "run-1" },
	})

	if _, _, err := service.CreateBatchRun(context.Background(), &CreateBatchRunRequest{BatchIDs: []string{"batch-1"}}); !errors.Is(err, ErrBatchSessionNotFound) {
		t.Fatalf("CreateBatchRun() error = %v, want ErrBatchSessionNotFound", err)
	}
}

func TestBatchRunServiceCancelBatchRun(t *testing.T) {
	t.Parallel()

	repo := &stubBatchRunRepo{run: &BatchRunRecord{ID: "run-1"}}
	service := NewBatchRunService(BatchRunServiceConfig{Repo: repo})

	if err := service.CancelBatchRun(context.Background(), " run-1 "); err != nil {
		t.Fatalf("CancelBatchRun() error = %v", err)
	}
	if repo.updateRun == nil || !repo.updateRun.CancelRequested {
		t.Fatalf("updateRun = %+v, want cancel requested", repo.updateRun)
	}
}

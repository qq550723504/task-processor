package listingkit

import (
	"context"
	"errors"
	"testing"
	"time"

	studiodomain "task-processor/internal/listing/studio"
)

func TestStudioBatchRunServiceCreateReturnsOrderedItemsFromSavedBatchIDs(t *testing.T) {
	repo := NewMemStudioBatchRunRepository()
	sessionRepo := newStudioBatchRunTestSessionRepo()
	svc := newTaskStudioBatchRunService(taskStudioBatchRunServiceConfig{
		repo:              repo,
		studioSessionRepo: sessionRepo,
		startRun: func(context.Context, string) error {
			return nil
		},
	})
	ctx := WithTenantID(context.Background(), "tenant-a")

	seedStudioBatchRunSavedBatch(t, sessionRepo, ctx, "batch-1")
	seedStudioBatchRunSavedBatch(t, sessionRepo, ctx, "batch-2")

	run, items, err := svc.CreateStudioBatchRun(ctx, &CreateStudioBatchRunRequest{
		BatchIDs: []string{"batch-2", "batch-1"},
	})
	if err != nil {
		t.Fatalf("CreateStudioBatchRun() error = %v", err)
	}
	if run == nil {
		t.Fatal("CreateStudioBatchRun() run = nil")
	}
	if run.TotalBatches != 2 {
		t.Fatalf("run.TotalBatches = %d, want 2", run.TotalBatches)
	}
	if run.Mode != StudioBatchRunModeGenerate {
		t.Fatalf("run.Mode = %q, want %q", run.Mode, StudioBatchRunModeGenerate)
	}
	if run.FailurePolicy != StudioBatchRunFailurePolicyContinueOnError {
		t.Fatalf("run.FailurePolicy = %q, want %q", run.FailurePolicy, StudioBatchRunFailurePolicyContinueOnError)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0].BatchID != "batch-2" || items[0].Position != 1 {
		t.Fatalf("items[0] = %+v, want batch-2 at position 1", items[0])
	}
	if items[1].BatchID != "batch-1" || items[1].Position != 2 {
		t.Fatalf("items[1] = %+v, want batch-1 at position 2", items[1])
	}
}

func TestStudioBatchRunServiceCreateUsesRequestedMode(t *testing.T) {
	repo := NewMemStudioBatchRunRepository()
	sessionRepo := newStudioBatchRunTestSessionRepo()
	svc := newTaskStudioBatchRunService(taskStudioBatchRunServiceConfig{
		repo:              repo,
		studioSessionRepo: sessionRepo,
		startRun: func(context.Context, string) error {
			return nil
		},
	})
	ctx := WithTenantID(context.Background(), "tenant-a")

	seedStudioBatchRunSavedBatch(t, sessionRepo, ctx, "batch-1")

	run, _, err := svc.CreateStudioBatchRun(ctx, &CreateStudioBatchRunRequest{
		BatchIDs: []string{"batch-1"},
		Mode:     string(StudioBatchRunModeCreateTasks),
	})
	if err != nil {
		t.Fatalf("CreateStudioBatchRun() error = %v", err)
	}
	if run.Mode != StudioBatchRunModeCreateTasks {
		t.Fatalf("run.Mode = %q, want %q", run.Mode, StudioBatchRunModeCreateTasks)
	}
}

func TestStudioBatchRunServiceCreateRejectsUnsupportedMode(t *testing.T) {
	repo := NewMemStudioBatchRunRepository()
	sessionRepo := newStudioBatchRunTestSessionRepo()
	svc := newTaskStudioBatchRunService(taskStudioBatchRunServiceConfig{
		repo:              repo,
		studioSessionRepo: sessionRepo,
		startRun: func(context.Context, string) error {
			return nil
		},
	})
	ctx := WithTenantID(context.Background(), "tenant-a")

	seedStudioBatchRunSavedBatch(t, sessionRepo, ctx, "batch-1")

	if _, _, err := svc.CreateStudioBatchRun(ctx, &CreateStudioBatchRunRequest{
		BatchIDs: []string{"batch-1"},
		Mode:     "invalid-mode",
	}); err == nil {
		t.Fatal("CreateStudioBatchRun() error = nil, want invalid mode error")
	}
}

func TestStudioBatchRunServiceCreateRejectsDuplicateBatchIDs(t *testing.T) {
	repo := NewMemStudioBatchRunRepository()
	sessionRepo := newStudioBatchRunTestSessionRepo()
	svc := newTaskStudioBatchRunService(taskStudioBatchRunServiceConfig{
		repo:              repo,
		studioSessionRepo: sessionRepo,
		startRun: func(context.Context, string) error {
			return nil
		},
	})
	ctx := WithTenantID(context.Background(), "tenant-a")

	seedStudioBatchRunSavedBatch(t, sessionRepo, ctx, "batch-1")

	if _, _, err := svc.CreateStudioBatchRun(ctx, &CreateStudioBatchRunRequest{
		BatchIDs: []string{"batch-1", "batch-1"},
	}); err == nil {
		t.Fatal("CreateStudioBatchRun() error = nil, want duplicate batch id error")
	}
}

func TestStudioBatchRunServiceCreateRequiresConfiguredStarter(t *testing.T) {
	repo := NewMemStudioBatchRunRepository()
	sessionRepo := newStudioBatchRunTestSessionRepo()
	svc := newTaskStudioBatchRunService(taskStudioBatchRunServiceConfig{
		repo:              repo,
		studioSessionRepo: sessionRepo,
	})
	ctx := WithTenantID(context.Background(), "tenant-a")

	seedStudioBatchRunSavedBatch(t, sessionRepo, ctx, "batch-1")

	if _, _, err := svc.CreateStudioBatchRun(ctx, &CreateStudioBatchRunRequest{
		BatchIDs: []string{"batch-1"},
	}); err == nil {
		t.Fatal("CreateStudioBatchRun() error = nil, want starter configuration error")
	}
}

func TestStudioBatchRunServiceCreateStartsConfiguredRun(t *testing.T) {
	repo := NewMemStudioBatchRunRepository()
	sessionRepo := newStudioBatchRunTestSessionRepo()
	var startedRunID string
	startCalls := 0
	svc := newTaskStudioBatchRunService(taskStudioBatchRunServiceConfig{
		repo:              repo,
		studioSessionRepo: sessionRepo,
		startRun: func(_ context.Context, runID string) error {
			startCalls++
			startedRunID = runID
			return nil
		},
	})
	ctx := WithTenantID(context.Background(), "tenant-a")

	seedStudioBatchRunSavedBatch(t, sessionRepo, ctx, "batch-1")

	run, _, err := svc.CreateStudioBatchRun(ctx, &CreateStudioBatchRunRequest{
		BatchIDs: []string{"batch-1"},
	})
	if err != nil {
		t.Fatalf("CreateStudioBatchRun() error = %v", err)
	}
	if startCalls != 1 {
		t.Fatalf("startCalls = %d, want 1", startCalls)
	}
	if startedRunID != run.ID {
		t.Fatalf("startedRunID = %q, want %q", startedRunID, run.ID)
	}
}

func TestTaskStudioBatchRunServiceUsesListingStudioRunner(t *testing.T) {
	ctx := WithTenantID(context.Background(), "tenant-a")
	svc := newTaskStudioBatchRunService(taskStudioBatchRunServiceConfig{
		runner: studiodomain.NewBatchRunService(studiodomain.BatchRunServiceConfig{
			Repo: studioBatchRunDomainRepoStub{},
			SessionRepo: studioBatchRunDomainSessionStub{
				session: &studiodomain.BatchSeedSession{SavedAsBatch: true},
			},
			StartRun:      func(context.Context, string) error { return nil },
			NewRunID:      func() string { return "run-domain-1" },
			RequestUserID: func(context.Context) string { return "user-1" },
		}),
	})

	run, items, err := svc.CreateStudioBatchRun(ctx, &CreateStudioBatchRunRequest{BatchIDs: []string{"batch-1"}})
	if err != nil {
		t.Fatalf("CreateStudioBatchRun() error = %v", err)
	}
	if run == nil || run.ID != "run-domain-1" || run.Mode != StudioBatchRunModeGenerate {
		t.Fatalf("run = %+v", run)
	}
	if len(items) != 1 || items[0].BatchID != "batch-1" {
		t.Fatalf("items = %+v", items)
	}
}

func TestTaskStudioBatchRunServiceMapsListingStudioMissingBatchError(t *testing.T) {
	svc := newTaskStudioBatchRunService(taskStudioBatchRunServiceConfig{
		runner: studiodomain.NewBatchRunService(studiodomain.BatchRunServiceConfig{
			Repo:          studioBatchRunDomainRepoStub{},
			SessionRepo:   studioBatchRunDomainSessionStub{},
			StartRun:      func(context.Context, string) error { return nil },
			NewRunID:      func() string { return "run-domain-1" },
			RequestUserID: func(context.Context) string { return "user-1" },
		}),
	})

	_, _, err := svc.CreateStudioBatchRun(context.Background(), &CreateStudioBatchRunRequest{BatchIDs: []string{"batch-1"}})
	if !errors.Is(err, ErrStudioSessionNotFound) {
		t.Fatalf("CreateStudioBatchRun() error = %v, want ErrStudioSessionNotFound", err)
	}
}

func TestStudioBatchRunServiceGetAndListProxyToRepository(t *testing.T) {
	repo := NewMemStudioBatchRunRepository()
	batchRepo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	run, wantItems := mustCreateStudioBatchRunForTest(t, repo, ctx, "run-1", []string{"batch-1", "batch-2"})
	now := time.Now().UTC()
	if err := batchRepo.CreateStudioBatchGraph(ctx, newStudioBatchRecordForTest("batch-1", now), []StudioBatchItemRecord{{
		ID:             "item-1",
		BatchID:        "batch-1",
		Status:         StudioBatchItemStatusGenerating,
		LastError:      "waiting upstream",
		SelectionCount: 1,
		CreatedAt:      now,
		UpdatedAt:      now,
	}}, nil, nil); err != nil {
		t.Fatalf("CreateStudioBatchGraph(batch-1) error = %v", err)
	}
	batch2 := newStudioBatchRecordForTest("batch-2", now.Add(time.Second))
	batch2.Status = StudioBatchStatusFailed
	if err := batchRepo.CreateStudioBatchGraph(ctx, batch2, []StudioBatchItemRecord{{
		ID:             "item-2",
		BatchID:        "batch-2",
		Status:         StudioBatchItemStatusFailed,
		LastError:      "provider failed",
		SelectionCount: 1,
		CreatedAt:      now.Add(time.Second),
		UpdatedAt:      now.Add(time.Second),
	}}, nil, nil); err != nil {
		t.Fatalf("CreateStudioBatchGraph(batch-2) error = %v", err)
	}

	svc := &service{
		studioDeps: studioDependencies{batchRepo: batchRepo},
		studio: studioCollaborators{
			runGroup: taskStudioBatchRunCollaborators{
				batchRun: &taskStudioBatchRunService{repo: repo, batchRepo: batchRepo},
			},
		},
	}

	gotRun, err := svc.GetStudioBatchRun(ctx, "run-1")
	if err != nil {
		t.Fatalf("GetStudioBatchRun() error = %v", err)
	}
	if gotRun == nil || gotRun.ID != run.ID {
		t.Fatalf("GetStudioBatchRun() = %+v, want run %q", gotRun, run.ID)
	}

	gotItems, err := svc.ListStudioBatchRunItems(ctx, "run-1")
	if err != nil {
		t.Fatalf("ListStudioBatchRunItems() error = %v", err)
	}
	if len(gotItems) != len(wantItems) {
		t.Fatalf("len(gotItems) = %d, want %d", len(gotItems), len(wantItems))
	}
	for i := range wantItems {
		if gotItems[i].ID != wantItems[i].ID || gotItems[i].BatchID != wantItems[i].BatchID || gotItems[i].Position != wantItems[i].Position {
			t.Fatalf("gotItems[%d] = %+v, want %+v", i, gotItems[i], wantItems[i])
		}
	}
	if gotItems[0].BatchStatus != StudioBatchStatusGenerating || gotItems[0].BatchLastError != "waiting upstream" {
		t.Fatalf("gotItems[0] diagnostics = %+v, want generating/waiting upstream", gotItems[0])
	}
	if gotItems[1].BatchStatus != StudioBatchStatusFailed || gotItems[1].BatchLastError != "provider failed" {
		t.Fatalf("gotItems[1] diagnostics = %+v, want failed/provider failed", gotItems[1])
	}
}

func TestStudioBatchRunServiceCancelMarksRunAsCancelRequested(t *testing.T) {
	repo := NewMemStudioBatchRunRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	run, _ := mustCreateStudioBatchRunForTest(t, repo, ctx, "run-1", []string{"batch-1"})

	svc := &service{
		studio: studioCollaborators{
			runGroup: taskStudioBatchRunCollaborators{
				batchRun: &taskStudioBatchRunService{repo: repo},
			},
		},
	}

	if err := svc.CancelStudioBatchRun(ctx, run.ID); err != nil {
		t.Fatalf("CancelStudioBatchRun() error = %v", err)
	}
	updatedRun, err := repo.GetStudioBatchRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetStudioBatchRun() error = %v", err)
	}
	if !updatedRun.CancelRequested {
		t.Fatalf("updatedRun.CancelRequested = false, want true")
	}
}

func TestStudioBatchRunServiceRecoverStartsPendingRun(t *testing.T) {
	repo := NewMemStudioBatchRunRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	run, _ := mustCreateStudioBatchRunForTest(t, repo, ctx, "run-1", []string{"batch-1"})

	var recoveredRunID string
	svc := &service{
		studio: studioCollaborators{
			runGroup: taskStudioBatchRunCollaborators{
				batchRun: &taskStudioBatchRunService{
					repo: repo,
					startRun: func(_ context.Context, runID string) error {
						recoveredRunID = runID
						return nil
					},
				},
			},
		},
	}

	if err := svc.RecoverStudioBatchRun(ctx, run.ID); err != nil {
		t.Fatalf("RecoverStudioBatchRun() error = %v", err)
	}
	if recoveredRunID != "run-1" {
		t.Fatalf("recoveredRunID = %q, want run-1", recoveredRunID)
	}
}

func TestStudioBatchRunServiceRecoverRejectsRunningAndSucceededRuns(t *testing.T) {
	repo := NewMemStudioBatchRunRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	runRunning, _ := mustCreateStudioBatchRunForTest(t, repo, ctx, "run-running", []string{"batch-1"})
	runRunning.Status = StudioBatchRunStatusRunning
	if err := repo.UpdateStudioBatchRun(ctx, runRunning); err != nil {
		t.Fatalf("UpdateStudioBatchRun(run-running) error = %v", err)
	}
	runSucceeded, _ := mustCreateStudioBatchRunForTest(t, repo, ctx, "run-succeeded", []string{"batch-2"})
	runSucceeded.Status = StudioBatchRunStatusSucceeded
	if err := repo.UpdateStudioBatchRun(ctx, runSucceeded); err != nil {
		t.Fatalf("UpdateStudioBatchRun(run-succeeded) error = %v", err)
	}

	svc := &service{
		studio: studioCollaborators{
			runGroup: taskStudioBatchRunCollaborators{
				batchRun: &taskStudioBatchRunService{
					repo: repo,
					startRun: func(_ context.Context, runID string) error {
						t.Fatalf("startRun should not be called for %s", runID)
						return nil
					},
				},
			},
		},
	}

	if err := svc.RecoverStudioBatchRun(ctx, "run-running"); err == nil {
		t.Fatal("RecoverStudioBatchRun(run-running) error = nil, want validation error")
	}
	if err := svc.RecoverStudioBatchRun(ctx, "run-succeeded"); err == nil {
		t.Fatal("RecoverStudioBatchRun(run-succeeded) error = nil, want validation error")
	}
}

type studioBatchRunTestSessionRepo struct {
	sessions map[string]*SheinStudioSession
}

func newStudioBatchRunTestSessionRepo() *studioBatchRunTestSessionRepo {
	return &studioBatchRunTestSessionRepo{
		sessions: map[string]*SheinStudioSession{},
	}
}

func (r *studioBatchRunTestSessionRepo) FindLatestSessionBySelectionKey(context.Context, string) (*SheinStudioSession, error) {
	return nil, nil
}

func (r *studioBatchRunTestSessionRepo) CreateSession(_ context.Context, session *SheinStudioSession) error {
	r.sessions[session.ID] = cloneStudioBatchRunTestSession(session)
	return nil
}

func (r *studioBatchRunTestSessionRepo) GetSession(_ context.Context, sessionID string) (*SheinStudioSession, error) {
	session := r.sessions[sessionID]
	return cloneStudioBatchRunTestSession(session), nil
}

func (r *studioBatchRunTestSessionRepo) UpdateSession(_ context.Context, session *SheinStudioSession) error {
	r.sessions[session.ID] = cloneStudioBatchRunTestSession(session)
	return nil
}

func (r *studioBatchRunTestSessionRepo) DeleteSession(_ context.Context, sessionID string) error {
	delete(r.sessions, sessionID)
	return nil
}

func (r *studioBatchRunTestSessionRepo) ReplaceDesigns(context.Context, string, []string, []SheinStudioDesign) error {
	return nil
}

func (r *studioBatchRunTestSessionRepo) UpsertDesigns(context.Context, string, []string, []SheinStudioDesign) error {
	return nil
}

func (r *studioBatchRunTestSessionRepo) ListSessionDesigns(context.Context, string) ([]SheinStudioDesign, error) {
	return nil, nil
}

func (r *studioBatchRunTestSessionRepo) CountSessionDesignsBySessionIDs(context.Context, []string) (map[string]int, error) {
	return map[string]int{}, nil
}

func (r *studioBatchRunTestSessionRepo) ListGalleryItems(context.Context, int) ([]SheinStudioSessionGalleryItem, error) {
	return nil, nil
}

func (r *studioBatchRunTestSessionRepo) ListBatchSessions(context.Context, int) ([]SheinStudioSession, error) {
	return nil, nil
}

func (r *studioBatchRunTestSessionRepo) ListTenantBatchNames(context.Context) ([]string, error) {
	return nil, nil
}

func seedStudioBatchRunSavedBatch(t *testing.T, repo *studioBatchRunTestSessionRepo, ctx context.Context, batchID string) {
	t.Helper()

	now := time.Unix(1717113600, 0).UTC()
	session := &SheinStudioSession{
		ID:           batchID,
		TenantID:     TenantIDFromContextOrTest(ctx),
		Status:       SheinStudioSessionStatusReviewing,
		SavedAsBatch: true,
		Selection: SheinStudioSelectionSnapshot{
			VariantID: 1,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := repo.CreateSession(ctx, session); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
}

func mustCreateStudioBatchRunForTest(t *testing.T, repo StudioBatchRunRepository, ctx context.Context, runID string, batchIDs []string) (*StudioBatchRunRecord, []StudioBatchRunItemRecord) {
	t.Helper()

	run := &StudioBatchRunRecord{
		ID:            runID,
		Status:        StudioBatchRunStatusPending,
		Mode:          StudioBatchRunModeGenerate,
		FailurePolicy: StudioBatchRunFailurePolicyContinueOnError,
		TotalBatches:  len(batchIDs),
	}
	items := make([]StudioBatchRunItemRecord, 0, len(batchIDs))
	for i, batchID := range batchIDs {
		items = append(items, StudioBatchRunItemRecord{
			ID:       runID + ":" + batchID,
			RunID:    runID,
			BatchID:  batchID,
			Position: i + 1,
			Status:   StudioBatchRunItemStatusPending,
		})
	}
	if err := repo.CreateStudioBatchRun(ctx, run, items); err != nil {
		t.Fatalf("CreateStudioBatchRun() error = %v", err)
	}
	return run, items
}

func cloneStudioBatchRunTestSession(session *SheinStudioSession) *SheinStudioSession {
	if session == nil {
		return nil
	}
	cloned := *session
	cloned.SelectedVariantIDs = append(SheinStudioInt64List(nil), session.SelectedVariantIDs...)
	cloned.ProductImagePrompts = append(SheinStudioProductImagePromptList(nil), session.ProductImagePrompts...)
	cloned.ApprovedDesignIDs = append(SheinStudioStringList(nil), session.ApprovedDesignIDs...)
	cloned.CreatedTaskIDs = append(SheinStudioStringList(nil), session.CreatedTaskIDs...)
	cloned.CreatedTasks = append(SheinStudioCreatedTaskList(nil), session.CreatedTasks...)
	cloned.GenerationJobs = append(SheinStudioGenerationJobList(nil), session.GenerationJobs...)
	cloned.GroupedSelections = append(SheinStudioGroupedSelectionList(nil), session.GroupedSelections...)
	cloned.Selection = session.Selection
	return &cloned
}

func TenantIDFromContextOrTest(ctx context.Context) string {
	tenantID, _ := TenantScopeFromContext(ctx)
	return tenantID
}

type studioBatchRunDomainRepoStub struct{}

func (studioBatchRunDomainRepoStub) CreateBatchRun(context.Context, *studiodomain.BatchRunRecord, []studiodomain.BatchRunItemRecord) error {
	return nil
}

func (studioBatchRunDomainRepoStub) GetBatchRun(context.Context, string) (*studiodomain.BatchRunRecord, error) {
	return nil, nil
}

func (studioBatchRunDomainRepoStub) ListBatchRunItems(context.Context, string) ([]studiodomain.BatchRunItemRecord, error) {
	return nil, nil
}

func (studioBatchRunDomainRepoStub) UpdateBatchRun(context.Context, *studiodomain.BatchRunRecord) error {
	return nil
}

type studioBatchRunDomainSessionStub struct {
	session *studiodomain.BatchSeedSession
}

func (s studioBatchRunDomainSessionStub) GetSession(context.Context, string) (*studiodomain.BatchSeedSession, error) {
	return s.session, nil
}

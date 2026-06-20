package listingkit

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	openaiclient "task-processor/internal/infra/clients/openai"
)

func TestStudioBatchRunExecutorContinuesAfterOneItemFailure(t *testing.T) {
	repo := NewMemStudioBatchRunRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	_, _ = mustCreateStudioBatchRunForTest(t, repo, ctx, "run-1", []string{"batch-1", "batch-2"})

	executor := newTaskStudioBatchRunExecutor(taskStudioBatchRunExecutorConfig{
		repo: repo,
		executeOne: func(ctx context.Context, batchID string) error {
			if batchID == "batch-1" {
				return errors.New("upstream failed")
			}
			return nil
		},
	})

	if err := executor.Run(ctx, "run-1"); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	run, err := repo.GetStudioBatchRun(ctx, "run-1")
	if err != nil {
		t.Fatalf("GetStudioBatchRun() error = %v", err)
	}
	if run.Status != StudioBatchRunStatusPartiallySucceeded {
		t.Fatalf("run.Status = %q, want %q", run.Status, StudioBatchRunStatusPartiallySucceeded)
	}
	if run.FailedBatches != 1 {
		t.Fatalf("run.FailedBatches = %d, want 1", run.FailedBatches)
	}
	if run.SucceededBatches != 1 {
		t.Fatalf("run.SucceededBatches = %d, want 1", run.SucceededBatches)
	}
	if run.CompletedBatches != 2 {
		t.Fatalf("run.CompletedBatches = %d, want 2", run.CompletedBatches)
	}
	if run.LastError != "upstream failed" {
		t.Fatalf("run.LastError = %q, want %q", run.LastError, "upstream failed")
	}

	items, err := repo.ListStudioBatchRunItems(ctx, "run-1")
	if err != nil {
		t.Fatalf("ListStudioBatchRunItems() error = %v", err)
	}
	if items[0].Status != StudioBatchRunItemStatusFailed {
		t.Fatalf("items[0].Status = %q, want %q", items[0].Status, StudioBatchRunItemStatusFailed)
	}
	if items[0].ErrorMessage != "upstream failed" {
		t.Fatalf("items[0].ErrorMessage = %q, want %q", items[0].ErrorMessage, "upstream failed")
	}
	if items[1].Status != StudioBatchRunItemStatusSucceeded {
		t.Fatalf("items[1].Status = %q, want %q", items[1].Status, StudioBatchRunItemStatusSucceeded)
	}
}

func TestStudioBatchRunExecutorStopsStartingNewItemsAfterCancelRequested(t *testing.T) {
	repo := NewMemStudioBatchRunRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	_, _ = mustCreateStudioBatchRunForTest(t, repo, ctx, "run-1", []string{"batch-1", "batch-2"})
	mustCancelStudioBatchRunForTest(t, repo, ctx, "run-1")

	executor := newTaskStudioBatchRunExecutor(taskStudioBatchRunExecutorConfig{
		repo: repo,
		executeOne: func(ctx context.Context, batchID string) error {
			t.Fatalf("executeOne should not start when cancellation is already requested")
			return nil
		},
	})

	if err := executor.Run(ctx, "run-1"); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	run, err := repo.GetStudioBatchRun(ctx, "run-1")
	if err != nil {
		t.Fatalf("GetStudioBatchRun() error = %v", err)
	}
	if run.Status != StudioBatchRunStatusCancelled {
		t.Fatalf("run.Status = %q, want %q", run.Status, StudioBatchRunStatusCancelled)
	}
	if run.CompletedBatches != 2 {
		t.Fatalf("run.CompletedBatches = %d, want 2", run.CompletedBatches)
	}

	items, err := repo.ListStudioBatchRunItems(ctx, "run-1")
	if err != nil {
		t.Fatalf("ListStudioBatchRunItems() error = %v", err)
	}
	for i := range items {
		if items[i].Status != StudioBatchRunItemStatusCancelled {
			t.Fatalf("items[%d].Status = %q, want %q", i, items[i].Status, StudioBatchRunItemStatusCancelled)
		}
	}
}

func TestStudioBatchRunExecutorCancelsRunningAndPendingItemsWhenCancelRequestedBeforeResume(t *testing.T) {
	repo := NewMemStudioBatchRunRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	run, items := mustCreateStudioBatchRunForTest(t, repo, ctx, "run-1", []string{"batch-1", "batch-2"})

	items[0].Status = StudioBatchRunItemStatusRunning
	if err := repo.UpdateStudioBatchRunItem(ctx, &items[0]); err != nil {
		t.Fatalf("UpdateStudioBatchRunItem() error = %v", err)
	}
	run.CancelRequested = true
	run.Status = StudioBatchRunStatusRunning
	run.CurrentBatchID = items[0].BatchID
	run.CurrentIndex = items[0].Position
	if err := repo.UpdateStudioBatchRun(ctx, run); err != nil {
		t.Fatalf("UpdateStudioBatchRun() error = %v", err)
	}

	executor := newTaskStudioBatchRunExecutor(taskStudioBatchRunExecutorConfig{
		repo: repo,
		executeOne: func(ctx context.Context, batchID string) error {
			t.Fatalf("executeOne should not run when cancellation is already requested")
			return nil
		},
	})

	if err := executor.Run(ctx, "run-1"); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	run, err := repo.GetStudioBatchRun(ctx, "run-1")
	if err != nil {
		t.Fatalf("GetStudioBatchRun() error = %v", err)
	}
	if run.Status != StudioBatchRunStatusCancelled {
		t.Fatalf("run.Status = %q, want %q", run.Status, StudioBatchRunStatusCancelled)
	}

	items, err = repo.ListStudioBatchRunItems(ctx, "run-1")
	if err != nil {
		t.Fatalf("ListStudioBatchRunItems() error = %v", err)
	}
	for i := range items {
		if items[i].Status != StudioBatchRunItemStatusCancelled {
			t.Fatalf("items[%d].Status = %q, want %q", i, items[i].Status, StudioBatchRunItemStatusCancelled)
		}
	}
}

func TestStudioBatchRunExecutorTreatsGenerationErrorClearFailureAsSuccessAfterDesignPersistence(t *testing.T) {
	repo := NewMemStudioBatchRepository()
	sessionRepo := &studioBatchRunExecutorSessionRepoStub{
		session: &SheinStudioSession{
			ID:               "batch-1",
			SavedAsBatch:     true,
			Prompt:           "retro cherries",
			StyleCount:       "1",
			GroupedImageMode: "per_product",
			Selection: SheinStudioSelectionSnapshot{
				ProductID:          101,
				ParentProductID:    7001,
				VariantID:          101,
				PrototypeGroupID:   9001,
				LayerID:            "layer-1",
				ProductName:        "Canvas Tote",
				VariantLabel:       "Red",
				PrintableWidth:     1200,
				PrintableHeight:    1200,
				SelectedVariantIDs: []int64{101},
				MockupImageURL:     "https://example.com/mockup.png",
			},
		},
	}
	svc := &service{studioDeps: studioDependencies{sessionRepo: sessionRepo, batchRepo: repo}}
	svc.studio.batchGroup.batch = newTaskStudioBatchService(taskStudioBatchServiceConfig{
		repo:              repo,
		studioSessionRepo: sessionRepo,
		generator: newStudioBatchGenerationService(studioBatchGenerationServiceConfig{
			repo: repo,
			execute: func(ctx context.Context, input StudioBatchGenerateExecutionInput) (*StudioBatchGenerateExecutionOutput, error) {
				return &StudioBatchGenerateExecutionOutput{
					Response:  testStudioDesignResponse("design-1", "https://example.com/design.png"),
					BatchID:   input.BatchID,
					ItemID:    input.ItemID,
					AttemptID: input.AttemptID,
				}, nil
			},
		}),
	})

	ctx := WithTenantID(context.Background(), "tenant-a")
	err := svc.executeStudioBatchRunItem(ctx, "batch-1")
	if err != nil {
		t.Fatalf("executeStudioBatchRunItem() error = %v, want nil after itemized designs persisted", err)
	}

	detail, err := repo.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}
	if detail.Batch == nil || detail.Batch.Status != StudioBatchStatusReviewReady {
		t.Fatalf("detail.Batch = %+v, want review_ready batch", detail.Batch)
	}
	if len(detail.DesignsByItem["batch-1:item:1"]) != 1 {
		t.Fatalf("designs = %+v, want 1 materialized design", detail.DesignsByItem)
	}
}

func TestExecuteStudioBatchRunItemResumesExistingGraphWithoutWipingMaterializedDesigns(t *testing.T) {
	repo := NewMemStudioBatchRepository()
	sessionRepo := &studioBatchRunExecutorSessionRepoStub{
		session: &SheinStudioSession{
			ID:               "batch-1",
			SavedAsBatch:     true,
			Prompt:           "new prompt that should not overwrite resume",
			StyleCount:       "1",
			GroupedImageMode: "shared_by_size",
			Selection: SheinStudioSelectionSnapshot{
				ProductID:          101,
				ParentProductID:    7001,
				VariantID:          101,
				PrototypeGroupID:   9001,
				LayerID:            "layer-1",
				ProductName:        "Canvas Tote",
				VariantLabel:       "Red",
				PrintableWidth:     1200,
				PrintableHeight:    1200,
				SelectedVariantIDs: []int64{101},
			},
		},
	}
	svc := &service{studioDeps: studioDependencies{sessionRepo: sessionRepo, batchRepo: repo}}
	svc.studio.batchGroup.batch = newTaskStudioBatchService(taskStudioBatchServiceConfig{
		repo:              repo,
		studioSessionRepo: sessionRepo,
		generator: newStudioBatchGenerationService(studioBatchGenerationServiceConfig{
			repo:        repo,
			execute:     stubStudioBatchExecutionByItem(map[string]*StudioDesignResponse{}),
			currentTime: func() time.Time { return time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC) },
		}),
	})

	ctx := WithTenantID(context.Background(), "tenant-a")
	resultPayload, err := json.Marshal(testStudioDesignResponse("design-1", "https://example.com/design.png"))
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	seedStudioBatchGenerationGraph(t, repo, ctx, studioBatchGenerationSeed{
		batch: StudioBatchRecord{
			ID:               "batch-1",
			Status:           StudioBatchStatusReviewReady,
			Prompt:           "persisted prompt",
			GroupedImageMode: "per_product",
		},
		items: []StudioBatchItemRecord{{
			ID:               "item-1",
			BatchID:          "batch-1",
			TargetGroupKey:   "7001:9001:101:layer-1:101",
			TargetGroupLabel: "Canvas Tote · Red",
			GroupMode:        "per_product",
			Status:           StudioBatchItemStatusReviewReady,
			SelectionCount:   1,
		}},
		attempts: []StudioGenerationAttemptRecord{{
			ID:            "attempt-1",
			ItemID:        "item-1",
			AttemptNo:     1,
			Status:        StudioGenerationAttemptStatusMaterialized,
			ResultPayload: string(resultPayload),
		}},
		designs: []StudioMaterializedDesignRecord{{
			ID:               "design-1",
			BatchID:          "batch-1",
			ItemID:           "item-1",
			SourceAttemptID:  "attempt-1",
			TargetGroupKey:   "7001:9001:101:layer-1:101",
			TargetGroupLabel: "Canvas Tote · Red",
			ImageURL:         "https://example.com/design.png",
		}},
	})

	if err := svc.executeStudioBatchRunItem(ctx, "batch-1"); err != nil {
		t.Fatalf("executeStudioBatchRunItem() error = %v", err)
	}

	detail, err := repo.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}
	if detail.Batch == nil || detail.Batch.Prompt != "persisted prompt" {
		t.Fatalf("detail.Batch = %+v, want existing graph preserved on resume", detail.Batch)
	}
	if len(detail.AttemptsByItem["item-1"]) != 1 {
		t.Fatalf("attempts = %+v, want preserved attempt graph", detail.AttemptsByItem)
	}
	if len(detail.DesignsByItem["item-1"]) != 1 {
		t.Fatalf("designs = %+v, want preserved materialized design graph", detail.DesignsByItem)
	}
}

func TestStudioBatchRunCoordinatorRecoversOnlyUnfinishedRunsAndContinuesAfterErrors(t *testing.T) {
	repo := NewMemStudioBatchRunRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	_, _ = mustCreateStudioBatchRunForTest(t, repo, ctx, "run-pending", []string{"batch-1"})
	_, _ = mustCreateStudioBatchRunForTest(t, repo, ctx, "run-running", []string{"batch-2"})
	_, _ = mustCreateStudioBatchRunForTest(t, repo, ctx, "run-done", []string{"batch-3"})

	run, err := repo.GetStudioBatchRun(ctx, "run-running")
	if err != nil {
		t.Fatalf("GetStudioBatchRun(run-running) error = %v", err)
	}
	run.Status = StudioBatchRunStatusRunning
	if err := repo.UpdateStudioBatchRun(ctx, run); err != nil {
		t.Fatalf("UpdateStudioBatchRun(run-running) error = %v", err)
	}

	run, err = repo.GetStudioBatchRun(ctx, "run-done")
	if err != nil {
		t.Fatalf("GetStudioBatchRun(run-done) error = %v", err)
	}
	run.Status = StudioBatchRunStatusSucceeded
	if err := repo.UpdateStudioBatchRun(ctx, run); err != nil {
		t.Fatalf("UpdateStudioBatchRun(run-done) error = %v", err)
	}

	var recovered []string
	coordinator := newStudioBatchRunCoordinator(studioBatchRunCoordinatorConfig{
		repo: repo,
		recoverRun: func(ctx context.Context, runID string) error {
			recovered = append(recovered, runID)
			if runID == "run-pending" {
				return errors.New("resume failed")
			}
			return nil
		},
	})

	err = coordinator.RecoverUnfinishedRuns(ctx)
	if err == nil {
		t.Fatal("RecoverUnfinishedRuns() error = nil, want aggregated recovery error")
	}
	if !strings.Contains(err.Error(), "run-pending") {
		t.Fatalf("RecoverUnfinishedRuns() error = %v, want run id in error", err)
	}
	if len(recovered) != 2 {
		t.Fatalf("len(recovered) = %d, want 2", len(recovered))
	}
	if recovered[0] != "run-pending" || recovered[1] != "run-running" {
		t.Fatalf("recovered = %v, want [run-pending run-running]", recovered)
	}
}

func TestStudioBatchRunCoordinatorStartRunDetachesRequestContext(t *testing.T) {
	baseCtx, cancel := context.WithCancel(WithTenantID(context.Background(), "tenant-a"))
	baseCtx = openaiclient.WithIdentity(baseCtx, openaiclient.Identity{TenantID: "tenant-a", UserID: "user-42"})
	baseCtx = WithRequestRoles(baseCtx, []string{"studio:write"})
	baseCtx = WithRequestTrace(baseCtx, RequestTrace{
		BatchRunID: "parent-run",
		BatchID:    "batch-9",
		SessionID:  "session-9",
		QueueMode:  "generate",
		QueueIndex: 2,
		QueueTotal: 4,
	})

	type startedRun struct {
		runID     string
		tenantID  string
		userID    string
		roles     []string
		trace     RequestTrace
		cancelled bool
	}
	startedCh := make(chan startedRun, 1)
	coordinator := newStudioBatchRunCoordinator(studioBatchRunCoordinatorConfig{
		recoverRun: func(ctx context.Context, runID string) error {
			select {
			case <-ctx.Done():
				startedCh <- startedRun{cancelled: true}
			default:
				startedCh <- startedRun{
					runID:     runID,
					tenantID:  TenantIDFromContext(ctx),
					userID:    RequestUserIDFromContext(ctx),
					roles:     RequestRolesFromContext(ctx),
					trace:     RequestTraceFromContext(ctx),
					cancelled: false,
				}
			}
			return nil
		},
	})

	if err := coordinator.StartRun(baseCtx, "run-1"); err != nil {
		t.Fatalf("StartRun() error = %v", err)
	}
	cancel()

	select {
	case started := <-startedCh:
		if started.cancelled {
			t.Fatal("started run context was cancelled, want detached background context")
		}
		if started.runID != "run-1" {
			t.Fatalf("started.runID = %q, want %q", started.runID, "run-1")
		}
		if started.tenantID != "tenant-a" {
			t.Fatalf("started.tenantID = %q, want %q", started.tenantID, "tenant-a")
		}
		if started.userID != "user-42" {
			t.Fatalf("started.userID = %q, want %q", started.userID, "user-42")
		}
		if len(started.roles) != 1 || started.roles[0] != "studio:write" {
			t.Fatalf("started.roles = %v, want [studio:write]", started.roles)
		}
		if started.trace.BatchRunID != "parent-run" || started.trace.BatchID != "batch-9" || started.trace.SessionID != "session-9" {
			t.Fatalf("started.trace = %+v, want propagated trace ids", started.trace)
		}
		if started.trace.QueueMode != "generate" || started.trace.QueueIndex != 2 || started.trace.QueueTotal != 4 {
			t.Fatalf("started.trace = %+v, want propagated queue metadata", started.trace)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("StartRun() did not launch recoverRun callback")
	}
}

func mustCancelStudioBatchRunForTest(t *testing.T, repo StudioBatchRunRepository, ctx context.Context, runID string) {
	t.Helper()

	run, err := repo.GetStudioBatchRun(ctx, runID)
	if err != nil {
		t.Fatalf("GetStudioBatchRun() error = %v", err)
	}
	run.CancelRequested = true
	if err := repo.UpdateStudioBatchRun(ctx, run); err != nil {
		t.Fatalf("UpdateStudioBatchRun() error = %v", err)
	}
}

type studioBatchRunExecutorSessionRepoStub struct {
	session                *SheinStudioSession
	replaceCalled          bool
	failUpdateAfterReplace bool
}

func (s *studioBatchRunExecutorSessionRepoStub) FindLatestSessionBySelectionKey(context.Context, string) (*SheinStudioSession, error) {
	return nil, nil
}

func (s *studioBatchRunExecutorSessionRepoStub) CreateSession(context.Context, *SheinStudioSession) error {
	return nil
}

func (s *studioBatchRunExecutorSessionRepoStub) GetSession(context.Context, string) (*SheinStudioSession, error) {
	if s.session == nil {
		return nil, nil
	}
	cloned := *s.session
	return &cloned, nil
}

func (s *studioBatchRunExecutorSessionRepoStub) UpdateSession(_ context.Context, session *SheinStudioSession) error {
	if s.failUpdateAfterReplace && s.replaceCalled {
		return errors.New("clear generation error failed")
	}
	if session != nil {
		cloned := *session
		s.session = &cloned
	}
	return nil
}

func (s *studioBatchRunExecutorSessionRepoStub) DeleteSession(context.Context, string) error {
	return nil
}

func (s *studioBatchRunExecutorSessionRepoStub) ReplaceDesigns(context.Context, string, []string, []SheinStudioDesign) error {
	s.replaceCalled = true
	return nil
}

func (s *studioBatchRunExecutorSessionRepoStub) UpsertDesigns(context.Context, string, []string, []SheinStudioDesign) error {
	return nil
}

func (s *studioBatchRunExecutorSessionRepoStub) ListSessionDesigns(context.Context, string) ([]SheinStudioDesign, error) {
	return nil, nil
}

func (s *studioBatchRunExecutorSessionRepoStub) CountSessionDesignsBySessionIDs(context.Context, []string) (map[string]int, error) {
	return nil, nil
}

func (s *studioBatchRunExecutorSessionRepoStub) ListGalleryItems(context.Context, int) ([]SheinStudioSessionGalleryItem, error) {
	return nil, nil
}

func (s *studioBatchRunExecutorSessionRepoStub) ListBatchSessions(context.Context, int) ([]SheinStudioSession, error) {
	return nil, nil
}

func (s *studioBatchRunExecutorSessionRepoStub) ListTenantBatchNames(context.Context) ([]string, error) {
	return nil, nil
}

type studioBatchRunExecutorImageGeneratorStub struct{}

func (s *studioBatchRunExecutorImageGeneratorStub) GenerateImage(context.Context, *openaiclient.ImageGenerateRequest) (*openaiclient.ImageResponse, error) {
	return &openaiclient.ImageResponse{
		Created: 1,
		Data: []openaiclient.ImageData{{
			B64JSON: base64.StdEncoding.EncodeToString([]byte{0xFF, 0xD8, 0xFF, 0xD9}),
		}},
	}, nil
}

func (s *studioBatchRunExecutorImageGeneratorStub) EditImage(context.Context, *openaiclient.ImageEditRequest) (*openaiclient.ImageResponse, error) {
	return &openaiclient.ImageResponse{
		Created: 1,
		Data: []openaiclient.ImageData{{
			B64JSON: base64.StdEncoding.EncodeToString([]byte{0xFF, 0xD8, 0xFF, 0xD9}),
		}},
	}, nil
}

func (s *studioBatchRunExecutorImageGeneratorStub) GetDefaultModel() string {
	return "gpt-image-1"
}

func (s *studioBatchRunExecutorImageGeneratorStub) SupportsAsyncImageGeneration() bool {
	return false
}

func (s *studioBatchRunExecutorImageGeneratorStub) SubmitImageGeneration(context.Context, *openaiclient.ImageGenerateRequest) (*openaiclient.ImageAsyncSubmitResponse, error) {
	return nil, openaiclient.ErrAsyncImageGenerationNotSupported
}

func (s *studioBatchRunExecutorImageGeneratorStub) SubmitImageEdit(context.Context, *openaiclient.ImageEditRequest) (*openaiclient.ImageAsyncSubmitResponse, error) {
	return nil, openaiclient.ErrAsyncImageGenerationNotSupported
}

func (s *studioBatchRunExecutorImageGeneratorStub) QueryImageGeneration(context.Context, string) (*openaiclient.ImageAsyncQueryResponse, error) {
	return nil, openaiclient.ErrAsyncImageGenerationNotSupported
}

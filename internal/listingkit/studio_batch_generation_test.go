package listingkit

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestStartStudioBatchGenerationExpandsPerProductIntoSeparateItems(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	sessionRepo := &studioBatchGenerationSessionRepoStub{
		session: &SheinStudioSession{
			ID:               "batch-1",
			SavedAsBatch:     true,
			Status:           SheinStudioSessionStatusSelecting,
			Prompt:           "retro summer fruit",
			StyleCount:       "1",
			ArtworkModel:     "gpt-image-1",
			GroupedImageMode: "per_product",
			ImageStrategy:    "sds_official",
			SheinStoreID:     "store-1",
			Selection:        SheinStudioSelectionSnapshot(testStudioBatchSelection(101, "Canvas Tote", "Red", 1200, 1200)),
			GroupedSelections: SheinStudioGroupedSelectionList{
				{
					SelectionID: "7001:9001:102:layer-1:102",
					Selection:   testStudioBatchSelection(102, "Canvas Tote", "Blue", 1200, 1200),
					Eligible:    true,
				},
			},
		},
	}
	service := newTaskStudioBatchService(taskStudioBatchServiceConfig{
		repo:              repo,
		studioSessionRepo: sessionRepo,
		generator: newStudioBatchGenerationService(studioBatchGenerationServiceConfig{
			repo:        repo,
			execute:     stubStudioBatchExecutionByItem(map[string]*StudioDesignResponse{}),
			currentTime: func() time.Time { return time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC) },
		}),
	})
	ctx := WithTenantID(context.Background(), "tenant-a")

	detail, err := service.StartStudioBatchGeneration(ctx, "batch-1")
	if err != nil {
		t.Fatalf("StartStudioBatchGeneration() error = %v", err)
	}

	if detail.Batch == nil || detail.Batch.ID != "batch-1" {
		t.Fatalf("detail.Batch = %+v, want batch-1", detail.Batch)
	}
	if len(detail.Items) != 2 {
		t.Fatalf("len(detail.Items) = %d, want 2", len(detail.Items))
	}
	if detail.Items[0].Item.TargetGroupKey != "7001:9001:101:layer-1:101" {
		t.Fatalf("item-1 target group key = %q, want per-product selection key", detail.Items[0].Item.TargetGroupKey)
	}
	if detail.Items[0].Item.TargetGroupLabel != "Canvas Tote · Red" {
		t.Fatalf("item-1 target group label = %q, want per-product label", detail.Items[0].Item.TargetGroupLabel)
	}
	if detail.Items[1].Item.TargetGroupKey != "7001:9001:102:layer-1:102" {
		t.Fatalf("item-2 target group key = %q, want second per-product selection key", detail.Items[1].Item.TargetGroupKey)
	}
	if detail.Items[1].Item.SelectionCount != 1 {
		t.Fatalf("item-2 selection count = %d, want 1", detail.Items[1].Item.SelectionCount)
	}

	graph, err := repo.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}
	if len(graph.Items) != 2 {
		t.Fatalf("persisted item count = %d, want 2", len(graph.Items))
	}
}

func TestStartStudioBatchGenerationRejectsPromptlessBatchBeforeCreatingGraph(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	sessionRepo := &studioBatchGenerationSessionRepoStub{
		session: &SheinStudioSession{
			ID:               "batch-1",
			SavedAsBatch:     true,
			Status:           SheinStudioSessionStatusSelecting,
			Prompt:           "",
			StyleCount:       "1",
			ArtworkModel:     "gpt-image-2",
			GroupedImageMode: "per_product",
			Selection:        SheinStudioSelectionSnapshot(testStudioBatchSelection(101, "Canvas Tote", "Red", 1200, 1200)),
		},
	}
	var executions int
	service := newTaskStudioBatchService(taskStudioBatchServiceConfig{
		repo:              repo,
		studioSessionRepo: sessionRepo,
		generator: newStudioBatchGenerationService(studioBatchGenerationServiceConfig{
			repo: repo,
			execute: func(ctx context.Context, input StudioBatchGenerateExecutionInput) (*StudioBatchGenerateExecutionOutput, error) {
				executions++
				return &StudioBatchGenerateExecutionOutput{
					Response: testStudioDesignResponse("design-1", "https://cdn.example.com/design.png"),
					ItemID:   input.ItemID,
					BatchID:  input.BatchID,
				}, nil
			},
			currentTime: func() time.Time { return time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC) },
		}),
	})
	ctx := WithTenantID(context.Background(), "tenant-a")

	_, err := service.StartStudioBatchGeneration(ctx, "batch-1")
	if err == nil {
		t.Fatal("StartStudioBatchGeneration() error = nil, want prompt validation error")
	}
	if !strings.Contains(err.Error(), "prompt is required") {
		t.Fatalf("StartStudioBatchGeneration() error = %v, want prompt validation error", err)
	}
	if executions != 0 {
		t.Fatalf("executions = %d, want no generation attempt", executions)
	}
	if _, getErr := repo.GetStudioBatch(ctx, "batch-1"); !errors.Is(getErr, gorm.ErrRecordNotFound) {
		t.Fatalf("GetStudioBatch() error = %v, want graph not created", getErr)
	}
}

func TestValidateStudioBatchRecordDesignSourceAllowsPromptlessHotReference(t *testing.T) {
	t.Parallel()

	batch := newStudioBatchRecordForTest("batch-1", time.Date(2026, 7, 5, 14, 0, 0, 0, time.UTC))
	batch.Prompt = ""
	batch.HotStyleReferenceImageURLs = SheinStudioStringList{"https://example.com/hot-ref.png"}

	if err := validateStudioBatchRecordDesignSource(batch); err != nil {
		t.Fatalf("validateStudioBatchRecordDesignSource() error = %v, want promptless hot reference to be valid", err)
	}
}

func TestStartStudioBatchGenerationPreservesExistingGraphWhenDraftIsPromptless(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Date(2026, 7, 4, 13, 0, 0, 0, time.UTC)
	batch := newStudioBatchRecordForTest("batch-1", now)
	batch.Prompt = ""
	batch.HotStyleReferenceImageURLs = SheinStudioStringList{"https://cdn.example.com/hot-ref.png"}
	batch.HotStyleReferenceBrief = "dense streetwear skull reference"
	batch.HotStyleReferencePrompt = "Create an original skull streetwear print."
	batch.ArtworkModel = "gpt-image-2"

	if err := repo.CreateStudioBatchGraph(ctx, batch, []StudioBatchItemRecord{
		{
			ID:               "item-1",
			BatchID:          "batch-1",
			TargetGroupKey:   "size:2880x3000",
			TargetGroupLabel: "2880 x 3000",
			Status:           StudioBatchItemStatusReviewReady,
			SelectionCount:   1,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
	}, []StudioGenerationAttemptRecord{
		{
			ID:        "attempt-1",
			BatchID:   "batch-1",
			ItemID:    "item-1",
			AttemptNo: 1,
			Status:    StudioGenerationAttemptStatusMaterialized,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, []StudioMaterializedDesignRecord{
		{
			ID:              "design-1",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			ImageURL:        "https://cdn.example.com/generated-hot-ref.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusUnreviewed,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	sessionRepo := &studioBatchGenerationSessionRepoStub{
		session: &SheinStudioSession{
			ID:               "batch-1",
			SavedAsBatch:     true,
			Status:           SheinStudioSessionStatusSelecting,
			Prompt:           "",
			StyleCount:       "1",
			ArtworkModel:     "",
			GroupedImageMode: "shared_by_size",
			Selection:        SheinStudioSelectionSnapshot(testStudioBatchSelection(101, "Dress", "White", 2880, 3000)),
		},
	}
	var executions int
	service := newTaskStudioBatchService(taskStudioBatchServiceConfig{
		repo:              repo,
		studioSessionRepo: sessionRepo,
		generator: newStudioBatchGenerationService(studioBatchGenerationServiceConfig{
			repo: repo,
			execute: func(ctx context.Context, input StudioBatchGenerateExecutionInput) (*StudioBatchGenerateExecutionOutput, error) {
				executions++
				return &StudioBatchGenerateExecutionOutput{
					Response: testStudioDesignResponse("design-new", "https://cdn.example.com/new.png"),
					ItemID:   input.ItemID,
					BatchID:  input.BatchID,
				}, nil
			},
			currentTime: func() time.Time { return now.Add(time.Minute) },
		}),
	})

	detail, err := service.StartStudioBatchGeneration(ctx, "batch-1")
	if err != nil {
		t.Fatalf("StartStudioBatchGeneration() error = %v", err)
	}
	if executions != 0 {
		t.Fatalf("executions = %d, want no new generation for review-ready graph", executions)
	}
	if detail.Batch == nil {
		t.Fatal("detail.Batch = nil")
	}
	if got := detail.Batch.HotStyleReferenceImageURLs; len(got) != 1 || got[0] != "https://cdn.example.com/hot-ref.png" {
		t.Fatalf("hot style reference urls = %#v, want existing graph reference preserved", got)
	}
	if got := detail.Batch.HotStyleReferencePrompt; got != "Create an original skull streetwear print." {
		t.Fatalf("hot style reference prompt = %q, want existing graph prompt preserved", got)
	}
	if len(detail.Items) != 1 {
		t.Fatalf("len(detail.Items) = %d, want 1", len(detail.Items))
	}
	if got := len(detail.Items[0].Designs); got != 1 {
		t.Fatalf("design count = %d, want existing materialized design preserved", got)
	}
}

func TestExpandStudioBatchItemsGroupsSharedSelectionsByCompatibilityFingerprint(t *testing.T) {
	t.Parallel()

	first := testStudioBatchSelection(101, "Canvas Tote", "Red", 1200, 1200)
	second := testStudioBatchSelection(102, "Canvas Tote", "Blue", 1200, 1200)
	second.TemplateImageURL = first.TemplateImageURL
	second.MaskImageURL = first.MaskImageURL
	third := testStudioBatchSelection(103, "Canvas Tote", "White", 1200, 1200)
	third.MaskImageURL = "https://cdn.example.com/mask-other.png"
	batch := &StudioBatchRecord{
		ID:               "batch-compat",
		GroupedImageMode: "shared_by_size",
		Selection:        SheinStudioSelectionSnapshot(first),
		GroupedSelections: SheinStudioGroupedSelectionList{
			{SelectionID: selectionIDForStudioSelection(second), Selection: second, Eligible: true},
			{SelectionID: selectionIDForStudioSelection(third), Selection: third, Eligible: true},
		},
	}

	items := expandStudioBatchItems(batch)

	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2 compatibility buckets: %+v", len(items), items)
	}
	wantFirstKey := buildStudioBatchSharedCompatibilityGroupKey(first)
	if items[0].TargetGroupKey != wantFirstKey {
		t.Fatalf("first target key = %q, want %q", items[0].TargetGroupKey, wantFirstKey)
	}
	if items[0].SelectionCount != 2 {
		t.Fatalf("first selection count = %d, want 2", items[0].SelectionCount)
	}
	wantSecondKey := buildStudioBatchSharedCompatibilityGroupKey(third)
	if items[1].TargetGroupKey != wantSecondKey {
		t.Fatalf("second target key = %q, want %q", items[1].TargetGroupKey, wantSecondKey)
	}
	if items[1].SelectionCount != 1 {
		t.Fatalf("second selection count = %d, want 1", items[1].SelectionCount)
	}
}

func TestExpandStudioBatchItemsFallsBackToPerProductWhenCompatibilityIncomplete(t *testing.T) {
	t.Parallel()

	selection := testStudioBatchSelection(101, "Canvas Tote", "Red", 1200, 1200)
	selection.MaskImageURL = ""
	batch := &StudioBatchRecord{
		ID:               "batch-incomplete",
		GroupedImageMode: "shared_by_size",
		Selection:        SheinStudioSelectionSnapshot(selection),
	}

	items := expandStudioBatchItems(batch)

	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1 fallback item", len(items))
	}
	wantKey := selectionIDForStudioSelection(selection)
	if items[0].TargetGroupKey != wantKey {
		t.Fatalf("target key = %q, want fallback selection key %q", items[0].TargetGroupKey, wantKey)
	}
	if items[0].GroupMode != "per_product" {
		t.Fatalf("group mode = %q, want per_product fallback", items[0].GroupMode)
	}
}

func TestMaterializeAttemptDoesNotBorrowImagesAcrossItems(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	engine := newStudioBatchGenerationService(studioBatchGenerationServiceConfig{
		repo:        repo,
		execute:     stubStudioBatchExecutionByItem(map[string]*StudioDesignResponse{"item-1": testStudioDesignResponse("design-1", "https://cdn.example.com/design-1.png"), "item-2": &StudioDesignResponse{}}),
		currentTime: func() time.Time { return time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC) },
	})
	ctx := WithTenantID(context.Background(), "tenant-a")

	seedStudioBatchGenerationGraph(t, repo, ctx, studioBatchGenerationSeed{
		batch: StudioBatchRecord{
			ID:               "batch-1",
			Status:           StudioBatchStatusGenerating,
			Prompt:           "retro summer fruit",
			GroupedImageMode: "per_product",
		},
		items: []StudioBatchItemRecord{
			{
				ID:               "item-1",
				BatchID:          "batch-1",
				TargetGroupKey:   "7001:9001:101:layer-1:101",
				TargetGroupLabel: "Canvas Tote · Red",
				GroupMode:        "per_product",
				Status:           StudioBatchItemStatusPending,
				SelectionCount:   1,
			},
			{
				ID:               "item-2",
				BatchID:          "batch-1",
				TargetGroupKey:   "7001:9001:102:layer-1:102",
				TargetGroupLabel: "Canvas Tote · Blue",
				GroupMode:        "per_product",
				Status:           StudioBatchItemStatusPending,
				SelectionCount:   1,
			},
		},
	})

	if err := engine.RunPendingStudioBatchItems(ctx, "batch-1"); err != nil {
		t.Fatalf("RunPendingStudioBatchItems() error = %v", err)
	}

	detail, err := repo.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}
	if len(detail.DesignsByItem["item-1"]) != 1 {
		t.Fatalf("designs for item-1 = %+v, want 1", detail.DesignsByItem["item-1"])
	}
	if len(detail.DesignsByItem["item-2"]) != 0 {
		t.Fatalf("designs for item-2 = %+v, want none", detail.DesignsByItem["item-2"])
	}
}

func TestRunPendingStudioBatchItemsPersistsAttemptDiagnostics(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	engine := newStudioBatchGenerationService(studioBatchGenerationServiceConfig{
		repo: repo,
		execute: func(context.Context, StudioBatchGenerateExecutionInput) (*StudioBatchGenerateExecutionOutput, error) {
			return &StudioBatchGenerateExecutionOutput{
				Response: &StudioDesignResponse{
					RequestID:     "req-batch-1",
					UpstreamJobID: "job-batch-1",
					Usage:         StudioAIUsage{TotalTokens: 456},
					RawResponse:   `{"id":"raw-image-response-1"}`,
					Images: []StudioGeneratedImage{{
						ID:       "design-1",
						ImageURL: "https://cdn.example.com/design-1.png",
					}},
				},
				ItemID:    "item-1",
				BatchID:   "batch-1",
				AttemptID: "attempt-1",
			}, nil
		},
		currentTime: func() time.Time { return time.Date(2026, 6, 1, 9, 30, 0, 0, time.UTC) },
	})
	ctx := WithTenantID(context.Background(), "tenant-a")

	seedStudioBatchGenerationGraph(t, repo, ctx, studioBatchGenerationSeed{
		batch: StudioBatchRecord{
			ID:               "batch-1",
			Status:           StudioBatchStatusGenerating,
			Prompt:           "retro summer fruit",
			GroupedImageMode: "per_product",
		},
		items: []StudioBatchItemRecord{{
			ID:               "item-1",
			BatchID:          "batch-1",
			TargetGroupKey:   "7001:9001:101:layer-1:101",
			TargetGroupLabel: "Canvas Tote · Red",
			GroupMode:        "per_product",
			Status:           StudioBatchItemStatusPending,
			SelectionCount:   1,
		}},
	})

	if err := engine.RunPendingStudioBatchItems(ctx, "batch-1"); err != nil {
		t.Fatalf("RunPendingStudioBatchItems() error = %v", err)
	}

	detail, err := repo.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}
	attempts := detail.AttemptsByItem["item-1"]
	if len(attempts) != 1 {
		t.Fatalf("attempt count = %d, want 1", len(attempts))
	}
	if attempts[0].UpstreamJobID != "job-batch-1" {
		t.Fatalf("upstream job id = %q, want job-batch-1", attempts[0].UpstreamJobID)
	}
	if !strings.Contains(attempts[0].ResultPayload, "\"request_id\":\"req-batch-1\"") {
		t.Fatalf("result payload = %q, want request id", attempts[0].ResultPayload)
	}
	if !strings.Contains(attempts[0].ResultPayload, "\"raw_response\":\"{\\\"id\\\":\\\"raw-image-response-1\\\"}\"") {
		t.Fatalf("result payload = %q, want raw response", attempts[0].ResultPayload)
	}
}

func TestStudioBatchRepositoryPersistsAsyncAttemptMetadata(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)

	seedStudioBatchGenerationGraph(t, repo, ctx, studioBatchGenerationSeed{
		batch: StudioBatchRecord{
			ID:     "batch-1",
			Status: StudioBatchStatusGenerating,
		},
		items: []StudioBatchItemRecord{{
			ID:      "item-1",
			BatchID: "batch-1",
			Status:  StudioBatchItemStatusGenerating,
		}},
		attempts: []StudioGenerationAttemptRecord{{
			ID:        "attempt-1",
			ItemID:    "item-1",
			AttemptNo: 1,
			Status:    StudioGenerationAttemptStatusRunning,
		}},
	})

	err := repo.UpdateStudioGenerationAttempt(ctx, &StudioGenerationAttemptRecord{
		ID:                    "attempt-1",
		ItemID:                "item-1",
		AttemptNo:             1,
		Status:                StudioGenerationAttemptStatusSubmitted,
		Provider:              "grsai",
		UpstreamJobID:         "job-1",
		RequestID:             "req-1",
		RequestPayload:        "{\"prompt\":\"x\"}",
		SubmitResponsePayload: "{\"id\":\"job-1\",\"status\":\"running\"}",
		ResultPayload:         "{\"status\":\"running\"}",
		ResultCheckedAt:       &now,
		QueryAttempts:         3,
		ErrorMessage:          "",
	})
	if err != nil {
		t.Fatalf("UpdateStudioGenerationAttempt() error = %v", err)
	}

	detail, err := repo.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}
	attempts := detail.AttemptsByItem["item-1"]
	if len(attempts) != 1 {
		t.Fatalf("attempt count = %d, want 1", len(attempts))
	}
	if attempts[0].Status != StudioGenerationAttemptStatusSubmitted {
		t.Fatalf("attempt status = %q, want %q", attempts[0].Status, StudioGenerationAttemptStatusSubmitted)
	}
	if attempts[0].Provider != "grsai" {
		t.Fatalf("provider = %q, want grsai", attempts[0].Provider)
	}
	if attempts[0].UpstreamJobID != "job-1" {
		t.Fatalf("upstream job id = %q, want job-1", attempts[0].UpstreamJobID)
	}
	if attempts[0].RequestID != "req-1" {
		t.Fatalf("request id = %q, want req-1", attempts[0].RequestID)
	}
	if attempts[0].SubmitResponsePayload != "{\"id\":\"job-1\",\"status\":\"running\"}" {
		t.Fatalf("submit response payload = %q, want submit response persisted", attempts[0].SubmitResponsePayload)
	}
	if attempts[0].ResultCheckedAt == nil || !attempts[0].ResultCheckedAt.Equal(now) {
		t.Fatalf("result checked at = %v, want %v", attempts[0].ResultCheckedAt, now)
	}
	if attempts[0].QueryAttempts != 3 {
		t.Fatalf("query attempts = %d, want 3", attempts[0].QueryAttempts)
	}
}

func TestRunPendingStudioBatchItemsSubmitsAsyncAttemptWithoutImmediateMaterialization(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	engine := newStudioBatchGenerationService(studioBatchGenerationServiceConfig{
		repo: repo,
		execute: func(context.Context, StudioBatchGenerateExecutionInput) (*StudioBatchGenerateExecutionOutput, error) {
			t.Fatal("execute should not be called for async-capable submission")
			return nil, nil
		},
		submitAsync: func(context.Context, StudioBatchGenerateExecutionInput) (*studioBatchAsyncSubmitOutput, error) {
			return &studioBatchAsyncSubmitOutput{Submit: &AIImageAsyncSubmit{
				JobID:             "job-async-1",
				RequestID:         "req-async-1",
				Provider:          "grsai",
				RawSubmitResponse: `{"id":"job-async-1","status":"running"}`,
				AcceptedAt:        time.Date(2026, 6, 20, 11, 0, 0, 0, time.UTC),
			}}, nil
		},
		currentTime: func() time.Time { return time.Date(2026, 6, 20, 11, 0, 5, 0, time.UTC) },
	})
	ctx := WithTenantID(context.Background(), "tenant-a")

	seedStudioBatchGenerationGraph(t, repo, ctx, studioBatchGenerationSeed{
		batch: StudioBatchRecord{
			ID:               "batch-1",
			Status:           StudioBatchStatusGenerating,
			Prompt:           "retro summer fruit",
			GroupedImageMode: "per_product",
		},
		items: []StudioBatchItemRecord{{
			ID:               "item-1",
			BatchID:          "batch-1",
			TargetGroupKey:   "7001:9001:101:layer-1:101",
			TargetGroupLabel: "Canvas Tote · Red",
			GroupMode:        "per_product",
			Status:           StudioBatchItemStatusPending,
			SelectionCount:   1,
		}},
	})

	if err := engine.RunPendingStudioBatchItems(ctx, "batch-1"); err != nil {
		t.Fatalf("RunPendingStudioBatchItems() error = %v", err)
	}

	detail, err := repo.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}
	if got := detail.Items[0].Status; got != StudioBatchItemStatusGenerating {
		t.Fatalf("item status = %q, want %q", got, StudioBatchItemStatusGenerating)
	}
	if got := len(detail.DesignsByItem["item-1"]); got != 0 {
		t.Fatalf("design count = %d, want 0 before async polling/materialization", got)
	}
	attempts := detail.AttemptsByItem["item-1"]
	if len(attempts) != 1 {
		t.Fatalf("attempt count = %d, want 1", len(attempts))
	}
	if got := attempts[0].Status; got != StudioGenerationAttemptStatusSubmitted {
		t.Fatalf("attempt status = %q, want %q", got, StudioGenerationAttemptStatusSubmitted)
	}
	if got := attempts[0].Provider; got != "grsai" {
		t.Fatalf("provider = %q, want grsai", got)
	}
	if got := attempts[0].UpstreamJobID; got != "job-async-1" {
		t.Fatalf("upstream job id = %q, want job-async-1", got)
	}
	if got := attempts[0].RequestID; got != "req-async-1" {
		t.Fatalf("request id = %q, want req-async-1", got)
	}
	if got := attempts[0].SubmitResponsePayload; got != `{"id":"job-async-1","status":"running"}` {
		t.Fatalf("submit response payload = %q, want raw submit response", got)
	}
}

func TestRunPendingStudioBatchItemsFallsBackToSyncWhenAsyncUnsupported(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	engine := newStudioBatchGenerationService(studioBatchGenerationServiceConfig{
		repo: repo,
		execute: func(context.Context, StudioBatchGenerateExecutionInput) (*StudioBatchGenerateExecutionOutput, error) {
			return &StudioBatchGenerateExecutionOutput{
				Response: testStudioDesignResponse("design-1", "https://cdn.example.com/design-1.png"),
				ItemID:   "item-1",
				BatchID:  "batch-1",
			}, nil
		},
		submitAsync: func(context.Context, StudioBatchGenerateExecutionInput) (*studioBatchAsyncSubmitOutput, error) {
			return nil, ErrAsyncImageGenerationNotSupported
		},
		currentTime: func() time.Time { return time.Date(2026, 6, 20, 11, 30, 0, 0, time.UTC) },
	})
	ctx := WithTenantID(context.Background(), "tenant-a")

	seedStudioBatchGenerationGraph(t, repo, ctx, studioBatchGenerationSeed{
		batch: StudioBatchRecord{
			ID:               "batch-1",
			Status:           StudioBatchStatusGenerating,
			Prompt:           "retro summer fruit",
			GroupedImageMode: "per_product",
		},
		items: []StudioBatchItemRecord{{
			ID:               "item-1",
			BatchID:          "batch-1",
			TargetGroupKey:   "7001:9001:101:layer-1:101",
			TargetGroupLabel: "Canvas Tote · Red",
			GroupMode:        "per_product",
			Status:           StudioBatchItemStatusPending,
			SelectionCount:   1,
		}},
	})

	if err := engine.RunPendingStudioBatchItems(ctx, "batch-1"); err != nil {
		t.Fatalf("RunPendingStudioBatchItems() error = %v", err)
	}

	detail, err := repo.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}
	if got := detail.Items[0].Status; got != StudioBatchItemStatusReviewReady {
		t.Fatalf("item status = %q, want %q", got, StudioBatchItemStatusReviewReady)
	}
	if got := len(detail.DesignsByItem["item-1"]); got != 1 {
		t.Fatalf("design count = %d, want 1 after sync fallback", got)
	}
	if got := detail.AttemptsByItem["item-1"][0].Status; got != StudioGenerationAttemptStatusMaterialized {
		t.Fatalf("attempt status = %q, want %q", got, StudioGenerationAttemptStatusMaterialized)
	}
}

func TestRunPendingStudioBatchItemsMaterializesDirectSubmitResult(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	engine := newStudioBatchGenerationService(studioBatchGenerationServiceConfig{
		repo: repo,
		execute: func(context.Context, StudioBatchGenerateExecutionInput) (*StudioBatchGenerateExecutionOutput, error) {
			t.Fatal("execute should not be called when submit already returned final result")
			return nil, nil
		},
		submitAsync: func(context.Context, StudioBatchGenerateExecutionInput) (*studioBatchAsyncSubmitOutput, error) {
			response := testStudioDesignResponse("design-1", "https://cdn.example.com/design-1.png")
			payload, _ := json.Marshal(response)
			return &studioBatchAsyncSubmitOutput{
				Submit: &AIImageAsyncSubmit{
					JobID:             "job-direct-1",
					RequestID:         "req-direct-1",
					Provider:          "grsai",
					Status:            AIImageAsyncResultSucceeded,
					RawSubmitResponse: `{"id":"job-direct-1","status":"succeeded"}`,
				},
				Response:      response,
				ResultPayload: string(payload),
			}, nil
		},
		currentTime: func() time.Time { return time.Date(2026, 6, 20, 11, 15, 0, 0, time.UTC) },
	})
	ctx := WithTenantID(context.Background(), "tenant-a")

	seedStudioBatchGenerationGraph(t, repo, ctx, studioBatchGenerationSeed{
		batch: StudioBatchRecord{
			ID:               "batch-1",
			Status:           StudioBatchStatusGenerating,
			Prompt:           "retro summer fruit",
			GroupedImageMode: "per_product",
		},
		items: []StudioBatchItemRecord{{
			ID:               "item-1",
			BatchID:          "batch-1",
			TargetGroupKey:   "7001:9001:101:layer-1:101",
			TargetGroupLabel: "Canvas Tote · Red",
			GroupMode:        "per_product",
			Status:           StudioBatchItemStatusPending,
			SelectionCount:   1,
		}},
	})

	if err := engine.RunPendingStudioBatchItems(ctx, "batch-1"); err != nil {
		t.Fatalf("RunPendingStudioBatchItems() error = %v", err)
	}

	detail, err := repo.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}
	if got := detail.Items[0].Status; got != StudioBatchItemStatusReviewReady {
		t.Fatalf("item status = %q, want %q", got, StudioBatchItemStatusReviewReady)
	}
	if got := len(detail.DesignsByItem["item-1"]); got != 1 {
		t.Fatalf("design count = %d, want 1", got)
	}
	attempts := detail.AttemptsByItem["item-1"]
	if len(attempts) != 1 {
		t.Fatalf("attempt count = %d, want 1", len(attempts))
	}
	if got := attempts[0].Status; got != StudioGenerationAttemptStatusMaterialized {
		t.Fatalf("attempt status = %q, want %q", got, StudioGenerationAttemptStatusMaterialized)
	}
	if got := attempts[0].UpstreamJobID; got != "job-direct-1" {
		t.Fatalf("upstream job id = %q, want job-direct-1", got)
	}
}

func TestRecoverAwaitingMaterializationReusesAttemptResult(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	engine := newStudioBatchGenerationService(studioBatchGenerationServiceConfig{
		repo:        repo,
		currentTime: func() time.Time { return time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC) },
	})
	ctx := WithTenantID(context.Background(), "tenant-a")

	resultPayload, err := json.Marshal(testStudioDesignResponse("design-1", "https://cdn.example.com/design-1.png"))
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	seedStudioBatchGenerationGraph(t, repo, ctx, studioBatchGenerationSeed{
		batch: StudioBatchRecord{
			ID:               "batch-1",
			Status:           StudioBatchStatusGenerating,
			Prompt:           "retro summer fruit",
			GroupedImageMode: "per_product",
		},
		items: []StudioBatchItemRecord{
			{
				ID:               "item-1",
				BatchID:          "batch-1",
				TargetGroupKey:   "7001:9001:101:layer-1:101",
				TargetGroupLabel: "Canvas Tote · Red",
				GroupMode:        "per_product",
				Status:           StudioBatchItemStatusAwaitingMaterialization,
				SelectionCount:   1,
			},
		},
		attempts: []StudioGenerationAttemptRecord{
			{
				ID:            "attempt-1",
				ItemID:        "item-1",
				AttemptNo:     1,
				Status:        StudioGenerationAttemptStatusSucceeded,
				ResultPayload: string(resultPayload),
			},
		},
	})

	if err := engine.RecoverStudioBatchMaterialization(ctx, "batch-1"); err != nil {
		t.Fatalf("RecoverStudioBatchMaterialization() error = %v", err)
	}

	detail, err := repo.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}
	if detail.Items[0].Status != StudioBatchItemStatusReviewReady {
		t.Fatalf("item status = %q, want %q", detail.Items[0].Status, StudioBatchItemStatusReviewReady)
	}
	if len(detail.DesignsByItem["item-1"]) != 1 {
		t.Fatalf("designs for item-1 = %+v, want 1", detail.DesignsByItem["item-1"])
	}
	if detail.DesignsByItem["item-1"][0].SourceAttemptID != "attempt-1" {
		t.Fatalf("design source attempt = %q, want attempt-1", detail.DesignsByItem["item-1"][0].SourceAttemptID)
	}
	if detail.DesignsByItem["item-1"][0].ReviewStatus != StudioMaterializedDesignReviewStatusApproved {
		t.Fatalf(
			"design review status = %q, want %q",
			detail.DesignsByItem["item-1"][0].ReviewStatus,
			StudioMaterializedDesignReviewStatusApproved,
		)
	}
}

func TestRecoverGeneratingAsyncAttemptPollsProviderAndMaterializesOnSuccess(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	engine := newStudioBatchGenerationService(studioBatchGenerationServiceConfig{
		repo: repo,
		queryAsync: func(context.Context, StudioBatchGenerateExecutionInput, string) (*studioBatchAsyncQueryOutput, error) {
			return &studioBatchAsyncQueryOutput{
				Result: &AIImageAsyncResult{
					JobID:             "job-1",
					RequestID:         "req-1",
					Provider:          "grsai",
					Status:            AIImageAsyncResultSucceeded,
					RawResultResponse: `{"id":"job-1","status":"succeeded"}`,
				},
				Response:      testStudioDesignResponse("design-1", "https://cdn.example.com/design-1.png"),
				ResultPayload: `{"images":[{"id":"design-1","image_url":"https://cdn.example.com/design-1.png"}]}`,
			}, nil
		},
		currentTime: func() time.Time { return time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC) },
	})
	ctx := WithTenantID(context.Background(), "tenant-a")

	seedStudioBatchGenerationGraph(t, repo, ctx, studioBatchGenerationSeed{
		batch: StudioBatchRecord{
			ID:               "batch-1",
			Status:           StudioBatchStatusGenerating,
			Prompt:           "retro summer fruit",
			GroupedImageMode: "per_product",
		},
		items: []StudioBatchItemRecord{{
			ID:               "item-1",
			BatchID:          "batch-1",
			TargetGroupKey:   "7001:9001:101:layer-1:101",
			TargetGroupLabel: "Canvas Tote · Red",
			GroupMode:        "per_product",
			Status:           StudioBatchItemStatusGenerating,
			SelectionCount:   1,
		}},
		attempts: []StudioGenerationAttemptRecord{{
			ID:            "attempt-1",
			ItemID:        "item-1",
			AttemptNo:     1,
			Status:        StudioGenerationAttemptStatusSubmitted,
			Provider:      "grsai",
			UpstreamJobID: "job-1",
		}},
	})

	if err := engine.RecoverStudioBatchMaterialization(ctx, "batch-1"); err != nil {
		t.Fatalf("RecoverStudioBatchMaterialization() error = %v", err)
	}

	detail, err := repo.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}
	if got := detail.Items[0].Status; got != StudioBatchItemStatusReviewReady {
		t.Fatalf("item status = %q, want %q", got, StudioBatchItemStatusReviewReady)
	}
	if got := len(detail.DesignsByItem["item-1"]); got != 1 {
		t.Fatalf("design count = %d, want 1", got)
	}
	if got := detail.AttemptsByItem["item-1"][0].Status; got != StudioGenerationAttemptStatusMaterialized {
		t.Fatalf("attempt status = %q, want %q", got, StudioGenerationAttemptStatusMaterialized)
	}
	if got := detail.AttemptsByItem["item-1"][0].QueryAttempts; got != 1 {
		t.Fatalf("query attempts = %d, want 1", got)
	}
}

func TestRecoverSubmittedAttemptWithoutJobIDFailsCleanly(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	engine := newStudioBatchGenerationService(studioBatchGenerationServiceConfig{
		repo:        repo,
		currentTime: func() time.Time { return time.Date(2026, 6, 20, 12, 10, 0, 0, time.UTC) },
	})
	ctx := WithTenantID(context.Background(), "tenant-a")

	seedStudioBatchGenerationGraph(t, repo, ctx, studioBatchGenerationSeed{
		batch: StudioBatchRecord{
			ID:               "batch-1",
			Status:           StudioBatchStatusGenerating,
			Prompt:           "retro summer fruit",
			GroupedImageMode: "per_product",
		},
		items: []StudioBatchItemRecord{{
			ID:               "item-1",
			BatchID:          "batch-1",
			TargetGroupKey:   "7001:9001:101:layer-1:101",
			TargetGroupLabel: "Canvas Tote · Red",
			GroupMode:        "per_product",
			Status:           StudioBatchItemStatusGenerating,
			SelectionCount:   1,
		}},
		attempts: []StudioGenerationAttemptRecord{{
			ID:        "attempt-1",
			ItemID:    "item-1",
			AttemptNo: 1,
			Status:    StudioGenerationAttemptStatusSubmitted,
			Provider:  "grsai",
		}},
	})

	if err := engine.RecoverStudioBatchMaterialization(ctx, "batch-1"); err != nil {
		t.Fatalf("RecoverStudioBatchMaterialization() error = %v", err)
	}

	detail, err := repo.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}
	if got := detail.Items[0].Status; got != StudioBatchItemStatusFailed {
		t.Fatalf("item status = %q, want %q", got, StudioBatchItemStatusFailed)
	}
	if got := detail.Items[0].LastError; got != "async generation attempt missing upstream job id" {
		t.Fatalf("item last error = %q, want missing job id diagnostic", got)
	}
	if got := detail.AttemptsByItem["item-1"][0].Status; got != StudioGenerationAttemptStatusFailed {
		t.Fatalf("attempt status = %q, want %q", got, StudioGenerationAttemptStatusFailed)
	}
}

func TestStartStudioBatchGenerationRerunRefreshesLatestSessionDraftInput(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	sessionRepo := &studioBatchGenerationSessionRepoStub{
		session: &SheinStudioSession{
			ID:               "batch-1",
			SavedAsBatch:     true,
			Status:           SheinStudioSessionStatusSelecting,
			Prompt:           "old prompt",
			StyleCount:       "1",
			ArtworkModel:     "gpt-image-1",
			GroupedImageMode: "per_product",
			Selection:        SheinStudioSelectionSnapshot(testStudioBatchSelection(101, "Canvas Tote", "Red", 1200, 1200)),
		},
	}
	var prompts []string
	service := newTaskStudioBatchService(taskStudioBatchServiceConfig{
		repo:              repo,
		studioSessionRepo: sessionRepo,
		generator: newStudioBatchGenerationService(studioBatchGenerationServiceConfig{
			repo: repo,
			execute: func(ctx context.Context, input StudioBatchGenerateExecutionInput) (*StudioBatchGenerateExecutionOutput, error) {
				prompts = append(prompts, input.Request.Prompt)
				return &StudioBatchGenerateExecutionOutput{
					Response: testStudioDesignResponse("design-"+input.ItemID, "https://cdn.example.com/"+input.ItemID+".png"),
					ItemID:   input.ItemID,
					BatchID:  input.BatchID,
				}, nil
			},
			currentTime: func() time.Time { return time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC) },
		}),
	})
	ctx := WithTenantID(context.Background(), "tenant-a")

	if _, err := service.StartStudioBatchGeneration(ctx, "batch-1"); err != nil {
		t.Fatalf("first StartStudioBatchGeneration() error = %v", err)
	}

	sessionRepo.session.Prompt = "new prompt"
	sessionRepo.session.GroupedImageMode = "shared_by_size"
	sessionRepo.session.GroupedSelections = SheinStudioGroupedSelectionList{
		{
			SelectionID: "7001:9001:102:layer-1:102",
			Selection:   testStudioBatchSelection(102, "Canvas Tote", "Blue", 1200, 1200),
			Eligible:    true,
		},
	}

	detail, err := service.StartStudioBatchGeneration(ctx, "batch-1")
	if err != nil {
		t.Fatalf("second StartStudioBatchGeneration() error = %v", err)
	}

	if detail.Batch == nil || detail.Batch.Prompt != "new prompt" {
		t.Fatalf("detail.Batch = %+v, want refreshed prompt", detail.Batch)
	}
	if detail.Batch.GroupedImageMode != "shared_by_size" {
		t.Fatalf("detail.Batch.GroupedImageMode = %q, want shared_by_size", detail.Batch.GroupedImageMode)
	}
	if len(detail.Items) != 1 {
		t.Fatalf("len(detail.Items) = %d, want 1 shared-by-size item after rerun", len(detail.Items))
	}
	wantSharedKey := buildStudioBatchSharedCompatibilityGroupKey(testStudioBatchSelection(101, "Canvas Tote", "Red", 1200, 1200))
	if detail.Items[0].Item.TargetGroupKey != wantSharedKey {
		t.Fatalf("item target group key = %q, want compatibility key %q", detail.Items[0].Item.TargetGroupKey, wantSharedKey)
	}
	if got := prompts[len(prompts)-1]; got != "new prompt" {
		t.Fatalf("latest execution prompt = %q, want refreshed prompt", got)
	}
}

func TestResumeStudioBatchGenerationPreservesExistingAttemptsAndDesigns(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	sessionRepo := &studioBatchGenerationSessionRepoStub{
		session: &SheinStudioSession{
			ID:               "batch-1",
			SavedAsBatch:     true,
			Status:           SheinStudioSessionStatusSelecting,
			Prompt:           "new prompt that should not overwrite resume",
			StyleCount:       "1",
			ArtworkModel:     "gpt-image-1",
			GroupedImageMode: "shared_by_size",
			Selection:        SheinStudioSelectionSnapshot(testStudioBatchSelection(101, "Canvas Tote", "Red", 1200, 1200)),
		},
	}
	service := newTaskStudioBatchService(taskStudioBatchServiceConfig{
		repo:              repo,
		studioSessionRepo: sessionRepo,
		generator: newStudioBatchGenerationService(studioBatchGenerationServiceConfig{
			repo:        repo,
			execute:     stubStudioBatchExecutionByItem(map[string]*StudioDesignResponse{}),
			currentTime: func() time.Time { return time.Date(2026, 6, 1, 8, 30, 0, 0, time.UTC) },
		}),
	})
	ctx := WithTenantID(context.Background(), "tenant-a")

	resultPayload, err := json.Marshal(testStudioDesignResponse("design-1", "https://cdn.example.com/design-1.png"))
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
			ImageURL:         "https://cdn.example.com/design-1.png",
		}},
	})

	detail, err := service.ResumeStudioBatchGeneration(ctx, "batch-1")
	if err != nil {
		t.Fatalf("ResumeStudioBatchGeneration() error = %v", err)
	}

	if detail.Batch == nil || detail.Batch.Prompt != "persisted prompt" {
		t.Fatalf("detail.Batch = %+v, want existing persisted batch to remain intact", detail.Batch)
	}
	if len(detail.Items) != 1 {
		t.Fatalf("len(detail.Items) = %d, want 1 existing item", len(detail.Items))
	}
	if len(detail.Items[0].Attempts) != 1 {
		t.Fatalf("len(detail.Items[0].Attempts) = %d, want preserved attempt", len(detail.Items[0].Attempts))
	}
	if len(detail.Items[0].Designs) != 1 {
		t.Fatalf("len(detail.Items[0].Designs) = %d, want preserved design", len(detail.Items[0].Designs))
	}
	if detail.Items[0].Designs[0].SourceAttemptID != "attempt-1" {
		t.Fatalf("design source attempt = %q, want attempt-1", detail.Items[0].Designs[0].SourceAttemptID)
	}
}

func TestResumeStudioBatchGenerationLeavesFreshRunningAttemptUntouched(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	now := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	service := newTaskStudioBatchService(taskStudioBatchServiceConfig{
		repo: repo,
		generator: newStudioBatchGenerationService(studioBatchGenerationServiceConfig{
			repo:        repo,
			execute:     stubStudioBatchExecutionByItem(map[string]*StudioDesignResponse{}),
			currentTime: func() time.Time { return now },
		}),
	})
	ctx := WithTenantID(context.Background(), "tenant-a")

	seedStudioBatchGenerationGraph(t, repo, ctx, studioBatchGenerationSeed{
		batch: StudioBatchRecord{
			ID:               "batch-1",
			Status:           StudioBatchStatusGenerating,
			Prompt:           "persisted prompt",
			GroupedImageMode: "per_product",
		},
		items: []StudioBatchItemRecord{{
			ID:               "item-1",
			BatchID:          "batch-1",
			TargetGroupKey:   "7001:9001:101:layer-1:101",
			TargetGroupLabel: "Canvas Tote · Red",
			GroupMode:        "per_product",
			Status:           StudioBatchItemStatusGenerating,
			SelectionCount:   1,
			CreatedAt:        now.Add(-2 * time.Minute),
			UpdatedAt:        now.Add(-2 * time.Minute),
		}},
		attempts: []StudioGenerationAttemptRecord{{
			ID:        "attempt-1",
			ItemID:    "item-1",
			AttemptNo: 1,
			Status:    StudioGenerationAttemptStatusRunning,
			StartedAt: timePtr(now.Add(-2 * time.Minute)),
			CreatedAt: now.Add(-2 * time.Minute),
			UpdatedAt: now.Add(-2 * time.Minute),
		}},
	})

	detail, err := service.ResumeStudioBatchGeneration(ctx, "batch-1")
	if err != nil {
		t.Fatalf("ResumeStudioBatchGeneration() error = %v", err)
	}

	if got := detail.Items[0].Item.Status; got != StudioBatchItemStatusGenerating {
		t.Fatalf("item status = %q, want %q", got, StudioBatchItemStatusGenerating)
	}
	if got := detail.Items[0].Attempts[0].Status; got != StudioGenerationAttemptStatusRunning {
		t.Fatalf("attempt status = %q, want %q", got, StudioGenerationAttemptStatusRunning)
	}
}

func TestResumeStudioBatchGenerationRetriesStaleRunningAttemptAndContinuesPendingItems(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	now := time.Date(2026, 6, 1, 9, 30, 0, 0, time.UTC)
	var executed []string
	service := newTaskStudioBatchService(taskStudioBatchServiceConfig{
		repo: repo,
		generator: newStudioBatchGenerationService(studioBatchGenerationServiceConfig{
			repo: repo,
			execute: func(_ context.Context, input StudioBatchGenerateExecutionInput) (*StudioBatchGenerateExecutionOutput, error) {
				executed = append(executed, input.ItemID)
				return &StudioBatchGenerateExecutionOutput{
					BatchID: input.BatchID,
					ItemID:  input.ItemID,
					Response: &StudioDesignResponse{
						Images: []StudioGeneratedImage{{
							ID:       "design-" + input.ItemID,
							ImageURL: "https://cdn.example.com/" + input.ItemID + ".png",
						}},
					},
				}, nil
			},
			currentTime: func() time.Time { return now },
		}),
	})
	ctx := WithTenantID(context.Background(), "tenant-a")

	seedStudioBatchGenerationGraph(t, repo, ctx, studioBatchGenerationSeed{
		batch: StudioBatchRecord{
			ID:               "batch-1",
			Status:           StudioBatchStatusGenerating,
			Prompt:           "persisted prompt",
			GroupedImageMode: "per_product",
		},
		items: []StudioBatchItemRecord{
			{
				ID:               "item-1",
				BatchID:          "batch-1",
				TargetGroupKey:   "7001:9001:101:layer-1:101",
				TargetGroupLabel: "Canvas Tote · Red",
				GroupMode:        "per_product",
				Status:           StudioBatchItemStatusGenerating,
				SelectionCount:   1,
				CreatedAt:        now.Add(-20 * time.Minute),
				UpdatedAt:        now.Add(-20 * time.Minute),
			},
			{
				ID:               "item-2",
				BatchID:          "batch-1",
				TargetGroupKey:   "7001:9001:102:layer-1:102",
				TargetGroupLabel: "Canvas Tote · Blue",
				GroupMode:        "per_product",
				Status:           StudioBatchItemStatusPending,
				SelectionCount:   1,
				CreatedAt:        now.Add(-20 * time.Minute),
				UpdatedAt:        now.Add(-20 * time.Minute),
			},
		},
		attempts: []StudioGenerationAttemptRecord{{
			ID:        "attempt-1",
			ItemID:    "item-1",
			AttemptNo: 1,
			Status:    StudioGenerationAttemptStatusRunning,
			StartedAt: timePtr(now.Add(-20 * time.Minute)),
			CreatedAt: now.Add(-20 * time.Minute),
			UpdatedAt: now.Add(-20 * time.Minute),
		}},
	})

	detail, err := service.ResumeStudioBatchGeneration(ctx, "batch-1")
	if err != nil {
		t.Fatalf("ResumeStudioBatchGeneration() error = %v", err)
	}

	if len(detail.Items) != 2 {
		t.Fatalf("len(detail.Items) = %d, want 2", len(detail.Items))
	}
	if got := detail.Items[0].Item.Status; got != StudioBatchItemStatusReviewReady {
		t.Fatalf("item-1 status = %q, want %q", got, StudioBatchItemStatusReviewReady)
	}
	if got := detail.Items[0].Attempts[0].Status; got != StudioGenerationAttemptStatusFailed {
		t.Fatalf("item-1 attempt status = %q, want %q", got, StudioGenerationAttemptStatusFailed)
	}
	if got := len(detail.Items[0].Attempts); got != 2 {
		t.Fatalf("item-1 attempt count = %d, want 2", got)
	}
	if got := detail.Items[0].Attempts[1].Status; got != StudioGenerationAttemptStatusMaterialized {
		t.Fatalf("item-1 retry attempt status = %q, want %q", got, StudioGenerationAttemptStatusMaterialized)
	}
	if len(detail.Items[0].Designs) != 1 || detail.Items[0].Designs[0].ID != "design-item-1" {
		t.Fatalf("item-1 designs = %+v, want retried materialized design", detail.Items[0].Designs)
	}
	if got := detail.Items[1].Item.Status; got != StudioBatchItemStatusReviewReady {
		t.Fatalf("item-2 status = %q, want %q", got, StudioBatchItemStatusReviewReady)
	}
	if len(detail.Items[1].Designs) != 1 || detail.Items[1].Designs[0].ID != "design-item-2" {
		t.Fatalf("item-2 designs = %+v, want resumed materialized design", detail.Items[1].Designs)
	}
	if got := strings.Join(executed, ","); got != "item-1,item-2" {
		t.Fatalf("executed items = %q, want item-1 retry before item-2 pending", got)
	}
}

func TestRunPendingStudioBatchItemsClaimsPendingItemBeforeAttemptCreation(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	var executions int
	engine := newStudioBatchGenerationService(studioBatchGenerationServiceConfig{
		repo: repo,
		execute: func(ctx context.Context, input StudioBatchGenerateExecutionInput) (*StudioBatchGenerateExecutionOutput, error) {
			executions++
			return &StudioBatchGenerateExecutionOutput{
				Response: testStudioDesignResponse("design-1", "https://cdn.example.com/design-1.png"),
				ItemID:   input.ItemID,
				BatchID:  input.BatchID,
			}, nil
		},
		currentTime: func() time.Time { return time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC) },
	})
	ctx := WithTenantID(context.Background(), "tenant-a")

	seedStudioBatchGenerationGraph(t, repo, ctx, studioBatchGenerationSeed{
		batch: StudioBatchRecord{
			ID:               "batch-1",
			Status:           StudioBatchStatusGenerating,
			Prompt:           "retro summer fruit",
			GroupedImageMode: "per_product",
		},
		items: []StudioBatchItemRecord{{
			ID:               "item-1",
			BatchID:          "batch-1",
			TargetGroupKey:   "7001:9001:101:layer-1:101",
			TargetGroupLabel: "Canvas Tote · Red",
			GroupMode:        "per_product",
			Status:           StudioBatchItemStatusPending,
			SelectionCount:   1,
		}},
	})

	item, claimed, err := repo.ClaimStudioBatchItem(ctx, "item-1", StudioBatchItemStatusPending, StudioBatchItemStatusGenerating, time.Date(2026, 6, 1, 8, 59, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("ClaimStudioBatchItem() error = %v", err)
	}
	if !claimed || item == nil {
		t.Fatal("expected initial claim to succeed")
	}

	if err := engine.RunPendingStudioBatchItems(ctx, "batch-1"); err != nil {
		t.Fatalf("RunPendingStudioBatchItems() error = %v", err)
	}
	if executions != 0 {
		t.Fatalf("executions = %d, want 0 after external claim", executions)
	}
}

func TestRunPendingStudioBatchItemsRejectsPromptlessBatchBeforeAttemptCreation(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	var executions int
	engine := newStudioBatchGenerationService(studioBatchGenerationServiceConfig{
		repo: repo,
		execute: func(ctx context.Context, input StudioBatchGenerateExecutionInput) (*StudioBatchGenerateExecutionOutput, error) {
			executions++
			return &StudioBatchGenerateExecutionOutput{
				Response: testStudioDesignResponse("design-1", "https://cdn.example.com/design-1.png"),
				ItemID:   input.ItemID,
				BatchID:  input.BatchID,
			}, nil
		},
		currentTime: func() time.Time { return time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC) },
	})
	ctx := WithTenantID(context.Background(), "tenant-a")

	seedStudioBatchGenerationGraph(t, repo, ctx, studioBatchGenerationSeed{
		batch: StudioBatchRecord{
			ID:               "batch-1",
			Status:           StudioBatchStatusGenerating,
			Prompt:           " ",
			GroupedImageMode: "per_product",
		},
		items: []StudioBatchItemRecord{{
			ID:               "item-1",
			BatchID:          "batch-1",
			TargetGroupKey:   "7001:9001:101:layer-1:101",
			TargetGroupLabel: "Canvas Tote · Red",
			GroupMode:        "per_product",
			Status:           StudioBatchItemStatusPending,
			SelectionCount:   1,
		}},
	})

	err := engine.RunPendingStudioBatchItems(ctx, "batch-1")
	if err == nil {
		t.Fatal("RunPendingStudioBatchItems() error = nil, want prompt validation error")
	}
	if !strings.Contains(err.Error(), "prompt is required") {
		t.Fatalf("RunPendingStudioBatchItems() error = %v, want prompt validation error", err)
	}
	if executions != 0 {
		t.Fatalf("executions = %d, want no generation attempt", executions)
	}
	detail, getErr := repo.GetStudioBatchDetail(ctx, "batch-1")
	if getErr != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", getErr)
	}
	if got := len(detail.AttemptsByItem["item-1"]); got != 0 {
		t.Fatalf("attempt count = %d, want no attempt created", got)
	}
	if got := detail.Items[0].Status; got != StudioBatchItemStatusPending {
		t.Fatalf("item status = %q, want pending", got)
	}
}

func TestRunPendingStudioBatchItemsAutoRetriesTransientFailuresUntilSuccess(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	executions := 0
	engine := newStudioBatchGenerationService(studioBatchGenerationServiceConfig{
		repo: repo,
		execute: func(ctx context.Context, input StudioBatchGenerateExecutionInput) (*StudioBatchGenerateExecutionOutput, error) {
			executions++
			if executions < 3 {
				return nil, errors.New(`generate studio design 1: 调用 OpenAI image API 失败，已重试3次: image api returned status 400: {"error":{"message":"excessive system load"}}`)
			}
			return &StudioBatchGenerateExecutionOutput{
				Response: testStudioDesignResponse("design-1", "https://cdn.example.com/design-1.png"),
				ItemID:   input.ItemID,
				BatchID:  input.BatchID,
			}, nil
		},
		currentTime: func() time.Time { return time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC) },
	})
	ctx := WithTenantID(context.Background(), "tenant-a")

	seedStudioBatchGenerationGraph(t, repo, ctx, studioBatchGenerationSeed{
		batch: StudioBatchRecord{
			ID:               "batch-1",
			Status:           StudioBatchStatusGenerating,
			Prompt:           "retro summer fruit",
			GroupedImageMode: "per_product",
		},
		items: []StudioBatchItemRecord{{
			ID:               "item-1",
			BatchID:          "batch-1",
			TargetGroupKey:   "7001:9001:101:layer-1:101",
			TargetGroupLabel: "Canvas Tote · Red",
			GroupMode:        "per_product",
			Status:           StudioBatchItemStatusPending,
			SelectionCount:   1,
		}},
	})

	if err := engine.RunPendingStudioBatchItems(ctx, "batch-1"); err != nil {
		t.Fatalf("RunPendingStudioBatchItems() error = %v", err)
	}

	detail, err := repo.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}
	if executions != 3 {
		t.Fatalf("executions = %d, want 3 attempts including transient retries", executions)
	}
	if detail.Items[0].Status != StudioBatchItemStatusReviewReady {
		t.Fatalf("item status = %q, want review_ready after transient retry recovery", detail.Items[0].Status)
	}
	if got := len(detail.AttemptsByItem["item-1"]); got != 3 {
		t.Fatalf("attempts = %+v, want 3 persisted attempts", detail.AttemptsByItem["item-1"])
	}
	if detail.AttemptsByItem["item-1"][0].Status != StudioGenerationAttemptStatusFailed || detail.AttemptsByItem["item-1"][1].Status != StudioGenerationAttemptStatusFailed || detail.AttemptsByItem["item-1"][2].Status != StudioGenerationAttemptStatusMaterialized {
		t.Fatalf("attempt statuses = %+v, want failed, failed, materialized", detail.AttemptsByItem["item-1"])
	}
	if len(detail.DesignsByItem["item-1"]) != 1 || detail.DesignsByItem["item-1"][0].ID != "design-1" {
		t.Fatalf("designs = %+v, want successful materialized design after retries", detail.DesignsByItem["item-1"])
	}
}

func TestRunPendingStudioBatchItemsStopsRetryingTransientFailuresAtAttemptLimit(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	executions := 0
	engine := newStudioBatchGenerationService(studioBatchGenerationServiceConfig{
		repo: repo,
		execute: func(ctx context.Context, input StudioBatchGenerateExecutionInput) (*StudioBatchGenerateExecutionOutput, error) {
			executions++
			return nil, errors.New(`generate studio design 1: 调用 OpenAI image API 失败，已重试3次: image api returned status 400: {"error":{"message":"excessive system load"}}`)
		},
		currentTime: func() time.Time { return time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC) },
	})
	ctx := WithTenantID(context.Background(), "tenant-a")

	seedStudioBatchGenerationGraph(t, repo, ctx, studioBatchGenerationSeed{
		batch: StudioBatchRecord{
			ID:               "batch-1",
			Status:           StudioBatchStatusGenerating,
			Prompt:           "retro summer fruit",
			GroupedImageMode: "per_product",
		},
		items: []StudioBatchItemRecord{{
			ID:               "item-1",
			BatchID:          "batch-1",
			TargetGroupKey:   "7001:9001:101:layer-1:101",
			TargetGroupLabel: "Canvas Tote · Red",
			GroupMode:        "per_product",
			Status:           StudioBatchItemStatusPending,
			SelectionCount:   1,
		}},
	})

	if err := engine.RunPendingStudioBatchItems(ctx, "batch-1"); err != nil {
		t.Fatalf("RunPendingStudioBatchItems() error = %v", err)
	}

	detail, err := repo.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}
	if executions != defaultStudioBatchTransientRetryLimit {
		t.Fatalf("executions = %d, want retry limit %d", executions, defaultStudioBatchTransientRetryLimit)
	}
	if detail.Items[0].Status != StudioBatchItemStatusFailed {
		t.Fatalf("item status = %q, want failed after retry limit", detail.Items[0].Status)
	}
	if detail.Items[0].LastError == "" {
		t.Fatal("expected final failed item to keep retry exhaustion error")
	}
	if got := len(detail.AttemptsByItem["item-1"]); got != defaultStudioBatchTransientRetryLimit {
		t.Fatalf("attempt count = %d, want %d", got, defaultStudioBatchTransientRetryLimit)
	}
	if len(detail.DesignsByItem["item-1"]) != 0 {
		t.Fatalf("designs = %+v, want none after exhausted retries", detail.DesignsByItem["item-1"])
	}
}

func TestRecoverStudioBatchMaterializationMarksMissingResultPayloadFailed(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	engine := newStudioBatchGenerationService(studioBatchGenerationServiceConfig{
		repo:        repo,
		currentTime: func() time.Time { return time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC) },
	})
	ctx := WithTenantID(context.Background(), "tenant-a")

	seedStudioBatchGenerationGraph(t, repo, ctx, studioBatchGenerationSeed{
		batch: StudioBatchRecord{
			ID:               "batch-1",
			Status:           StudioBatchStatusGenerating,
			Prompt:           "retro summer fruit",
			GroupedImageMode: "per_product",
		},
		items: []StudioBatchItemRecord{{
			ID:               "item-1",
			BatchID:          "batch-1",
			TargetGroupKey:   "7001:9001:101:layer-1:101",
			TargetGroupLabel: "Canvas Tote · Red",
			GroupMode:        "per_product",
			Status:           StudioBatchItemStatusAwaitingMaterialization,
			SelectionCount:   1,
		}},
		attempts: []StudioGenerationAttemptRecord{{
			ID:        "attempt-1",
			ItemID:    "item-1",
			AttemptNo: 1,
			Status:    StudioGenerationAttemptStatusSucceeded,
		}},
	})

	if err := engine.RecoverStudioBatchMaterialization(ctx, "batch-1"); err != nil {
		t.Fatalf("RecoverStudioBatchMaterialization() error = %v", err)
	}

	detail, err := repo.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}
	if detail.Items[0].Status != StudioBatchItemStatusFailed {
		t.Fatalf("item status = %q, want failed", detail.Items[0].Status)
	}
	if detail.Items[0].LastError == "" {
		t.Fatal("expected failed item to keep explicit recovery error")
	}
}

func TestRecoverStudioBatchMaterializationMarksStrandedGeneratingItemFailedAtRetryLimit(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	engine := newStudioBatchGenerationService(studioBatchGenerationServiceConfig{
		repo:        repo,
		currentTime: func() time.Time { return time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC) },
	})
	ctx := WithTenantID(context.Background(), "tenant-a")

	seedStudioBatchGenerationGraph(t, repo, ctx, studioBatchGenerationSeed{
		batch: StudioBatchRecord{
			ID:               "batch-1",
			Status:           StudioBatchStatusGenerating,
			Prompt:           "retro summer fruit",
			GroupedImageMode: "per_product",
		},
		items: []StudioBatchItemRecord{{
			ID:               "item-1",
			BatchID:          "batch-1",
			TargetGroupKey:   "7001:9001:101:layer-1:101",
			TargetGroupLabel: "Canvas Tote · Red",
			GroupMode:        "per_product",
			Status:           StudioBatchItemStatusGenerating,
			SelectionCount:   1,
		}},
		attempts: []StudioGenerationAttemptRecord{{
			ID:        "attempt-1",
			ItemID:    "item-1",
			AttemptNo: defaultStudioBatchStaleRecoveryLimit,
			Status:    StudioGenerationAttemptStatusRunning,
		}},
	})

	if err := engine.RecoverStudioBatchMaterialization(ctx, "batch-1"); err != nil {
		t.Fatalf("RecoverStudioBatchMaterialization() error = %v", err)
	}

	detail, err := repo.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}
	if detail.Items[0].Status != StudioBatchItemStatusFailed {
		t.Fatalf("item status = %q, want failed", detail.Items[0].Status)
	}
	if detail.AttemptsByItem["item-1"][0].Status != StudioGenerationAttemptStatusFailed {
		t.Fatalf("attempt status = %q, want failed", detail.AttemptsByItem["item-1"][0].Status)
	}
}

func TestRecoverStudioBatchMaterializationRequeuesRetryableFailedItem(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	now := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	engine := newStudioBatchGenerationService(studioBatchGenerationServiceConfig{
		repo:        repo,
		currentTime: func() time.Time { return now },
	})
	ctx := WithTenantID(context.Background(), "tenant-a")

	seedStudioBatchGenerationGraph(t, repo, ctx, studioBatchGenerationSeed{
		batch: StudioBatchRecord{
			ID:               "batch-1",
			Status:           StudioBatchStatusPartiallyFailed,
			Prompt:           "retro summer fruit",
			GroupedImageMode: "per_product",
		},
		items: []StudioBatchItemRecord{{
			ID:               "item-1",
			BatchID:          "batch-1",
			TargetGroupKey:   "7001:9001:101:layer-1:101",
			TargetGroupLabel: "Canvas Tote · Red",
			GroupMode:        "per_product",
			Status:           StudioBatchItemStatusFailed,
			LastError:        "generation attempt timed out before result persisted",
			SelectionCount:   1,
			CreatedAt:        now.Add(-20 * time.Minute),
			UpdatedAt:        now.Add(-20 * time.Minute),
		}},
		attempts: []StudioGenerationAttemptRecord{{
			ID:           "attempt-1",
			ItemID:       "item-1",
			AttemptNo:    defaultStudioBatchTransientRetryLimit,
			Status:       StudioGenerationAttemptStatusFailed,
			ErrorMessage: "generation attempt timed out before result persisted",
			CreatedAt:    now.Add(-20 * time.Minute),
			UpdatedAt:    now.Add(-20 * time.Minute),
		}},
	})

	if err := engine.RecoverStudioBatchMaterialization(ctx, "batch-1"); err != nil {
		t.Fatalf("RecoverStudioBatchMaterialization() error = %v", err)
	}

	detail, err := repo.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}
	if detail.Items[0].Status != StudioBatchItemStatusPending {
		t.Fatalf("item status = %q, want pending for auto retry", detail.Items[0].Status)
	}
	if detail.Items[0].LastError != "" {
		t.Fatalf("item last error = %q, want cleared", detail.Items[0].LastError)
	}
	if detail.AttemptsByItem["item-1"][0].Status != StudioGenerationAttemptStatusFailed {
		t.Fatalf("attempt status = %q, want failed preserved", detail.AttemptsByItem["item-1"][0].Status)
	}
	if got := detail.Batch.Status; got != StudioBatchStatusGenerating {
		t.Fatalf("batch status = %q, want generating after item requeue", got)
	}
}

type studioBatchGenerationSessionRepoStub struct {
	session *SheinStudioSession
}

func (s *studioBatchGenerationSessionRepoStub) FindLatestSessionBySelectionKey(context.Context, string) (*SheinStudioSession, error) {
	return nil, nil
}

func (s *studioBatchGenerationSessionRepoStub) CreateSession(context.Context, *SheinStudioSession) error {
	return nil
}

func (s *studioBatchGenerationSessionRepoStub) GetSession(context.Context, string) (*SheinStudioSession, error) {
	if s.session == nil {
		return nil, nil
	}
	cloned := *s.session
	return &cloned, nil
}

func (s *studioBatchGenerationSessionRepoStub) UpdateSession(context.Context, *SheinStudioSession) error {
	return nil
}

func (s *studioBatchGenerationSessionRepoStub) DeleteSession(context.Context, string) error {
	return nil
}

func (s *studioBatchGenerationSessionRepoStub) ReplaceDesigns(context.Context, string, []string, []SheinStudioDesign) error {
	return nil
}

func (s *studioBatchGenerationSessionRepoStub) UpsertDesigns(context.Context, string, []string, []SheinStudioDesign) error {
	return nil
}

func (s *studioBatchGenerationSessionRepoStub) ListSessionDesigns(context.Context, string) ([]SheinStudioDesign, error) {
	return nil, nil
}

func (s *studioBatchGenerationSessionRepoStub) CountSessionDesignsBySessionIDs(context.Context, []string) (map[string]int, error) {
	return nil, nil
}

func (s *studioBatchGenerationSessionRepoStub) ListGalleryItems(context.Context, int) ([]SheinStudioSessionGalleryItem, error) {
	return nil, nil
}

func (s *studioBatchGenerationSessionRepoStub) ListBatchSessions(context.Context, int) ([]SheinStudioSession, error) {
	return nil, nil
}

func (s *studioBatchGenerationSessionRepoStub) ListTenantBatchNames(context.Context) ([]string, error) {
	return nil, nil
}

type studioBatchGenerationSeed struct {
	batch    StudioBatchRecord
	items    []StudioBatchItemRecord
	attempts []StudioGenerationAttemptRecord
	designs  []StudioMaterializedDesignRecord
}

func seedStudioBatchGenerationGraph(t *testing.T, repo StudioBatchRepository, ctx context.Context, seed studioBatchGenerationSeed) {
	t.Helper()

	now := time.Date(2026, 6, 1, 7, 0, 0, 0, time.UTC)
	seed.batch.CreatedAt = now
	seed.batch.UpdatedAt = now
	for i := range seed.items {
		seed.items[i].CreatedAt = now.Add(time.Duration(i) * time.Second)
		seed.items[i].UpdatedAt = seed.items[i].CreatedAt
	}
	for i := range seed.attempts {
		seed.attempts[i].CreatedAt = now.Add(time.Duration(i) * time.Second)
		seed.attempts[i].UpdatedAt = seed.attempts[i].CreatedAt
	}
	for i := range seed.designs {
		seed.designs[i].CreatedAt = now.Add(time.Duration(i) * time.Second)
		seed.designs[i].UpdatedAt = seed.designs[i].CreatedAt
	}

	if err := repo.CreateStudioBatchGraph(ctx, &seed.batch, seed.items, seed.attempts, seed.designs); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}
}

func stubStudioBatchExecutionByItem(responses map[string]*StudioDesignResponse) func(context.Context, StudioBatchGenerateExecutionInput) (*StudioBatchGenerateExecutionOutput, error) {
	return func(ctx context.Context, input StudioBatchGenerateExecutionInput) (*StudioBatchGenerateExecutionOutput, error) {
		response := responses[input.ItemID]
		if response == nil {
			response = &StudioDesignResponse{}
		}
		return &StudioBatchGenerateExecutionOutput{
			Response: response,
			ItemID:   input.ItemID,
			BatchID:  input.BatchID,
		}, nil
	}
}

func testStudioDesignResponse(id string, imageURL string) *StudioDesignResponse {
	return &StudioDesignResponse{
		Images: []StudioGeneratedImage{{
			ID:       id,
			ImageURL: imageURL,
		}},
	}
}

func testStudioBatchSelection(variantID int64, productName string, variantLabel string, width int, height int) SheinStudioSelection {
	return SheinStudioSelection{
		ProductID:          variantID,
		ParentProductID:    7001,
		VariantID:          variantID,
		PrototypeGroupID:   9001,
		LayerID:            "layer-1",
		DesignType:         "material",
		ProductName:        productName,
		VariantLabel:       variantLabel,
		PrintableWidth:     width,
		PrintableHeight:    height,
		TemplateImageURL:   "https://cdn.example.com/template.png",
		MaskImageURL:       "https://cdn.example.com/mask.png",
		SelectedVariantIDs: []int64{variantID},
	}
}

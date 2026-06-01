package listingkit

import (
	"context"
	"encoding/json"
	"testing"
	"time"
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
		ProductName:        productName,
		VariantLabel:       variantLabel,
		PrintableWidth:     width,
		PrintableHeight:    height,
		SelectedVariantIDs: []int64{variantID},
	}
}

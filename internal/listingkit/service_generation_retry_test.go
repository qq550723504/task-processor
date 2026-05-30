package listingkit

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"task-processor/internal/asset"
	assetbundle "task-processor/internal/asset/bundle"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
	assetrepo "task-processor/internal/asset/repository"
	"task-processor/internal/catalog"
	"task-processor/internal/catalog/canonical"
	common "task-processor/internal/publishing/common"
)

type stubRetryNilDispatchGenerator struct{}

type recordingRetryPersistenceAssetRepo struct {
	delegate               assetrepo.Repository
	calls                  []string
	saveInventoryErr       error
	saveGenerationTasksErr error
}

func newRecordingRetryPersistenceAssetRepo() *recordingRetryPersistenceAssetRepo {
	return &recordingRetryPersistenceAssetRepo{delegate: assetrepo.NewMemRepository()}
}

func (r *recordingRetryPersistenceAssetRepo) SaveInventory(ctx context.Context, inventory *asset.Inventory) error {
	r.calls = append(r.calls, "save_inventory")
	if r.saveInventoryErr != nil {
		return r.saveInventoryErr
	}
	return r.delegate.SaveInventory(ctx, inventory)
}

func (r *recordingRetryPersistenceAssetRepo) GetInventory(ctx context.Context, ref asset.InventoryRef) (*asset.Inventory, error) {
	return r.delegate.GetInventory(ctx, ref)
}

func (r *recordingRetryPersistenceAssetRepo) SaveGenerationTasks(ctx context.Context, taskID string, tasks []assetgeneration.Task) error {
	r.calls = append(r.calls, "save_generation_tasks")
	if r.saveGenerationTasksErr != nil {
		return r.saveGenerationTasksErr
	}
	return r.delegate.SaveGenerationTasks(ctx, taskID, tasks)
}

func (r *recordingRetryPersistenceAssetRepo) ListGenerationTasks(ctx context.Context, taskID string) ([]assetgeneration.Task, error) {
	return r.delegate.ListGenerationTasks(ctx, taskID)
}

func (r *recordingRetryPersistenceAssetRepo) resetCalls() {
	r.calls = nil
}

func (s *stubRetryNilDispatchGenerator) Plan(ctx context.Context, req assetgeneration.Request) (*assetgeneration.Result, error) {
	return &assetgeneration.Result{}, nil
}

func (s *stubRetryNilDispatchGenerator) Execute(ctx context.Context, req assetgeneration.Request) (*assetgeneration.Result, error) {
	return &assetgeneration.Result{}, nil
}

func (s *stubRetryNilDispatchGenerator) Dispatch(ctx context.Context, req assetgeneration.DispatchRequest) (*assetgeneration.Result, error) {
	return nil, nil
}

func TestRetryGenerationPersistenceSavesInventoryBeforeTasks(t *testing.T) {
	t.Parallel()

	assetRepository := newRecordingRetryPersistenceAssetRepo()
	inventory := &asset.Inventory{
		Ref: asset.InventoryRef{TaskID: "task-generation-retry-persist-order-1"},
		Records: []asset.AssetRecord{
			{ID: "gallery-1", TaskID: "task-generation-retry-persist-order-1", Kind: asset.KindGalleryImage, Origin: asset.OriginDerived},
		},
		Summary: &asset.InventorySummary{TotalRecords: 1, DerivedRecords: 1},
	}
	updatedTasks := []assetgeneration.Task{{
		TaskID:          "task-generation-retry-persist-order-1",
		ID:              "amazon:amazon-lifestyle",
		Platform:        "amazon",
		RecipeID:        "amazon-lifestyle",
		AssetKind:       asset.KindSceneImage,
		Slot:            "auxiliary",
		ExecutionStatus: "completed",
	}}

	if err := buildRetryGenerationPersistPhase(assetRepository).run(context.Background(), "task-generation-retry-persist-order-1", inventory, updatedTasks); err != nil {
		t.Fatalf("retryGenerationPersistPhase.run() error = %v", err)
	}
	if !reflect.DeepEqual(assetRepository.calls, []string{"save_inventory", "save_generation_tasks"}) {
		t.Fatalf("persistence call order = %+v, want inventory before generation tasks", assetRepository.calls)
	}
	persistedInventory, err := assetRepository.GetInventory(context.Background(), asset.InventoryRef{TaskID: "task-generation-retry-persist-order-1"})
	if err != nil {
		t.Fatalf("GetInventory() error = %v", err)
	}
	if !reflect.DeepEqual(persistedInventory, inventory) {
		t.Fatalf("persisted inventory = %+v, want %+v", persistedInventory, inventory)
	}
	persistedTasks, err := assetRepository.ListGenerationTasks(context.Background(), "task-generation-retry-persist-order-1")
	if err != nil {
		t.Fatalf("ListGenerationTasks() error = %v", err)
	}
	if !reflect.DeepEqual(persistedTasks, updatedTasks) {
		t.Fatalf("persisted tasks = %+v, want %+v", persistedTasks, updatedTasks)
	}
}

func TestRetryGenerationPersistenceReturnsSaveErrors(t *testing.T) {
	t.Parallel()

	inventory := &asset.Inventory{Ref: asset.InventoryRef{TaskID: "task-generation-retry-persist-error-1"}}
	updatedTasks := []assetgeneration.Task{{
		TaskID:          "task-generation-retry-persist-error-1",
		ID:              "amazon:amazon-lifestyle",
		Platform:        "amazon",
		RecipeID:        "amazon-lifestyle",
		AssetKind:       asset.KindSceneImage,
		Slot:            "auxiliary",
		ExecutionStatus: "completed",
	}}

	t.Run("inventory_error", func(t *testing.T) {
		t.Parallel()

		wantErr := errors.New("save inventory failed")
		assetRepository := newRecordingRetryPersistenceAssetRepo()
		assetRepository.saveInventoryErr = wantErr

		err := buildRetryGenerationPersistPhase(assetRepository).run(context.Background(), "task-generation-retry-persist-error-1", inventory, updatedTasks)
		if !errors.Is(err, wantErr) {
			t.Fatalf("retryGenerationPersistPhase.run() error = %v, want %v", err, wantErr)
		}
		if !reflect.DeepEqual(assetRepository.calls, []string{"save_inventory"}) {
			t.Fatalf("persistence calls = %+v, want inventory save only", assetRepository.calls)
		}
	})

	t.Run("generation_tasks_error", func(t *testing.T) {
		t.Parallel()

		wantErr := errors.New("save generation tasks failed")
		assetRepository := newRecordingRetryPersistenceAssetRepo()
		assetRepository.saveGenerationTasksErr = wantErr

		err := buildRetryGenerationPersistPhase(assetRepository).run(context.Background(), "task-generation-retry-persist-error-1", inventory, updatedTasks)
		if !errors.Is(err, wantErr) {
			t.Fatalf("retryGenerationPersistPhase.run() error = %v, want %v", err, wantErr)
		}
		if !reflect.DeepEqual(assetRepository.calls, []string{"save_inventory", "save_generation_tasks"}) {
			t.Fatalf("persistence calls = %+v, want inventory then generation tasks before failing", assetRepository.calls)
		}
	})
}

func TestRetryGenerationMutationApplyMergesTasksAndReplacesRetriedAssets(t *testing.T) {
	t.Parallel()

	inventory := &asset.Inventory{
		Records: []asset.AssetRecord{
			{ID: "source-1", Kind: asset.KindGalleryImage, Origin: asset.OriginDerived, URL: "file:///tmp/source.jpg"},
			{
				ID:       "scene-stub-1",
				TaskID:   "task-retry-mutation-1",
				Kind:     asset.KindSceneImage,
				Origin:   asset.OriginGenerated,
				URL:      "file:///tmp/scene-stub.jpg",
				RecipeID: "amazon-lifestyle",
				Metadata: map[string]string{"bundle_slot": "auxiliary"},
			},
			{
				ID:       "gallery-stub-1",
				TaskID:   "task-retry-mutation-1",
				Kind:     asset.KindSceneImage,
				Origin:   asset.OriginGenerated,
				URL:      "file:///tmp/gallery-stub.jpg",
				RecipeID: "amazon-gallery-scene",
				Metadata: map[string]string{"bundle_slot": "gallery"},
			},
		},
		Summary: &asset.InventorySummary{TotalRecords: 3, DerivedRecords: 1, GeneratedRecords: 2, RecipeCount: 2},
	}
	existingTasks := []assetgeneration.Task{
		{
			ID:              "amazon:amazon-lifestyle",
			TaskID:          "task-retry-mutation-1",
			Platform:        "amazon",
			RecipeID:        "amazon-lifestyle",
			Slot:            "auxiliary",
			AssetKind:       asset.KindSceneImage,
			ExecutionStatus: "planned",
			Metadata:        map[string]string{"existing": "value"},
		},
		{
			ID:              "amazon:amazon-gallery-scene",
			TaskID:          "task-retry-mutation-1",
			Platform:        "amazon",
			RecipeID:        "amazon-gallery-scene",
			Slot:            "gallery",
			AssetKind:       asset.KindSceneImage,
			ExecutionStatus: "completed",
			Metadata:        map[string]string{"preserved": "value"},
		},
	}
	selectedTasks := []assetgeneration.Task{{
		ID:              "amazon:amazon-lifestyle",
		TaskID:          "task-retry-mutation-1",
		Platform:        "amazon",
		RecipeID:        "amazon-lifestyle",
		Slot:            "auxiliary",
		AssetKind:       asset.KindSceneImage,
		ExecutionStatus: "planned",
	}}
	dispatchResult := &assetgeneration.Result{
		Tasks: []assetgeneration.Task{{
			ID:              "amazon:amazon-lifestyle",
			TaskID:          "task-retry-mutation-1",
			Platform:        "amazon",
			RecipeID:        "amazon-lifestyle",
			Slot:            "auxiliary",
			AssetKind:       asset.KindSceneImage,
			ExecutionStatus: "completed",
			ExecutionMode:   assetgeneration.ExecutionModeRendererBacked,
			Metadata:        map[string]string{"updated": "value"},
		}},
		Assets: []asset.AssetRecord{{
			ID:       "scene-rendered-1",
			TaskID:   "task-retry-mutation-1",
			Kind:     asset.KindSceneImage,
			Origin:   asset.OriginGenerated,
			URL:      "file:///tmp/scene-rendered.jpg",
			RecipeID: "amazon-lifestyle",
			Metadata: map[string]string{"bundle_slot": "auxiliary"},
		}},
	}

	got := buildRetryGenerationMutationPhase().run(inventory, existingTasks, selectedTasks, dispatchResult)

	wantTasks := []assetgeneration.Task{
		{
			ID:              "amazon:amazon-lifestyle",
			TaskID:          "task-retry-mutation-1",
			Platform:        "amazon",
			RecipeID:        "amazon-lifestyle",
			Slot:            "auxiliary",
			AssetKind:       asset.KindSceneImage,
			ExecutionStatus: "completed",
			ExecutionMode:   assetgeneration.ExecutionModeRendererBacked,
			Metadata:        map[string]string{"updated": "value"},
		},
		{
			ID:              "amazon:amazon-gallery-scene",
			TaskID:          "task-retry-mutation-1",
			Platform:        "amazon",
			RecipeID:        "amazon-gallery-scene",
			Slot:            "gallery",
			AssetKind:       asset.KindSceneImage,
			ExecutionStatus: "completed",
			Metadata:        map[string]string{"preserved": "value"},
		},
	}
	if !reflect.DeepEqual(got, wantTasks) {
		t.Fatalf("mutation tasks = %+v, want %+v", got, wantTasks)
	}

	if len(inventory.Records) != 3 {
		t.Fatalf("inventory records = %+v, want 3 records after targeted replacement", inventory.Records)
	}
	recordIDs := make([]string, 0, len(inventory.Records))
	for _, record := range inventory.Records {
		recordIDs = append(recordIDs, record.ID)
	}
	wantRecordIDs := []string{"source-1", "gallery-stub-1", "scene-rendered-1"}
	if !reflect.DeepEqual(recordIDs, wantRecordIDs) {
		t.Fatalf("inventory record ids = %+v, want %+v", recordIDs, wantRecordIDs)
	}
	if inventory.Summary == nil {
		t.Fatalf("inventory summary = nil, want rebuilt summary")
	}
	if inventory.Summary.TotalRecords != 3 || inventory.Summary.DerivedRecords != 1 || inventory.Summary.GeneratedRecords != 2 || inventory.Summary.RecipeCount != 2 {
		t.Fatalf("inventory summary = %+v, want rebuilt counts", inventory.Summary)
	}
}

func TestRetryGenerationMutationApplySkipsInventoryMutationWhenDispatchResultNil(t *testing.T) {
	t.Parallel()

	inventory := &asset.Inventory{
		Records: []asset.AssetRecord{
			{ID: "source-1", Kind: asset.KindGalleryImage, Origin: asset.OriginDerived, URL: "file:///tmp/source.jpg"},
			{
				ID:       "scene-stub-1",
				TaskID:   "task-retry-mutation-nil-1",
				Kind:     asset.KindSceneImage,
				Origin:   asset.OriginGenerated,
				URL:      "file:///tmp/scene-stub.jpg",
				RecipeID: "amazon-lifestyle",
				Metadata: map[string]string{"bundle_slot": "auxiliary"},
			},
		},
		Summary: &asset.InventorySummary{TotalRecords: 2, DerivedRecords: 1, GeneratedRecords: 1, RecipeCount: 1},
	}
	wantRecords := append([]asset.AssetRecord(nil), inventory.Records...)
	wantSummary := *inventory.Summary
	existingTasks := []assetgeneration.Task{
		{
			ID:              "amazon:amazon-lifestyle",
			TaskID:          "task-retry-mutation-nil-1",
			Platform:        "amazon",
			RecipeID:        "amazon-lifestyle",
			Slot:            "auxiliary",
			AssetKind:       asset.KindSceneImage,
			ExecutionStatus: "planned",
			Metadata:        map[string]string{"existing": "value"},
		},
		{
			ID:              "amazon:amazon-gallery-scene",
			TaskID:          "task-retry-mutation-nil-1",
			Platform:        "amazon",
			RecipeID:        "amazon-gallery-scene",
			Slot:            "gallery",
			AssetKind:       asset.KindSceneImage,
			ExecutionStatus: "completed",
			Metadata:        map[string]string{"preserved": "value"},
		},
	}
	selectedTasks := []assetgeneration.Task{{
		ID:              "amazon:amazon-lifestyle",
		TaskID:          "task-retry-mutation-nil-1",
		Platform:        "amazon",
		RecipeID:        "amazon-lifestyle",
		Slot:            "auxiliary",
		AssetKind:       asset.KindSceneImage,
		ExecutionStatus: "planned",
	}}

	got := buildRetryGenerationMutationPhase().run(inventory, existingTasks, selectedTasks, nil)

	if !reflect.DeepEqual(got, existingTasks) {
		t.Fatalf("mutation tasks = %+v, want unchanged %+v", got, existingTasks)
	}
	if !reflect.DeepEqual(inventory.Records, wantRecords) {
		t.Fatalf("inventory records = %+v, want unchanged %+v", inventory.Records, wantRecords)
	}
	if inventory.Summary == nil {
		t.Fatalf("inventory summary = nil, want unchanged summary")
	}
	if !reflect.DeepEqual(*inventory.Summary, wantSummary) {
		t.Fatalf("inventory summary = %+v, want unchanged %+v", inventory.Summary, &wantSummary)
	}
}

func TestRetryTaskGenerationTasksIncludesMatchedQueueSummary(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	assetRepository := assetrepo.NewMemRepository()
	renderer := &stubServiceDeferredRenderer{
		result: &asset.AssetRecord{
			ID:       "scene-rendered-2",
			Kind:     asset.KindSceneImage,
			Origin:   asset.OriginGenerated,
			Role:     "scene",
			URL:      "file:///tmp/scene-rendered-2.jpg",
			RecipeID: "amazon-lifestyle",
			Metadata: map[string]string{"renderer": "service-test"},
		},
	}
	svc := &service{
		repo:                repo,
		assetRepo:           assetRepository,
		assetRecipeResolver: assetrecipe.NewStaticResolver(),
		assetBundleBuilder:  assetbundle.NewBuilder(),
		assetGenerator: assetgeneration.NewService(assetgeneration.Config{
			DeferredRenderer: renderer,
		}),
	}

	task := &Task{
		ID:        "task-generation-retry-match-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"amazon"}},
		Result: &ListingKitResult{
			TaskID:           "task-generation-retry-match-1",
			Platforms:        []string{"amazon"},
			CanonicalProduct: &canonical.Product{CategoryPath: []string{"Electronics", "Audio"}},
			CatalogProduct:   &catalog.Product{Title: "Portable Speaker", CategoryPath: []string{"Electronics", "Audio"}},
			Amazon: &AmazonPackage{
				ImageBundle: &common.PublishImageBundle{
					Platform: "amazon",
					Auxiliary: []common.BundleSlot{{
						Key:             "auxiliary",
						Purpose:         "scene",
						RecipeID:        "amazon-lifestyle",
						IdealKind:       string(asset.KindSceneImage),
						StateLabel:      "fallback_in_use",
						SatisfiedBy:     "fallback_asset",
						ExecutionStatus: "fallback",
					}},
				},
			},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	inventory := &asset.Inventory{
		Ref: asset.InventoryRef{TaskID: task.ID},
		Records: []asset.AssetRecord{
			{ID: "gallery-1", TaskID: task.ID, Kind: asset.KindGalleryImage, Origin: asset.OriginDerived, URL: "file:///tmp/gallery.jpg"},
			{ID: "scene-stub-1", TaskID: task.ID, Kind: asset.KindSceneImage, Origin: asset.OriginGenerated, URL: "file:///tmp/scene-stub.jpg", RecipeID: "amazon-lifestyle", Metadata: map[string]string{"execution_mode": assetgeneration.ExecutionModeDeferredStub, "bundle_slot": "auxiliary"}},
		},
		Summary: &asset.InventorySummary{TotalRecords: 2, GeneratedRecords: 1},
	}
	if err := assetRepository.SaveInventory(context.Background(), inventory); err != nil {
		t.Fatalf("SaveInventory() error = %v", err)
	}
	persistedTasks := []assetgeneration.Task{{
		TaskID:          task.ID,
		ID:              "amazon:amazon-lifestyle",
		Platform:        "amazon",
		RecipeID:        "amazon-lifestyle",
		AssetKind:       asset.KindSceneImage,
		Slot:            "auxiliary",
		Purpose:         "scene",
		ExecutionStatus: "completed",
		ExecutionMode:   assetgeneration.ExecutionModeDeferredStub,
		CanExecute:      true,
		SatisfiedBy:     "fallback_asset",
		SourceAssetIDs:  []string{"gallery-1"},
	}}
	if err := assetRepository.SaveGenerationTasks(context.Background(), task.ID, persistedTasks); err != nil {
		t.Fatalf("SaveGenerationTasks() error = %v", err)
	}

	page, err := svc.RetryTaskGenerationTasks(context.Background(), task.ID, &RetryGenerationTasksRequest{
		FallbackOnly: true,
		Slots:        []string{"auxiliary"},
	})
	if err != nil {
		t.Fatalf("RetryTaskGenerationTasks() error = %v", err)
	}
	if page.MatchedQueue == nil || page.MatchedQueue.Summary == nil {
		t.Fatalf("matched queue = %+v, want summary", page.MatchedQueue)
	}
	if page.MatchedQueue.Summary.TotalItems != 1 || len(page.MatchedQueue.Items) != 1 {
		t.Fatalf("matched queue = %+v, want one matched item", page.MatchedQueue)
	}
	if page.MatchedQueue.Items[0].Slot != "auxiliary" {
		t.Fatalf("matched queue item = %+v, want auxiliary slot", page.MatchedQueue.Items[0])
	}
	if page.MatchedQueue.Summary.PlatformStateCounts["amazon"]["completed"] != 1 {
		t.Fatalf("matched queue summary = %+v, want platform-state aggregation", page.MatchedQueue.Summary)
	}
	if page.ExecutedQueue == nil || page.ExecutedQueue.Summary == nil {
		t.Fatalf("executed queue = %+v, want summary", page.ExecutedQueue)
	}
	if page.ExecutedQueue.Summary.TotalItems != 1 || len(page.ExecutedQueue.Items) != 1 {
		t.Fatalf("executed queue = %+v, want one executed item", page.ExecutedQueue)
	}
	if page.ExecutedQueue.Items[0].State != "completed" {
		t.Fatalf("executed queue item = %+v, want completed state", page.ExecutedQueue.Items[0])
	}
	if page.ExecutedQueue.Items[0].ExecutionQuality != "renderer_output" {
		t.Fatalf("executed queue item = %+v, want renderer_output quality", page.ExecutedQueue.Items[0])
	}
	if page.ExecutedQueue.Items[0].ExecutionQualityLabel != "Renderer Output" {
		t.Fatalf("executed queue item = %+v, want renderer quality label", page.ExecutedQueue.Items[0])
	}
	if page.ExecutedQueue.Summary.ExecutionQualityCounts["renderer_output"] != 1 {
		t.Fatalf("executed queue summary = %+v, want renderer_output count", page.ExecutedQueue.Summary)
	}
	if page.ExecutedQueue.Summary.ExecutionQualityLabels["renderer_output"] != "Renderer Output" {
		t.Fatalf("executed queue summary = %+v, want renderer quality label map", page.ExecutedQueue.Summary)
	}
	if page.ExecutedQueue.Summary.PlatformExecutionQualityCounts["amazon"]["renderer_output"] != 1 {
		t.Fatalf("executed queue summary = %+v, want platform quality aggregation", page.ExecutedQueue.Summary)
	}
	if page.ExecutedQueue.Summary.QualityGradeCounts["ideal"] != 1 {
		t.Fatalf("executed queue summary = %+v, want ideal grade aggregation", page.ExecutedQueue.Summary)
	}
	if page.ExecutedQueue.Summary.PlatformQualityGradeCounts["amazon"]["ideal"] != 1 {
		t.Fatalf("executed queue summary = %+v, want platform ideal grade aggregation", page.ExecutedQueue.Summary)
	}
	if page.ExecutedQueue.Summary.DominantQualityGrade != "ideal" || page.ExecutedQueue.Summary.DominantQualityGradeLabel != "Ideal" {
		t.Fatalf("executed queue summary = %+v, want dominant ideal grade", page.ExecutedQueue.Summary)
	}
	if page.ExecutedQueue.Summary.GradeStateCounts["ideal"]["completed"] != 1 {
		t.Fatalf("executed queue summary = %+v, want ideal/completed grade-state aggregation", page.ExecutedQueue.Summary)
	}
	if page.ExecutedQueue.Summary.PlatformGradeStateCounts["amazon"]["ideal"]["completed"] != 1 {
		t.Fatalf("executed queue summary = %+v, want platform ideal/completed grade-state aggregation", page.ExecutedQueue.Summary)
	}
}

func TestRetryTaskGenerationTasksFiltersByExecutionQuality(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	assetRepository := assetrepo.NewMemRepository()
	svc := &service{
		repo:                repo,
		assetRepo:           assetRepository,
		assetRecipeResolver: assetrecipe.NewStaticResolver(),
		assetBundleBuilder:  assetbundle.NewBuilder(),
		assetGenerator:      assetgeneration.NewService(assetgeneration.Config{}),
	}

	task := &Task{
		ID:        "task-generation-retry-quality-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"amazon", "shein"}},
		Result: &ListingKitResult{
			TaskID: "task-generation-retry-quality-1",
			Amazon: &AmazonPackage{ImageBundle: &common.PublishImageBundle{
				Platform: "amazon",
				Auxiliary: []common.BundleSlot{{
					Key:             "auxiliary",
					Purpose:         "scene",
					RecipeID:        "amazon-lifestyle",
					IdealKind:       string(asset.KindSceneImage),
					StateLabel:      "fallback_in_use",
					SatisfiedBy:     "fallback_asset",
					ExecutionStatus: "fallback",
				}},
			}},
			Shein: &SheinPackage{ImageBundle: &common.PublishImageBundle{
				Platform: "shein",
				Main: &common.BundleSlot{
					Key:             "main",
					Purpose:         "main",
					RecipeID:        "shein-main-model",
					IdealKind:       string(asset.KindModelImage),
					StateLabel:      "ready",
					SatisfiedBy:     "exact_asset",
					ExecutionStatus: "ready",
				},
			}},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if err := assetRepository.SaveInventory(context.Background(), &asset.Inventory{Ref: asset.InventoryRef{TaskID: task.ID}}); err != nil {
		t.Fatalf("SaveInventory() error = %v", err)
	}
	persistedTasks := []assetgeneration.Task{
		{
			TaskID:          task.ID,
			ID:              "amazon:amazon-lifestyle",
			Platform:        "amazon",
			RecipeID:        "amazon-lifestyle",
			AssetKind:       asset.KindSceneImage,
			Slot:            "auxiliary",
			Purpose:         "scene",
			ExecutionStatus: "completed",
			ExecutionMode:   assetgeneration.ExecutionModeDeferredStub,
			CanExecute:      true,
			SatisfiedBy:     "fallback_asset",
		},
		{
			TaskID:          task.ID,
			ID:              "shein:shein-main-model",
			Platform:        "shein",
			RecipeID:        "shein-main-model",
			AssetKind:       asset.KindModelImage,
			Slot:            "main",
			Purpose:         "main",
			ExecutionStatus: "completed",
			ExecutionMode:   assetgeneration.ExecutionModeRendererBacked,
			CanExecute:      true,
			SatisfiedBy:     assetgeneration.ExecutionModeGeneratedAsset,
		},
	}
	if err := assetRepository.SaveGenerationTasks(context.Background(), task.ID, persistedTasks); err != nil {
		t.Fatalf("SaveGenerationTasks() error = %v", err)
	}

	page, err := svc.RetryTaskGenerationTasks(context.Background(), task.ID, &RetryGenerationTasksRequest{
		ExecutionQuality: "stub_fallback",
	})
	if err != nil {
		t.Fatalf("RetryTaskGenerationTasks() error = %v", err)
	}
	if page.MatchedQueue == nil || page.MatchedQueue.Summary == nil {
		t.Fatalf("matched queue = %+v, want summary", page.MatchedQueue)
	}
	if page.MatchedQueue.Summary.TotalItems != 1 || len(page.MatchedQueue.Items) != 1 {
		t.Fatalf("matched queue = %+v, want one stub_fallback item", page.MatchedQueue)
	}
	if page.MatchedQueue.Items[0].Slot != "auxiliary" {
		t.Fatalf("matched queue item = %+v, want auxiliary slot selected by stub_fallback filter", page.MatchedQueue.Items[0])
	}
}

func TestRetryTaskGenerationTasksFiltersByExecutionQualityLabel(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	assetRepository := assetrepo.NewMemRepository()
	svc := &service{
		repo:                repo,
		assetRepo:           assetRepository,
		assetRecipeResolver: assetrecipe.NewStaticResolver(),
		assetBundleBuilder:  assetbundle.NewBuilder(),
		assetGenerator:      assetgeneration.NewService(assetgeneration.Config{}),
	}

	task := &Task{
		ID:        "task-generation-retry-quality-label-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"amazon"}},
		Result: &ListingKitResult{
			TaskID: "task-generation-retry-quality-label-1",
			Amazon: &AmazonPackage{ImageBundle: &common.PublishImageBundle{
				Platform: "amazon",
				Auxiliary: []common.BundleSlot{{
					Key:             "auxiliary",
					Purpose:         "scene",
					RecipeID:        "amazon-lifestyle",
					IdealKind:       string(asset.KindSceneImage),
					StateLabel:      "fallback_in_use",
					SatisfiedBy:     "fallback_asset",
					ExecutionStatus: "fallback",
				}},
			}},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if err := assetRepository.SaveInventory(context.Background(), &asset.Inventory{Ref: asset.InventoryRef{TaskID: task.ID}}); err != nil {
		t.Fatalf("SaveInventory() error = %v", err)
	}
	persistedTasks := []assetgeneration.Task{{
		TaskID:          task.ID,
		ID:              "amazon:amazon-lifestyle",
		Platform:        "amazon",
		RecipeID:        "amazon-lifestyle",
		AssetKind:       asset.KindSceneImage,
		Slot:            "auxiliary",
		Purpose:         "scene",
		ExecutionStatus: "completed",
		ExecutionMode:   assetgeneration.ExecutionModeDeferredStub,
		CanExecute:      true,
		SatisfiedBy:     "fallback_asset",
	}}
	if err := assetRepository.SaveGenerationTasks(context.Background(), task.ID, persistedTasks); err != nil {
		t.Fatalf("SaveGenerationTasks() error = %v", err)
	}

	page, err := svc.RetryTaskGenerationTasks(context.Background(), task.ID, &RetryGenerationTasksRequest{
		ExecutionQualityLabel: "Stub Fallback",
	})
	if err != nil {
		t.Fatalf("RetryTaskGenerationTasks() error = %v", err)
	}
	if page.MatchedQueue == nil || len(page.MatchedQueue.Items) != 1 {
		t.Fatalf("matched queue = %+v, want one item", page.MatchedQueue)
	}
	if page.MatchedQueue.Items[0].ExecutionQualityLabel != "Queued" {
		t.Fatalf("matched queue item = %+v, want rebuilt queue item for selected target", page.MatchedQueue.Items[0])
	}
}

func TestRetryTaskGenerationTasksFiltersByQualityGradeLabel(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	assetRepository := assetrepo.NewMemRepository()
	svc := &service{
		repo:                repo,
		assetRepo:           assetRepository,
		assetRecipeResolver: assetrecipe.NewStaticResolver(),
		assetBundleBuilder:  assetbundle.NewBuilder(),
		assetGenerator:      assetgeneration.NewService(assetgeneration.Config{}),
	}

	task := &Task{
		ID:        "task-generation-retry-grade-label-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"amazon"}},
		Result: &ListingKitResult{
			TaskID: "task-generation-retry-grade-label-1",
			Amazon: &AmazonPackage{ImageBundle: &common.PublishImageBundle{
				Platform: "amazon",
				Auxiliary: []common.BundleSlot{{
					Key:             "auxiliary",
					Purpose:         "scene",
					RecipeID:        "amazon-lifestyle",
					IdealKind:       string(asset.KindSceneImage),
					StateLabel:      "fallback_in_use",
					SatisfiedBy:     "fallback_asset",
					ExecutionStatus: "fallback",
				}},
			}},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if err := assetRepository.SaveInventory(context.Background(), &asset.Inventory{Ref: asset.InventoryRef{TaskID: task.ID}}); err != nil {
		t.Fatalf("SaveInventory() error = %v", err)
	}
	persistedTasks := []assetgeneration.Task{{
		TaskID:          task.ID,
		ID:              "amazon:amazon-lifestyle",
		Platform:        "amazon",
		RecipeID:        "amazon-lifestyle",
		AssetKind:       asset.KindSceneImage,
		Slot:            "auxiliary",
		Purpose:         "scene",
		ExecutionStatus: "completed",
		ExecutionMode:   assetgeneration.ExecutionModeDeferredStub,
		CanExecute:      true,
		SatisfiedBy:     "fallback_asset",
	}}
	if err := assetRepository.SaveGenerationTasks(context.Background(), task.ID, persistedTasks); err != nil {
		t.Fatalf("SaveGenerationTasks() error = %v", err)
	}

	page, err := svc.RetryTaskGenerationTasks(context.Background(), task.ID, &RetryGenerationTasksRequest{
		QualityGradeLabel: "Provisional",
	})
	if err != nil {
		t.Fatalf("RetryTaskGenerationTasks() error = %v", err)
	}
	if page.MatchedQueue == nil || len(page.MatchedQueue.Items) != 1 {
		t.Fatalf("matched queue = %+v, want one item", page.MatchedQueue)
	}
	if page.MatchedQueue.Items[0].Slot != "auxiliary" {
		t.Fatalf("matched queue item = %+v, want auxiliary slot selected by provisional grade", page.MatchedQueue.Items[0])
	}
}

func TestRetryTaskGenerationTasksFiltersByQualityGrade(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	assetRepository := assetrepo.NewMemRepository()
	svc := &service{
		repo:                repo,
		assetRepo:           assetRepository,
		assetRecipeResolver: assetrecipe.NewStaticResolver(),
		assetBundleBuilder:  assetbundle.NewBuilder(),
		assetGenerator:      assetgeneration.NewService(assetgeneration.Config{}),
	}

	task := &Task{
		ID:        "task-generation-retry-grade-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"amazon"}},
		Result: &ListingKitResult{
			TaskID: "task-generation-retry-grade-1",
			Amazon: &AmazonPackage{ImageBundle: &common.PublishImageBundle{
				Platform: "amazon",
				Auxiliary: []common.BundleSlot{{
					Key:             "auxiliary",
					Purpose:         "scene",
					RecipeID:        "amazon-lifestyle",
					IdealKind:       string(asset.KindSceneImage),
					StateLabel:      "fallback_in_use",
					SatisfiedBy:     "fallback_asset",
					ExecutionStatus: "fallback",
				}},
			}},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if err := assetRepository.SaveInventory(context.Background(), &asset.Inventory{Ref: asset.InventoryRef{TaskID: task.ID}}); err != nil {
		t.Fatalf("SaveInventory() error = %v", err)
	}
	persistedTasks := []assetgeneration.Task{{
		TaskID:          task.ID,
		ID:              "amazon:amazon-lifestyle",
		Platform:        "amazon",
		RecipeID:        "amazon-lifestyle",
		AssetKind:       asset.KindSceneImage,
		Slot:            "auxiliary",
		Purpose:         "scene",
		ExecutionStatus: "completed",
		ExecutionMode:   assetgeneration.ExecutionModeDeferredStub,
		CanExecute:      true,
		SatisfiedBy:     "fallback_asset",
	}}
	if err := assetRepository.SaveGenerationTasks(context.Background(), task.ID, persistedTasks); err != nil {
		t.Fatalf("SaveGenerationTasks() error = %v", err)
	}

	page, err := svc.RetryTaskGenerationTasks(context.Background(), task.ID, &RetryGenerationTasksRequest{
		QualityGrade: "provisional",
	})
	if err != nil {
		t.Fatalf("RetryTaskGenerationTasks() error = %v", err)
	}
	if page.MatchedQueue == nil || len(page.MatchedQueue.Items) != 1 {
		t.Fatalf("matched queue = %+v, want one item", page.MatchedQueue)
	}
	if page.MatchedQueue.Items[0].Slot != "auxiliary" {
		t.Fatalf("matched queue item = %+v, want auxiliary slot selected by provisional grade", page.MatchedQueue.Items[0])
	}
}

func TestRetryTaskGenerationTasksReturnsEmptyPageWhenQueueFilterMatchesNothing(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	assetRepository := assetrepo.NewMemRepository()
	svc := &service{
		repo:                repo,
		assetRepo:           assetRepository,
		assetRecipeResolver: assetrecipe.NewStaticResolver(),
		assetBundleBuilder:  assetbundle.NewBuilder(),
		assetGenerator:      assetgeneration.NewService(assetgeneration.Config{}),
	}

	task := &Task{
		ID:        "task-generation-retry-empty-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"shein"}},
		Result: &ListingKitResult{
			TaskID: "task-generation-retry-empty-1",
			Shein: &SheinPackage{
				ImageBundle: &common.PublishImageBundle{
					Platform: "shein",
					Main: &common.BundleSlot{
						Key:             "main",
						Purpose:         "main",
						RecipeID:        "shein-main-model",
						StateLabel:      "ready",
						SatisfiedBy:     "exact_asset",
						ExecutionStatus: "ready",
					},
				},
			},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if err := assetRepository.SaveInventory(context.Background(), &asset.Inventory{Ref: asset.InventoryRef{TaskID: task.ID}}); err != nil {
		t.Fatalf("SaveInventory() error = %v", err)
	}
	persistedTasks := []assetgeneration.Task{{
		TaskID:          task.ID,
		ID:              "shein:shein-main-model",
		Platform:        "shein",
		RecipeID:        "shein-main-model",
		AssetKind:       asset.KindModelImage,
		Slot:            "main",
		Purpose:         "main",
		ExecutionStatus: "completed",
		ExecutionMode:   assetgeneration.ExecutionModeRendererBacked,
		CanExecute:      true,
		SatisfiedBy:     assetgeneration.ExecutionModeGeneratedAsset,
	}}
	if err := assetRepository.SaveGenerationTasks(context.Background(), task.ID, persistedTasks); err != nil {
		t.Fatalf("SaveGenerationTasks() error = %v", err)
	}

	page, err := svc.RetryTaskGenerationTasks(context.Background(), task.ID, &RetryGenerationTasksRequest{
		FallbackOnly: true,
		Slots:        []string{"main"},
	})
	if err != nil {
		t.Fatalf("RetryTaskGenerationTasks() error = %v", err)
	}
	if page.Total != 0 || len(page.Tasks) != 0 {
		t.Fatalf("page = %+v, want empty filtered page", page)
	}
	if page.MatchedQueue == nil || page.MatchedQueue.Summary == nil || page.MatchedQueue.Summary.TotalItems != 0 {
		t.Fatalf("matched queue = %+v, want empty matched queue", page.MatchedQueue)
	}
	if page.ExecutedQueue == nil || page.ExecutedQueue.Summary == nil || page.ExecutedQueue.Summary.TotalItems != 0 {
		t.Fatalf("executed queue = %+v, want empty executed queue", page.ExecutedQueue)
	}
	if len(page.ExecutedQueue.Summary.ExecutionQualityCounts) != 0 {
		t.Fatalf("executed queue summary = %+v, want empty execution quality counts", page.ExecutedQueue.Summary)
	}
}

func TestRetryTaskGenerationTasksReplacesFallbackAssetAndPersistsResult(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	assetRepository := assetrepo.NewMemRepository()
	renderer := &stubServiceDeferredRenderer{
		result: &asset.AssetRecord{
			ID:       "scene-rendered-1",
			Kind:     asset.KindSceneImage,
			Origin:   asset.OriginGenerated,
			Role:     "scene",
			URL:      "file:///tmp/scene-rendered.jpg",
			RecipeID: "amazon-lifestyle",
			Metadata: map[string]string{"renderer": "service-test"},
		},
	}
	svc := &service{
		repo:                repo,
		assetRepo:           assetRepository,
		assetRecipeResolver: assetrecipe.NewStaticResolver(),
		assetBundleBuilder:  assetbundle.NewBuilder(),
		assetGenerator: assetgeneration.NewService(assetgeneration.Config{
			DeferredRenderer: renderer,
		}),
	}

	task := &Task{
		ID:        "task-generation-retry-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"amazon"}},
		Result: &ListingKitResult{
			TaskID:           "task-generation-retry-1",
			Platforms:        []string{"amazon"},
			CanonicalProduct: &canonical.Product{CategoryPath: []string{"Electronics", "Audio"}},
			CatalogProduct:   &catalog.Product{Title: "Portable Speaker", CategoryPath: []string{"Electronics", "Audio"}},
			AssetBundle: &asset.Bundle{
				Assets: []asset.Asset{
					{ID: "gallery-1", Kind: asset.KindGalleryImage, URL: "file:///tmp/gallery.jpg", SourceURL: "https://example.com/gallery.jpg"},
					{ID: "scene-stub-1", Kind: asset.KindSceneImage, URL: "file:///tmp/scene-stub.jpg", RecipeID: "amazon-lifestyle", Metadata: map[string]string{"execution_mode": assetgeneration.ExecutionModeDeferredStub}},
				},
			},
			Amazon: &AmazonPackage{},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	inventory := &asset.Inventory{
		Ref: asset.InventoryRef{TaskID: task.ID},
		Records: []asset.AssetRecord{
			{ID: "gallery-1", TaskID: task.ID, Kind: asset.KindGalleryImage, Origin: asset.OriginDerived, URL: "file:///tmp/gallery.jpg", Metadata: map[string]string{"source_url": "https://example.com/gallery.jpg"}},
			{ID: "scene-stub-1", TaskID: task.ID, Kind: asset.KindSceneImage, Origin: asset.OriginGenerated, URL: "file:///tmp/scene-stub.jpg", RecipeID: "amazon-lifestyle", Metadata: map[string]string{"execution_mode": assetgeneration.ExecutionModeDeferredStub, "bundle_slot": "auxiliary"}},
			{ID: "scene-other-1", TaskID: task.ID, Kind: asset.KindSceneImage, Origin: asset.OriginGenerated, URL: "file:///tmp/scene-other.jpg", RecipeID: "amazon-lifestyle", Metadata: map[string]string{"execution_mode": assetgeneration.ExecutionModeDeferredStub, "bundle_slot": "gallery"}},
		},
		Summary: &asset.InventorySummary{TotalRecords: 3, GeneratedRecords: 2},
	}
	if err := assetRepository.SaveInventory(context.Background(), inventory); err != nil {
		t.Fatalf("SaveInventory() error = %v", err)
	}
	persistedTasks := []assetgeneration.Task{{
		TaskID:          task.ID,
		ID:              "amazon:amazon-lifestyle",
		Platform:        "amazon",
		RecipeID:        "amazon-lifestyle",
		AssetKind:       asset.KindSceneImage,
		Slot:            "auxiliary",
		Purpose:         "scene",
		Status:          "completed",
		ExecutionStatus: "completed",
		ExecutionMode:   assetgeneration.ExecutionModeDeferredStub,
		CanExecute:      true,
		SatisfiedBy:     assetgeneration.ExecutionModeGeneratedAsset,
		SourceAssetIDs:  []string{"gallery-1"},
	}}
	if err := assetRepository.SaveGenerationTasks(context.Background(), task.ID, persistedTasks); err != nil {
		t.Fatalf("SaveGenerationTasks() error = %v", err)
	}

	page, err := svc.RetryTaskGenerationTasks(context.Background(), task.ID, &RetryGenerationTasksRequest{})
	if err != nil {
		t.Fatalf("RetryTaskGenerationTasks() error = %v", err)
	}
	if page.Summary == nil || page.Summary.RendererBackedTasks != 1 {
		t.Fatalf("page summary = %+v, want renderer_backed_tasks=1", page.Summary)
	}
	if len(page.Tasks) != 1 || page.Tasks[0].ExecutionMode != assetgeneration.ExecutionModeRendererBacked {
		t.Fatalf("page tasks = %+v, want renderer_backed completed task", page.Tasks)
	}

	updatedTask, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if updatedTask.Result == nil || updatedTask.Result.AssetGenerationSummary == nil {
		t.Fatalf("updated result = %+v, want generation summary persisted", updatedTask.Result)
	}
	if updatedTask.Result.AssetGenerationSummary.RendererBackedTasks != 1 {
		t.Fatalf("updated summary = %+v, want renderer_backed_tasks=1", updatedTask.Result.AssetGenerationSummary)
	}

	updatedInventory, err := assetRepository.GetInventory(context.Background(), asset.InventoryRef{TaskID: task.ID})
	if err != nil {
		t.Fatalf("GetInventory() error = %v", err)
	}
	foundRendered := false
	foundOther := false
	for _, item := range updatedInventory.Records {
		if item.ID == "scene-rendered-1" && item.RecipeID == "amazon-lifestyle" {
			foundRendered = true
		}
		if item.ID == "scene-stub-1" {
			t.Fatalf("inventory records = %+v, want fallback asset replaced", updatedInventory.Records)
		}
		if item.ID == "scene-other-1" {
			foundOther = true
		}
	}
	if !foundRendered {
		t.Fatalf("inventory records = %+v, want rendered scene asset", updatedInventory.Records)
	}
	if !foundOther {
		t.Fatalf("inventory records = %+v, want non-target slot asset preserved", updatedInventory.Records)
	}
}

func TestRetryTaskGenerationTasksMergesReturnedTasksAndRefreshesRetriedAssets(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	assetRepository := assetrepo.NewMemRepository()
	renderer := &stubServiceDeferredRenderer{
		result: &asset.AssetRecord{
			ID:       "scene-rendered-merge-1",
			Kind:     asset.KindSceneImage,
			Origin:   asset.OriginGenerated,
			Role:     "scene",
			URL:      "file:///tmp/scene-rendered-merge.jpg",
			RecipeID: "amazon-lifestyle",
			Metadata: map[string]string{"renderer": "service-test"},
		},
	}
	svc := &service{
		repo:                repo,
		assetRepo:           assetRepository,
		assetRecipeResolver: assetrecipe.NewStaticResolver(),
		assetBundleBuilder:  assetbundle.NewBuilder(),
		assetGenerator: assetgeneration.NewService(assetgeneration.Config{
			DeferredRenderer: renderer,
		}),
	}

	task := &Task{
		ID:        "task-generation-retry-merge-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"amazon"}},
		Result: &ListingKitResult{
			TaskID:           "task-generation-retry-merge-1",
			Platforms:        []string{"amazon"},
			CanonicalProduct: &canonical.Product{CategoryPath: []string{"Electronics", "Audio"}},
			CatalogProduct:   &catalog.Product{Title: "Portable Speaker", CategoryPath: []string{"Electronics", "Audio"}},
			Amazon: &AmazonPackage{
				ImageBundle: &common.PublishImageBundle{
					Auxiliary: []common.BundleSlot{{
						Key:             "auxiliary",
						Purpose:         "scene",
						RecipeID:        "amazon-lifestyle",
						IdealKind:       string(asset.KindSceneImage),
						StateLabel:      "fallback_in_use",
						SatisfiedBy:     "fallback_asset",
						ExecutionStatus: "fallback",
					}},
					Gallery: []common.BundleSlot{{
						Key:             "gallery",
						Purpose:         "gallery",
						RecipeID:        "amazon-gallery-scene",
						IdealKind:       string(asset.KindSceneImage),
						StateLabel:      "fallback_in_use",
						SatisfiedBy:     "fallback_asset",
						ExecutionStatus: "fallback",
					}},
				},
			},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	inventory := &asset.Inventory{
		Ref: asset.InventoryRef{TaskID: task.ID},
		Records: []asset.AssetRecord{
			{ID: "gallery-1", TaskID: task.ID, Kind: asset.KindGalleryImage, Origin: asset.OriginDerived, URL: "file:///tmp/gallery.jpg"},
			{ID: "scene-stub-merge-1", TaskID: task.ID, Kind: asset.KindSceneImage, Origin: asset.OriginGenerated, URL: "file:///tmp/scene-stub-merge.jpg", RecipeID: "amazon-lifestyle", Metadata: map[string]string{"execution_mode": assetgeneration.ExecutionModeDeferredStub, "bundle_slot": "auxiliary"}},
			{ID: "scene-gallery-stub-1", TaskID: task.ID, Kind: asset.KindSceneImage, Origin: asset.OriginGenerated, URL: "file:///tmp/scene-gallery-stub.jpg", RecipeID: "amazon-gallery-scene", Metadata: map[string]string{"execution_mode": assetgeneration.ExecutionModeDeferredStub, "bundle_slot": "gallery"}},
		},
		Summary: &asset.InventorySummary{TotalRecords: 3, DerivedRecords: 1, GeneratedRecords: 2, RecipeCount: 2},
	}
	if err := assetRepository.SaveInventory(context.Background(), inventory); err != nil {
		t.Fatalf("SaveInventory() error = %v", err)
	}
	persistedTasks := []assetgeneration.Task{
		{
			TaskID:          task.ID,
			ID:              "amazon:amazon-lifestyle",
			Platform:        "amazon",
			RecipeID:        "amazon-lifestyle",
			AssetKind:       asset.KindSceneImage,
			Slot:            "auxiliary",
			Purpose:         "scene",
			Status:          "completed",
			ExecutionStatus: "completed",
			ExecutionMode:   assetgeneration.ExecutionModeDeferredStub,
			CanExecute:      true,
			SatisfiedBy:     "fallback_asset",
			SourceAssetIDs:  []string{"gallery-1"},
		},
		{
			TaskID:          task.ID,
			ID:              "amazon:amazon-gallery-scene",
			Platform:        "amazon",
			RecipeID:        "amazon-gallery-scene",
			AssetKind:       asset.KindSceneImage,
			Slot:            "gallery",
			Purpose:         "gallery",
			Status:          "completed",
			ExecutionStatus: "completed",
			ExecutionMode:   assetgeneration.ExecutionModeDeferredStub,
			CanExecute:      true,
			SatisfiedBy:     "fallback_asset",
			SourceAssetIDs:  []string{"gallery-1"},
		},
	}
	if err := assetRepository.SaveGenerationTasks(context.Background(), task.ID, persistedTasks); err != nil {
		t.Fatalf("SaveGenerationTasks() error = %v", err)
	}

	page, err := svc.RetryTaskGenerationTasks(context.Background(), task.ID, &RetryGenerationTasksRequest{
		Slots: []string{"auxiliary"},
	})
	if err != nil {
		t.Fatalf("RetryTaskGenerationTasks() error = %v", err)
	}

	if len(page.Tasks) != 2 {
		t.Fatalf("page tasks = %+v, want merged tasks", page.Tasks)
	}
	if page.Tasks[0].ID != "amazon:amazon-lifestyle" || page.Tasks[0].ExecutionMode != assetgeneration.ExecutionModeRendererBacked {
		t.Fatalf("retried task = %+v, want renderer-backed merged task", page.Tasks[0])
	}
	if page.Tasks[1].ID != "amazon:amazon-gallery-scene" || page.Tasks[1].ExecutionMode != assetgeneration.ExecutionModeDeferredStub {
		t.Fatalf("untouched task = %+v, want untouched persisted task", page.Tasks[1])
	}

	updatedInventory, err := assetRepository.GetInventory(context.Background(), asset.InventoryRef{TaskID: task.ID})
	if err != nil {
		t.Fatalf("GetInventory() error = %v", err)
	}
	recordIDs := make([]string, 0, len(updatedInventory.Records))
	for _, item := range updatedInventory.Records {
		recordIDs = append(recordIDs, item.ID)
	}
	wantRecordIDs := []string{"gallery-1", "scene-gallery-stub-1", "scene-rendered-merge-1"}
	if !reflect.DeepEqual(recordIDs, wantRecordIDs) {
		t.Fatalf("inventory record ids = %+v, want %+v", recordIDs, wantRecordIDs)
	}
	if updatedInventory.Summary == nil || updatedInventory.Summary.TotalRecords != 3 || updatedInventory.Summary.DerivedRecords != 1 || updatedInventory.Summary.GeneratedRecords != 2 || updatedInventory.Summary.RecipeCount != 2 {
		t.Fatalf("inventory summary = %+v, want rebuilt summary counts", updatedInventory.Summary)
	}
}

func TestRetryTaskGenerationTasksNilDispatchResultIsSafe(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	assetRepository := newRecordingRetryPersistenceAssetRepo()
	task := &Task{
		ID:        "task-generation-retry-nil-dispatch-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"amazon"}},
		Result: &ListingKitResult{
			TaskID:           "task-generation-retry-nil-dispatch-1",
			Platforms:        []string{"amazon"},
			CanonicalProduct: &canonical.Product{CategoryPath: []string{"Electronics", "Audio"}},
			CatalogProduct:   &catalog.Product{Title: "Portable Speaker", CategoryPath: []string{"Electronics", "Audio"}},
			Amazon: &AmazonPackage{
				ImageBundle: &common.PublishImageBundle{
					Auxiliary: []common.BundleSlot{{
						Key:             "auxiliary",
						Purpose:         "scene",
						RecipeID:        "amazon-lifestyle",
						IdealKind:       string(asset.KindSceneImage),
						StateLabel:      "fallback_in_use",
						SatisfiedBy:     "fallback_asset",
						ExecutionStatus: "fallback",
					}},
				},
			},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	inventory := &asset.Inventory{
		Ref: asset.InventoryRef{TaskID: task.ID},
		Records: []asset.AssetRecord{
			{ID: "gallery-1", TaskID: task.ID, Kind: asset.KindGalleryImage, Origin: asset.OriginDerived, URL: "file:///tmp/gallery.jpg"},
			{ID: "scene-stub-safe-1", TaskID: task.ID, Kind: asset.KindSceneImage, Origin: asset.OriginGenerated, URL: "file:///tmp/scene-stub-safe.jpg", RecipeID: "amazon-lifestyle", Metadata: map[string]string{"execution_mode": assetgeneration.ExecutionModeDeferredStub, "bundle_slot": "auxiliary"}},
		},
		Summary: &asset.InventorySummary{TotalRecords: 2, DerivedRecords: 1, GeneratedRecords: 1, RecipeCount: 1},
	}
	if err := assetRepository.SaveInventory(context.Background(), inventory); err != nil {
		t.Fatalf("SaveInventory() error = %v", err)
	}
	persistedTasks := []assetgeneration.Task{{
		TaskID:          task.ID,
		ID:              "amazon:amazon-lifestyle",
		Platform:        "amazon",
		RecipeID:        "amazon-lifestyle",
		AssetKind:       asset.KindSceneImage,
		Slot:            "auxiliary",
		Purpose:         "scene",
		Status:          "completed",
		ExecutionStatus: "completed",
		ExecutionMode:   assetgeneration.ExecutionModeDeferredStub,
		CanExecute:      true,
		SatisfiedBy:     "fallback_asset",
		SourceAssetIDs:  []string{"gallery-1"},
	}}
	selectedTasks := []assetgeneration.Task{{
		TaskID:          task.ID,
		ID:              "amazon:amazon-lifestyle",
		Platform:        "amazon",
		RecipeID:        "amazon-lifestyle",
		AssetKind:       asset.KindSceneImage,
		Slot:            "auxiliary",
		Purpose:         "scene",
		Status:          "planned",
		ExecutionStatus: "planned",
		ExecutionMode:   assetgeneration.PlannedExecutionMode(asset.KindSceneImage),
		CanExecute:      true,
		SourceAssetIDs:  []string{"gallery-1"},
	}}
	generation := newTaskGenerationService(taskGenerationServiceConfig{
		repo:                repo,
		assetRepo:           assetRepository,
		assetRecipeResolver: assetrecipe.NewStaticResolver(),
		assetBundleBuilder:  assetbundle.NewBuilder(),
		assetGenerator:      &stubRetryNilDispatchGenerator{},
		listAssetGenerationTasks: func(ctx context.Context, taskID string) ([]assetgeneration.Task, error) {
			return cloneGenerationTasks(persistedTasks), nil
		},
		listGenerationReviews: func(ctx context.Context, taskID string) ([]GenerationReviewRecord, error) {
			return nil, nil
		},
		buildRetryGenerationTaskSelection: func(ctx context.Context, task *Task, inventory *asset.Inventory, existing []assetgeneration.Task, req *RetryGenerationTasksRequest) ([]assetgeneration.Task, error) {
			return cloneGenerationTasks(selectedTasks), nil
		},
	})
	assetRepository.resetCalls()

	page, err := generation.RetryTaskGenerationTasks(context.Background(), task.ID, &RetryGenerationTasksRequest{
		Slots: []string{"auxiliary"},
	})
	if err != nil {
		t.Fatalf("RetryTaskGenerationTasks() error = %v", err)
	}
	if page == nil {
		t.Fatalf("page = nil, want safe retry page")
	}
	if len(page.Tasks) != 1 || page.Tasks[0].ID != "amazon:amazon-lifestyle" || page.Tasks[0].ExecutionStatus != "completed" {
		t.Fatalf("page tasks = %+v, want unchanged persisted task", page.Tasks)
	}
	if page.MatchedQueue == nil || page.MatchedQueue.Summary == nil || page.MatchedQueue.Summary.TotalItems != 1 {
		t.Fatalf("matched queue = %+v, want selected target still surfaced", page.MatchedQueue)
	}
	if page.ExecutedQueue == nil || page.ExecutedQueue.Summary == nil {
		t.Fatalf("executed queue = %+v, want empty safe queue", page.ExecutedQueue)
	}
	if page.ExecutedQueue.Summary.TotalItems != 0 || len(page.ExecutedQueue.Items) != 0 {
		t.Fatalf("executed queue = %+v, want empty queue for nil dispatch result", page.ExecutedQueue)
	}
	if !reflect.DeepEqual(assetRepository.calls, []string{"save_inventory", "save_generation_tasks"}) {
		t.Fatalf("persistence calls = %+v, want inventory/tasks save sequence even when dispatch result is nil", assetRepository.calls)
	}

	updatedInventory, err := assetRepository.GetInventory(context.Background(), asset.InventoryRef{TaskID: task.ID})
	if err != nil {
		t.Fatalf("GetInventory() error = %v", err)
	}
	wantRecordIDs := []string{"gallery-1", "scene-stub-safe-1"}
	recordIDs := make([]string, 0, len(updatedInventory.Records))
	for _, item := range updatedInventory.Records {
		recordIDs = append(recordIDs, item.ID)
	}
	if !reflect.DeepEqual(recordIDs, wantRecordIDs) {
		t.Fatalf("inventory record ids = %+v, want unchanged %+v", recordIDs, wantRecordIDs)
	}
	if updatedInventory.Summary == nil || updatedInventory.Summary.TotalRecords != 2 || updatedInventory.Summary.DerivedRecords != 1 || updatedInventory.Summary.GeneratedRecords != 1 || updatedInventory.Summary.RecipeCount != 1 {
		t.Fatalf("inventory summary = %+v, want unchanged counts", updatedInventory.Summary)
	}
}

func TestRetryTaskGenerationTasksEmptySelectionSkipsPersistence(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	assetRepository := newRecordingRetryPersistenceAssetRepo()
	task := &Task{
		ID:        "task-generation-retry-empty-selection-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"amazon"}},
		Result: &ListingKitResult{
			TaskID:           "task-generation-retry-empty-selection-1",
			Platforms:        []string{"amazon"},
			CanonicalProduct: &canonical.Product{CategoryPath: []string{"Electronics", "Audio"}},
			CatalogProduct:   &catalog.Product{Title: "Portable Speaker", CategoryPath: []string{"Electronics", "Audio"}},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if err := assetRepository.SaveInventory(context.Background(), &asset.Inventory{
		Ref:     asset.InventoryRef{TaskID: task.ID},
		Summary: &asset.InventorySummary{},
	}); err != nil {
		t.Fatalf("SaveInventory() error = %v", err)
	}

	generation := newTaskGenerationService(taskGenerationServiceConfig{
		repo:                repo,
		assetRepo:           assetRepository,
		assetRecipeResolver: assetrecipe.NewStaticResolver(),
		assetBundleBuilder:  assetbundle.NewBuilder(),
		assetGenerator:      &stubRetryNilDispatchGenerator{},
		listAssetGenerationTasks: func(ctx context.Context, taskID string) ([]assetgeneration.Task, error) {
			return nil, nil
		},
		listGenerationReviews: func(ctx context.Context, taskID string) ([]GenerationReviewRecord, error) {
			return nil, nil
		},
		buildRetryGenerationTaskSelection: func(ctx context.Context, task *Task, inventory *asset.Inventory, existing []assetgeneration.Task, req *RetryGenerationTasksRequest) ([]assetgeneration.Task, error) {
			return nil, nil
		},
	})
	assetRepository.resetCalls()

	page, err := generation.RetryTaskGenerationTasks(context.Background(), task.ID, &RetryGenerationTasksRequest{})
	if err != nil {
		t.Fatalf("RetryTaskGenerationTasks() error = %v", err)
	}
	if page == nil {
		t.Fatalf("page = nil, want empty retry page")
	}
	if len(assetRepository.calls) != 0 {
		t.Fatalf("persistence calls = %+v, want no persistence when selection is empty", assetRepository.calls)
	}
	if page.MatchedQueue == nil || page.MatchedQueue.Summary == nil || page.MatchedQueue.Summary.TotalItems != 0 {
		t.Fatalf("matched queue = %+v, want empty summary", page.MatchedQueue)
	}
	if page.ExecutedQueue == nil || page.ExecutedQueue.Summary == nil || page.ExecutedQueue.Summary.TotalItems != 0 {
		t.Fatalf("executed queue = %+v, want empty summary", page.ExecutedQueue)
	}
}

func TestRetryTaskGenerationTasksCanFilterFallbackSlotsOnly(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	assetRepository := assetrepo.NewMemRepository()
	svc := &service{
		repo:                repo,
		assetRepo:           assetRepository,
		assetRecipeResolver: assetrecipe.NewStaticResolver(),
		assetBundleBuilder:  assetbundle.NewBuilder(),
		assetGenerator:      assetgeneration.NewService(assetgeneration.Config{}),
	}

	task := &Task{
		ID:        "task-generation-retry-filter-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"shein"}},
		Result:    &ListingKitResult{TaskID: "task-generation-retry-filter-1", CatalogProduct: &catalog.Product{Title: "Tee"}},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if err := assetRepository.SaveInventory(context.Background(), &asset.Inventory{
		Ref: asset.InventoryRef{TaskID: task.ID},
	}); err != nil {
		t.Fatalf("SaveInventory() error = %v", err)
	}
	persistedTasks := []assetgeneration.Task{
		{
			TaskID:          task.ID,
			ID:              "shein:shein-main-model",
			Platform:        "shein",
			RecipeID:        "shein-main-model",
			AssetKind:       asset.KindModelImage,
			Slot:            "main",
			Purpose:         "main",
			ExecutionStatus: "completed",
			ExecutionMode:   assetgeneration.ExecutionModeDeferredStub,
			CanExecute:      true,
			SatisfiedBy:     "fallback_asset",
			FallbackFrom:    string(asset.KindModelImage),
		},
		{
			TaskID:          task.ID,
			ID:              "shein:shein-gallery-scene",
			Platform:        "shein",
			RecipeID:        "shein-gallery-scene",
			AssetKind:       asset.KindSceneImage,
			Slot:            "gallery",
			Purpose:         "gallery",
			ExecutionStatus: "completed",
			ExecutionMode:   assetgeneration.ExecutionModeRendererBacked,
			CanExecute:      true,
			SatisfiedBy:     assetgeneration.ExecutionModeGeneratedAsset,
		},
	}
	if err := assetRepository.SaveGenerationTasks(context.Background(), task.ID, persistedTasks); err != nil {
		t.Fatalf("SaveGenerationTasks() error = %v", err)
	}

	page, err := svc.RetryTaskGenerationTasks(context.Background(), task.ID, &RetryGenerationTasksRequest{
		FallbackOnly: true,
		Slots:        []string{"main"},
	})
	if err != nil {
		t.Fatalf("RetryTaskGenerationTasks() error = %v", err)
	}
	if len(page.Tasks) != 2 {
		t.Fatalf("tasks = %+v, want 2", page.Tasks)
	}
	if page.Tasks[0].ExecutionStatus != "planned" {
		t.Fatalf("main task = %+v, want planned for retry", page.Tasks[0])
	}
	if page.Tasks[1].ExecutionStatus != "completed" {
		t.Fatalf("gallery task = %+v, want untouched completed task", page.Tasks[1])
	}
}

func TestRetryTaskGenerationTasksPlansMissingQueueFallbackSlot(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	assetRepository := assetrepo.NewMemRepository()
	renderer := &stubServiceDeferredRenderer{
		result: &asset.AssetRecord{
			ID:       "scene-rendered-gallery-1",
			Kind:     asset.KindSceneImage,
			Origin:   asset.OriginGenerated,
			Role:     "scene",
			URL:      "file:///tmp/scene-rendered-gallery.jpg",
			RecipeID: "shein-gallery-scene",
			Metadata: map[string]string{
				"renderer":    "service-test",
				"bundle_slot": "gallery",
			},
		},
	}
	svc := &service{
		repo:                repo,
		assetRepo:           assetRepository,
		assetRecipeResolver: assetrecipe.NewStaticResolver(),
		assetBundleBuilder:  assetbundle.NewBuilder(),
		assetGenerator: assetgeneration.NewService(assetgeneration.Config{
			DeferredRenderer: renderer,
		}),
	}

	task := &Task{
		ID:        "task-generation-retry-plan-missing-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"shein"}},
		Result: &ListingKitResult{
			TaskID:           "task-generation-retry-plan-missing-1",
			Platforms:        []string{"shein"},
			CanonicalProduct: &canonical.Product{CategoryPath: []string{"Home", "Cushions"}},
			CatalogProduct:   &catalog.Product{Title: "Bench Cushion", CategoryPath: []string{"Home", "Cushions"}},
			Shein: &SheinPackage{ImageBundle: &common.PublishImageBundle{
				Platform: "shein",
				Gallery: []common.BundleSlot{{
					Key:             "gallery",
					Purpose:         "gallery",
					RecipeID:        "shein-gallery-scene",
					IdealKind:       string(asset.KindSceneImage),
					TemplateLabel:   "SHEIN Lifestyle Gallery",
					StateLabel:      "fallback_in_use",
					SatisfiedBy:     "fallback_asset",
					ExecutionStatus: "fallback",
					AssetID:         "gallery-1",
				}},
			}},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if err := assetRepository.SaveInventory(context.Background(), &asset.Inventory{
		Ref: asset.InventoryRef{TaskID: task.ID},
		Records: []asset.AssetRecord{
			{
				ID:     "gallery-1",
				TaskID: task.ID,
				Kind:   asset.KindSceneImage,
				Origin: asset.OriginDerived,
				URL:    "file:///tmp/gallery-fallback.jpg",
			},
		},
		Summary: &asset.InventorySummary{TotalRecords: 1},
	}); err != nil {
		t.Fatalf("SaveInventory() error = %v", err)
	}

	page, err := svc.RetryTaskGenerationTasks(context.Background(), task.ID, &RetryGenerationTasksRequest{
		FallbackOnly: true,
		Slots:        []string{"gallery"},
	})
	if err != nil {
		t.Fatalf("RetryTaskGenerationTasks() error = %v", err)
	}
	if len(page.Tasks) != 1 {
		t.Fatalf("tasks = %+v, want one planned-and-executed gallery task", page.Tasks)
	}
	if page.Tasks[0].RecipeID != "shein-gallery-scene" || page.Tasks[0].Slot != "gallery" {
		t.Fatalf("task = %+v, want shein-gallery-scene/gallery", page.Tasks[0])
	}
	if page.Tasks[0].ExecutionMode != assetgeneration.ExecutionModeRendererBacked || page.Tasks[0].ExecutionStatus != "completed" {
		t.Fatalf("task = %+v, want completed renderer-backed gallery task", page.Tasks[0])
	}
	if page.ExecutedQueue == nil || page.ExecutedQueue.Summary == nil || page.ExecutedQueue.Summary.TotalItems == 0 {
		t.Fatalf("executed queue = %+v, want executed gallery queue items", page.ExecutedQueue)
	}
	foundCompletedGallery := false
	for _, item := range page.ExecutedQueue.Items {
		if item.Slot == "gallery" && item.ExecutionMode == assetgeneration.ExecutionModeRendererBacked && item.ExecutionState == "completed" {
			foundCompletedGallery = true
			break
		}
	}
	if !foundCompletedGallery {
		t.Fatalf("executed queue items = %+v, want completed renderer-backed gallery item", page.ExecutedQueue.Items)
	}
}

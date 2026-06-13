package listingkit

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
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

type stubRetryDispatchGenerator struct {
	dispatchResult *assetgeneration.Result
	dispatchErr    error
}

type recordingRetryPersistenceServiceRepo struct {
	delegate            *stubGenerationRepo
	saveTaskResultCalls int
}

type retryPersistenceFailureFixture struct {
	taskID          string
	repo            *recordingRetryPersistenceServiceRepo
	assetRepository *recordingRetryPersistenceAssetRepo
	generation      *taskGenerationService
}

type sequencedTaskSnapshotsRepo struct {
	snapshots []*Task
	getCalls  int
}

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

func (r *recordingRetryPersistenceServiceRepo) CreateTask(ctx context.Context, task *Task) error {
	return r.delegate.CreateTask(ctx, task)
}

func (r *recordingRetryPersistenceServiceRepo) GetTask(ctx context.Context, taskID string) (*Task, error) {
	return r.delegate.GetTask(ctx, taskID)
}

func (r *recordingRetryPersistenceServiceRepo) ListTasks(ctx context.Context, query *TaskListQuery) ([]Task, int64, error) {
	return r.delegate.ListTasks(ctx, query)
}

func (r *recordingRetryPersistenceServiceRepo) MarkProcessing(ctx context.Context, taskID string) error {
	return r.delegate.MarkProcessing(ctx, taskID)
}

func (r *recordingRetryPersistenceServiceRepo) MarkCompleted(ctx context.Context, taskID string, result *ListingKitResult) error {
	return r.delegate.MarkCompleted(ctx, taskID, result)
}

func (r *recordingRetryPersistenceServiceRepo) MarkNeedsReview(ctx context.Context, taskID string, result *ListingKitResult, reason string) error {
	return r.delegate.MarkNeedsReview(ctx, taskID, result, reason)
}

func (r *recordingRetryPersistenceServiceRepo) MarkFailed(ctx context.Context, taskID string, errorMsg string) error {
	return r.delegate.MarkFailed(ctx, taskID, errorMsg)
}

func (r *recordingRetryPersistenceServiceRepo) MarkBlockedRetryable(ctx context.Context, taskID string, block *RetryableBlock, errorMsg string) error {
	return r.delegate.MarkBlockedRetryable(ctx, taskID, block, errorMsg)
}

func (r *recordingRetryPersistenceServiceRepo) ListRecoverableTasks(ctx context.Context, query *RecoverableTaskQuery) ([]Task, error) {
	return r.delegate.ListRecoverableTasks(ctx, query)
}

func (r *recordingRetryPersistenceServiceRepo) RecoverBlockedTaskNow(ctx context.Context, taskID string, recoveredAt time.Time) error {
	return r.delegate.RecoverBlockedTaskNow(ctx, taskID, recoveredAt)
}

func (r *recordingRetryPersistenceServiceRepo) BulkRecoverBlockedTasks(ctx context.Context, query *RecoverBlockedTasksQuery) (int64, error) {
	return r.delegate.BulkRecoverBlockedTasks(ctx, query)
}

func (r *recordingRetryPersistenceServiceRepo) PrepareRetry(ctx context.Context, taskID string) error {
	return r.delegate.PrepareRetry(ctx, taskID)
}

func (r *recordingRetryPersistenceServiceRepo) IncrementRetryCount(ctx context.Context, taskID string) error {
	return r.delegate.IncrementRetryCount(ctx, taskID)
}

func (r *recordingRetryPersistenceServiceRepo) SaveTaskResult(ctx context.Context, taskID string, result *ListingKitResult) error {
	r.saveTaskResultCalls++
	return r.delegate.SaveTaskResult(ctx, taskID, result)
}

func (r *sequencedTaskSnapshotsRepo) CreateTask(ctx context.Context, task *Task) error {
	copied := *task
	r.snapshots = []*Task{&copied}
	r.getCalls = 0
	return nil
}

func (r *sequencedTaskSnapshotsRepo) GetTask(ctx context.Context, taskID string) (*Task, error) {
	if len(r.snapshots) == 0 {
		return nil, ErrTaskNotFound
	}
	index := r.getCalls
	if index >= len(r.snapshots) {
		index = len(r.snapshots) - 1
	}
	snapshot := r.snapshots[index]
	r.getCalls++
	if snapshot == nil || snapshot.ID != taskID {
		return nil, ErrTaskNotFound
	}
	copied := *snapshot
	return &copied, nil
}

func (r *sequencedTaskSnapshotsRepo) ListTasks(ctx context.Context, query *TaskListQuery) ([]Task, int64, error) {
	if len(r.snapshots) == 0 || r.snapshots[len(r.snapshots)-1] == nil {
		return nil, 0, nil
	}
	copied := *r.snapshots[len(r.snapshots)-1]
	return []Task{copied}, 1, nil
}

func (r *sequencedTaskSnapshotsRepo) MarkProcessing(ctx context.Context, taskID string) error {
	return nil
}

func (r *sequencedTaskSnapshotsRepo) MarkCompleted(ctx context.Context, taskID string, result *ListingKitResult) error {
	return r.SaveTaskResult(ctx, taskID, result)
}

func (r *sequencedTaskSnapshotsRepo) MarkNeedsReview(ctx context.Context, taskID string, result *ListingKitResult, reason string) error {
	return r.SaveTaskResult(ctx, taskID, result)
}

func (r *sequencedTaskSnapshotsRepo) MarkFailed(ctx context.Context, taskID string, errorMsg string) error {
	return nil
}

func (r *sequencedTaskSnapshotsRepo) MarkBlockedRetryable(ctx context.Context, taskID string, block *RetryableBlock, errorMsg string) error {
	if len(r.snapshots) == 0 {
		return ErrTaskNotFound
	}
	latest := r.snapshots[len(r.snapshots)-1]
	if latest == nil || latest.ID != taskID {
		return ErrTaskNotFound
	}
	copied := *latest
	copied.Status = TaskStatusBlockedRetryable
	copied.RetryableBlock = block
	copied.Error = errorMsg
	r.snapshots[len(r.snapshots)-1] = &copied
	return nil
}

func (r *sequencedTaskSnapshotsRepo) ListRecoverableTasks(context.Context, *RecoverableTaskQuery) ([]Task, error) {
	return []Task{}, nil
}

func (r *sequencedTaskSnapshotsRepo) RecoverBlockedTaskNow(_ context.Context, taskID string, recoveredAt time.Time) error {
	if len(r.snapshots) == 0 {
		return ErrTaskNotFound
	}
	latest := r.snapshots[len(r.snapshots)-1]
	if latest == nil || latest.ID != taskID {
		return ErrTaskNotFound
	}
	copied := *latest
	copied.Status = TaskStatusPending
	copied.RetryableBlock = nil
	copied.Error = ""
	copied.UpdatedAt = recoveredAt
	r.snapshots[len(r.snapshots)-1] = &copied
	return nil
}

func (r *sequencedTaskSnapshotsRepo) BulkRecoverBlockedTasks(context.Context, *RecoverBlockedTasksQuery) (int64, error) {
	return 0, nil
}

func (r *sequencedTaskSnapshotsRepo) PrepareRetry(ctx context.Context, taskID string) error {
	return nil
}

func (r *sequencedTaskSnapshotsRepo) IncrementRetryCount(ctx context.Context, taskID string) error {
	return nil
}

func (r *sequencedTaskSnapshotsRepo) SaveTaskResult(ctx context.Context, taskID string, result *ListingKitResult) error {
	if len(r.snapshots) == 0 {
		return ErrTaskNotFound
	}
	latest := r.snapshots[len(r.snapshots)-1]
	if latest == nil || latest.ID != taskID {
		return ErrTaskNotFound
	}
	copiedTask := *latest
	if result != nil {
		copiedResult := *result
		copiedTask.Result = &copiedResult
	} else {
		copiedTask.Result = nil
	}
	r.snapshots[len(r.snapshots)-1] = &copiedTask
	return nil
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

func (s *stubRetryDispatchGenerator) Plan(ctx context.Context, req assetgeneration.Request) (*assetgeneration.Result, error) {
	return &assetgeneration.Result{}, nil
}

func (s *stubRetryDispatchGenerator) Execute(ctx context.Context, req assetgeneration.Request) (*assetgeneration.Result, error) {
	return &assetgeneration.Result{}, nil
}

func (s *stubRetryDispatchGenerator) Dispatch(ctx context.Context, req assetgeneration.DispatchRequest) (*assetgeneration.Result, error) {
	return s.dispatchResult, s.dispatchErr
}

func newRetryPersistenceFailureFixture(t *testing.T, taskID string) *retryPersistenceFailureFixture {
	t.Helper()

	repo := &recordingRetryPersistenceServiceRepo{delegate: &stubGenerationRepo{}}
	assetRepository := newRecordingRetryPersistenceAssetRepo()
	task := &Task{
		ID:        taskID,
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"amazon"}},
		Result: &ListingKitResult{
			TaskID:           taskID,
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
		Ref: asset.InventoryRef{TaskID: taskID},
		Records: []asset.AssetRecord{
			{ID: "gallery-1", TaskID: taskID, Kind: asset.KindGalleryImage, Origin: asset.OriginDerived, URL: "file:///tmp/gallery.jpg"},
			{ID: "scene-stub-" + taskID, TaskID: taskID, Kind: asset.KindSceneImage, Origin: asset.OriginGenerated, URL: "file:///tmp/scene-stub-" + taskID + ".jpg", RecipeID: "amazon-lifestyle", Metadata: map[string]string{"execution_mode": assetgeneration.ExecutionModeDeferredStub, "bundle_slot": "auxiliary"}},
		},
		Summary: &asset.InventorySummary{TotalRecords: 2, DerivedRecords: 1, GeneratedRecords: 1, RecipeCount: 1},
	}
	if err := assetRepository.SaveInventory(context.Background(), inventory); err != nil {
		t.Fatalf("SaveInventory() error = %v", err)
	}
	persistedTasks := []assetgeneration.Task{{
		TaskID:          taskID,
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
	if err := assetRepository.SaveGenerationTasks(context.Background(), taskID, persistedTasks); err != nil {
		t.Fatalf("SaveGenerationTasks() error = %v", err)
	}
	selectedTasks := []assetgeneration.Task{{
		TaskID:          taskID,
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
		assetGenerator: &stubRetryDispatchGenerator{
			dispatchResult: &assetgeneration.Result{
				Tasks: []assetgeneration.Task{{
					TaskID:          taskID,
					ID:              "amazon:amazon-lifestyle",
					Platform:        "amazon",
					RecipeID:        "amazon-lifestyle",
					AssetKind:       asset.KindSceneImage,
					Slot:            "auxiliary",
					Purpose:         "scene",
					Status:          "completed",
					ExecutionStatus: "completed",
					ExecutionMode:   assetgeneration.ExecutionModeRendererBacked,
					CanExecute:      true,
					SourceAssetIDs:  []string{"gallery-1"},
				}},
				Assets: []asset.AssetRecord{{
					ID:       "scene-rendered-" + taskID,
					TaskID:   taskID,
					Kind:     asset.KindSceneImage,
					Origin:   asset.OriginGenerated,
					URL:      "file:///tmp/scene-rendered-" + taskID + ".jpg",
					RecipeID: "amazon-lifestyle",
					Metadata: map[string]string{"bundle_slot": "auxiliary"},
				}},
			},
		},
		listAssetGenerationTasks: func(ctx context.Context, requestedTaskID string) ([]assetgeneration.Task, error) {
			return cloneGenerationTasks(persistedTasks), nil
		},
		listGenerationReviews: func(ctx context.Context, requestedTaskID string) ([]GenerationReviewRecord, error) {
			return nil, nil
		},
		buildRetryGenerationTaskSelection: func(ctx context.Context, task *Task, inventory *asset.Inventory, existing []assetgeneration.Task, req *RetryGenerationTasksRequest) ([]assetgeneration.Task, error) {
			return cloneGenerationTasks(selectedTasks), nil
		},
	})
	return &retryPersistenceFailureFixture{
		taskID:          taskID,
		repo:            repo,
		assetRepository: assetRepository,
		generation:      generation,
	}
}

func newTaskGenerationActionQueueFixture(t *testing.T, taskID string) (*Task, *taskGenerationService) {
	t.Helper()

	repo := &stubGenerationRepo{}
	task := &Task{
		ID:        taskID,
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"amazon"}},
		Result: &ListingKitResult{
			TaskID: taskID,
			Amazon: &AmazonPackage{ImageBundle: &common.PublishImageBundle{
				Platform: "amazon",
				MissingSlots: []common.MissingSlot{{
					Slot:          "auxiliary",
					Purpose:       "scene",
					RecipeID:      "amazon-lifestyle",
					TemplateLabel: "Amazon Lifestyle Scene",
					RenderProfile: "amazon_lifestyle_scene",
					StateLabel:    "missing",
				}},
			}},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	return task, newTaskGenerationService(taskGenerationServiceConfig{
		repo: repo,
		listAssetGenerationTasks: func(ctx context.Context, requestedTaskID string) ([]assetgeneration.Task, error) {
			return nil, nil
		},
		listGenerationReviews: func(ctx context.Context, requestedTaskID string) ([]GenerationReviewRecord, error) {
			return nil, nil
		},
	})
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
		repo: repo, mirrors: serviceDependencyMirrors{assetRepo: assetRepository, assetRecipeResolver: assetrecipe.NewStaticResolver(), assetBundleBuilder: assetbundle.NewBuilder(), assetGenerator: assetgeneration.NewService(assetgeneration.Config{
			DeferredRenderer: renderer,
		})},
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
		repo: repo, mirrors: serviceDependencyMirrors{assetRepo: assetRepository, assetRecipeResolver: assetrecipe.NewStaticResolver(), assetBundleBuilder: assetbundle.NewBuilder(), assetGenerator: assetgeneration.NewService(assetgeneration.Config{})},
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
		repo: repo, mirrors: serviceDependencyMirrors{assetRepo: assetRepository, assetRecipeResolver: assetrecipe.NewStaticResolver(), assetBundleBuilder: assetbundle.NewBuilder(), assetGenerator: assetgeneration.NewService(assetgeneration.Config{})},
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
		repo: repo, mirrors: serviceDependencyMirrors{assetRepo: assetRepository, assetRecipeResolver: assetrecipe.NewStaticResolver(), assetBundleBuilder: assetbundle.NewBuilder(), assetGenerator: assetgeneration.NewService(assetgeneration.Config{})},
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
		repo: repo, mirrors: serviceDependencyMirrors{assetRepo: assetRepository, assetRecipeResolver: assetrecipe.NewStaticResolver(), assetBundleBuilder: assetbundle.NewBuilder(), assetGenerator: assetgeneration.NewService(assetgeneration.Config{})},
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
		repo: repo, mirrors: serviceDependencyMirrors{assetRepo: assetRepository, assetRecipeResolver: assetrecipe.NewStaticResolver(), assetBundleBuilder: assetbundle.NewBuilder(), assetGenerator: assetgeneration.NewService(assetgeneration.Config{})},
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
		repo: repo, mirrors: serviceDependencyMirrors{assetRepo: assetRepository, assetRecipeResolver: assetrecipe.NewStaticResolver(), assetBundleBuilder: assetbundle.NewBuilder(), assetGenerator: assetgeneration.NewService(assetgeneration.Config{
			DeferredRenderer: renderer,
		})},
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
		repo: repo, mirrors: serviceDependencyMirrors{assetRepo: assetRepository, assetRecipeResolver: assetrecipe.NewStaticResolver(), assetBundleBuilder: assetbundle.NewBuilder(), assetGenerator: assetgeneration.NewService(assetgeneration.Config{
			DeferredRenderer: renderer,
		})},
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

func TestRetryGenerationResultProjectionRebuildsListingKitResult(t *testing.T) {
	t.Parallel()

	reviewedAt := time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC)
	task := &Task{
		ID:        "task-generation-retry-projection-1",
		UpdatedAt: reviewedAt,
		Request:   &GenerateRequest{Platforms: []string{"amazon"}},
		Result: &ListingKitResult{
			TaskID:           "task-generation-retry-projection-1",
			Platforms:        []string{"amazon"},
			CanonicalProduct: &canonical.Product{CategoryPath: []string{"Electronics", "Audio"}},
			CatalogProduct:   &catalog.Product{Title: "Portable Speaker", CategoryPath: []string{"Electronics", "Audio"}},
			AssetBundle: &asset.Bundle{
				Assets: []asset.Asset{
					{ID: "gallery-1", Kind: asset.KindGalleryImage, URL: "file:///tmp/gallery.jpg"},
					{ID: "scene-stub-1", Kind: asset.KindSceneImage, URL: "file:///tmp/scene-stub.jpg", RecipeID: "amazon-lifestyle", Metadata: map[string]string{"bundle_slot": "auxiliary"}},
				},
			},
			AssetInventorySummary: &asset.InventorySummary{TotalRecords: 2, DerivedRecords: 1, GeneratedRecords: 1, RecipeCount: 1},
			AssetRenderPreviews: []AssetRenderPreview{{
				AssetID:         "scene-rendered-1",
				AssetRevision:   "asset-rev-1",
				PreviewRevision: "preview-rev-1",
				Kind:            asset.KindSceneImage,
				Role:            "scene",
				LayerTypes:      []string{"subject"},
			}},
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
						AssetID:         "scene-stub-1",
					}},
				},
			},
		},
	}
	inventory := &asset.Inventory{
		Ref: asset.InventoryRef{TaskID: task.ID},
		Records: []asset.AssetRecord{
			{ID: "gallery-1", TaskID: task.ID, Kind: asset.KindGalleryImage, Origin: asset.OriginDerived, URL: "file:///tmp/gallery.jpg"},
			{
				ID:       "scene-rendered-1",
				TaskID:   task.ID,
				Kind:     asset.KindSceneImage,
				Origin:   asset.OriginGenerated,
				URL:      "https://cdn.example.com/scene-rendered-1.jpg",
				RecipeID: "amazon-lifestyle",
				Metadata: map[string]string{"bundle_slot": "auxiliary", "published_url": "https://cdn.example.com/scene-rendered-1.jpg"},
			},
		},
		Summary: &asset.InventorySummary{TotalRecords: 2, DerivedRecords: 1, GeneratedRecords: 1, RecipeCount: 1},
	}
	updatedTasks := []assetgeneration.Task{
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
			ExecutionMode:   assetgeneration.ExecutionModeRendererBacked,
			CanExecute:      true,
			SatisfiedBy:     assetgeneration.ExecutionModeGeneratedAsset,
			SourceAssetIDs:  []string{"gallery-1"},
		},
	}
	selectedTasks := cloneGenerationTasks(updatedTasks)
	dispatchResult := &assetgeneration.Result{Tasks: cloneGenerationTasks(updatedTasks)}
	reviews := []GenerationReviewRecord{{
		TaskID:       task.ID,
		Platform:     "amazon",
		Slot:         "auxiliary",
		Capability:   "subject_preview",
		Decision:     GenerationReviewDecisionApprove,
		Status:       "approved",
		ReviewedAt:   reviewedAt,
		ReviewedBy:   "reviewer",
		AssetID:      "scene-rendered-1",
		TaskRevision: "",
	}}

	result, page := buildRetryGenerationProjectionPhase(assetrecipe.NewStaticResolver(), assetbundle.NewBuilder()).run(
		task,
		inventory,
		updatedTasks,
		selectedTasks,
		dispatchResult,
		reviews,
	)

	if result == nil {
		t.Fatalf("result = nil, want rebuilt listing kit result")
	}
	if page == nil {
		t.Fatalf("page = nil, want generation task page")
	}
	if result.AssetBundle == nil || len(result.AssetBundle.Assets) != 2 {
		t.Fatalf("asset bundle = %+v, want rebuilt bundle with inventory assets", result.AssetBundle)
	}
	if result.AssetBundle.Assets[1].ID != "scene-rendered-1" {
		t.Fatalf("asset bundle assets = %+v, want rendered asset in rebuilt bundle", result.AssetBundle.Assets)
	}
	if !reflect.DeepEqual(result.AssetInventorySummary, inventory.Summary) {
		t.Fatalf("asset inventory summary = %+v, want %+v", result.AssetInventorySummary, inventory.Summary)
	}
	if result.Amazon == nil || result.Amazon.ImageBundle == nil || len(result.Amazon.ImageBundle.Auxiliary) != 1 {
		t.Fatalf("amazon image bundle = %+v, want rebuilt platform bundle", result.Amazon)
	}
	auxiliary := result.Amazon.ImageBundle.Auxiliary[0]
	if auxiliary.AssetID != "scene-rendered-1" || auxiliary.URL != "https://cdn.example.com/scene-rendered-1.jpg" {
		t.Fatalf("amazon auxiliary slot = %+v, want rendered asset rebound from inventory", auxiliary)
	}
	if result.AssetGenerationSummary == nil || result.AssetGenerationSummary.TotalTasks != 1 || result.AssetGenerationSummary.CompletedTasks != 1 {
		t.Fatalf("asset generation summary = %+v, want completed updated task summary", result.AssetGenerationSummary)
	}
	if result.AssetGenerationQueue == nil || result.AssetGenerationQueue.Summary == nil {
		t.Fatalf("asset generation queue = %+v, want decorated queue", result.AssetGenerationQueue)
	}
	if result.AssetGenerationOverview == nil || result.AssetGenerationOverview.PrimaryActionKey == "" {
		t.Fatalf("asset generation overview = %+v, want decorated overview", result.AssetGenerationOverview)
	}
	if len(result.PlatformAssetRenderPreviews) != 1 || len(result.PlatformAssetRenderPreviews[0].Auxiliary) != 1 {
		t.Fatalf("platform previews = %+v, want synced previews", result.PlatformAssetRenderPreviews)
	}
	if result.PlatformAssetRenderPreviews[0].Auxiliary[0].AssetID != "scene-rendered-1" {
		t.Fatalf("platform preview slot = %+v, want preview synced to rendered asset", result.PlatformAssetRenderPreviews[0].Auxiliary[0])
	}
	if result.ReviewSummary == nil || result.ReviewSummary.ApprovedSections != 1 || result.ReviewSummary.ReviewPendingSections != 0 {
		t.Fatalf("review summary = %+v, want review decoration applied", result.ReviewSummary)
	}
	if page.MatchedQueue == nil || page.MatchedQueue.Summary == nil {
		t.Fatalf("matched queue = %+v, want selected task queue summary", page.MatchedQueue)
	}
	if page.ExecutedQueue == nil || page.ExecutedQueue.Summary == nil {
		t.Fatalf("executed queue = %+v, want dispatch task queue summary", page.ExecutedQueue)
	}
	foundMatchedAuxiliary := false
	for _, item := range page.MatchedQueue.Items {
		if item.RecipeID == "amazon-lifestyle" && item.Slot == "auxiliary" {
			foundMatchedAuxiliary = true
			break
		}
	}
	if !foundMatchedAuxiliary {
		t.Fatalf("matched queue items = %+v, want auxiliary retry item", page.MatchedQueue.Items)
	}
	foundExecutedAuxiliary := false
	for _, item := range page.ExecutedQueue.Items {
		if item.RecipeID == "amazon-lifestyle" && item.Slot == "auxiliary" {
			foundExecutedAuxiliary = true
			break
		}
	}
	if !foundExecutedAuxiliary {
		t.Fatalf("executed queue items = %+v, want executed auxiliary item", page.ExecutedQueue.Items)
	}
}

func TestRetryGenerationResultProjectionBuildsQueues(t *testing.T) {
	t.Parallel()

	task := &Task{
		ID:        "task-generation-retry-projection-queues-1",
		UpdatedAt: time.Date(2026, 5, 30, 10, 5, 0, 0, time.UTC),
		Request:   &GenerateRequest{Platforms: []string{"amazon"}},
		Result: &ListingKitResult{
			TaskID:           "task-generation-retry-projection-queues-1",
			Platforms:        []string{"amazon"},
			CanonicalProduct: &canonical.Product{CategoryPath: []string{"Electronics", "Audio"}},
			CatalogProduct:   &catalog.Product{Title: "Portable Speaker", CategoryPath: []string{"Electronics", "Audio"}},
			Amazon:           &AmazonPackage{},
		},
	}
	inventory := &asset.Inventory{
		Ref: asset.InventoryRef{TaskID: task.ID},
		Records: []asset.AssetRecord{
			{ID: "gallery-1", TaskID: task.ID, Kind: asset.KindGalleryImage, Origin: asset.OriginDerived, URL: "file:///tmp/gallery.jpg"},
			{ID: "scene-rendered-queue-1", TaskID: task.ID, Kind: asset.KindSceneImage, Origin: asset.OriginGenerated, URL: "https://cdn.example.com/scene-rendered-queue-1.jpg", RecipeID: "amazon-lifestyle", Metadata: map[string]string{"bundle_slot": "auxiliary", "published_url": "https://cdn.example.com/scene-rendered-queue-1.jpg"}},
			{ID: "gallery-rendered-queue-1", TaskID: task.ID, Kind: asset.KindSceneImage, Origin: asset.OriginGenerated, URL: "https://cdn.example.com/gallery-rendered-queue-1.jpg", RecipeID: "amazon-gallery-scene", Metadata: map[string]string{"bundle_slot": "gallery", "published_url": "https://cdn.example.com/gallery-rendered-queue-1.jpg"}},
		},
		Summary: &asset.InventorySummary{TotalRecords: 3, DerivedRecords: 1, GeneratedRecords: 2, RecipeCount: 2},
	}
	updatedTasks := []assetgeneration.Task{
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
			ExecutionMode:   assetgeneration.ExecutionModeRendererBacked,
			CanExecute:      true,
			SatisfiedBy:     assetgeneration.ExecutionModeGeneratedAsset,
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
			Status:          "planned",
			ExecutionStatus: "planned",
			ExecutionMode:   assetgeneration.PlannedExecutionMode(asset.KindSceneImage),
			CanExecute:      true,
			SourceAssetIDs:  []string{"gallery-1"},
		},
	}
	selectedTasks := cloneGenerationTasks(updatedTasks)
	dispatchResult := &assetgeneration.Result{
		Tasks: []assetgeneration.Task{updatedTasks[0]},
	}

	result, page := buildRetryGenerationProjectionPhase(assetrecipe.NewStaticResolver(), assetbundle.NewBuilder()).run(
		task,
		inventory,
		updatedTasks,
		selectedTasks,
		dispatchResult,
		nil,
	)

	if result == nil || page == nil {
		t.Fatalf("phase output = (%+v, %+v), want result and page", result, page)
	}
	if page.Total != len(updatedTasks) || len(page.Tasks) != len(updatedTasks) {
		t.Fatalf("page tasks = %+v, want full updated task page", page.Tasks)
	}
	if page.MatchedQueue == nil || page.MatchedQueue.Summary == nil {
		t.Fatalf("matched queue = %+v, want selected tasks represented", page.MatchedQueue)
	}
	matchedRecipes := map[string]bool{}
	for _, item := range page.MatchedQueue.Items {
		matchedRecipes[item.RecipeID] = true
	}
	if !matchedRecipes["amazon-lifestyle"] || !matchedRecipes["amazon-gallery-scene"] {
		t.Fatalf("matched queue items = %+v, want both selected recipe ids", page.MatchedQueue.Items)
	}
	if page.ExecutedQueue == nil || page.ExecutedQueue.Summary == nil {
		t.Fatalf("executed queue = %+v, want dispatch tasks represented", page.ExecutedQueue)
	}
	for _, item := range page.ExecutedQueue.Items {
		if item.RecipeID != "amazon-lifestyle" {
			t.Fatalf("executed queue items = %+v, want only dispatched auxiliary recipe ids", page.ExecutedQueue.Items)
		}
	}
	if result.AssetGenerationQueue == nil || result.AssetGenerationQueue.Summary == nil {
		t.Fatalf("result queue = %+v, want queue from updated tasks", result.AssetGenerationQueue)
	}
}

func TestRetryGenerationResultProjectionHandlesNilTaskRequest(t *testing.T) {
	t.Parallel()

	task := &Task{
		ID:        "task-generation-retry-projection-nil-request-1",
		UpdatedAt: time.Date(2026, 5, 30, 10, 10, 0, 0, time.UTC),
		Request:   nil,
		Result: &ListingKitResult{
			TaskID:           "task-generation-retry-projection-nil-request-1",
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
						AssetID:         "scene-stub-nil-request-1",
					}},
				},
			},
		},
	}
	inventory := &asset.Inventory{
		Ref: asset.InventoryRef{TaskID: task.ID},
		Records: []asset.AssetRecord{
			{ID: "gallery-1", TaskID: task.ID, Kind: asset.KindGalleryImage, Origin: asset.OriginDerived, URL: "file:///tmp/gallery.jpg"},
			{ID: "scene-rendered-nil-request-1", TaskID: task.ID, Kind: asset.KindSceneImage, Origin: asset.OriginGenerated, URL: "https://cdn.example.com/scene-rendered-nil-request-1.jpg", RecipeID: "amazon-lifestyle", Metadata: map[string]string{"bundle_slot": "auxiliary", "published_url": "https://cdn.example.com/scene-rendered-nil-request-1.jpg"}},
		},
		Summary: &asset.InventorySummary{TotalRecords: 2, DerivedRecords: 1, GeneratedRecords: 1, RecipeCount: 1},
	}
	updatedTasks := []assetgeneration.Task{{
		TaskID:          task.ID,
		ID:              "amazon:amazon-lifestyle",
		Platform:        "amazon",
		RecipeID:        "amazon-lifestyle",
		AssetKind:       asset.KindSceneImage,
		Slot:            "auxiliary",
		Purpose:         "scene",
		Status:          "completed",
		ExecutionStatus: "completed",
		ExecutionMode:   assetgeneration.ExecutionModeRendererBacked,
		CanExecute:      true,
		SatisfiedBy:     assetgeneration.ExecutionModeGeneratedAsset,
		SourceAssetIDs:  []string{"gallery-1"},
	}}

	result, page := buildRetryGenerationProjectionPhase(assetrecipe.NewStaticResolver(), assetbundle.NewBuilder()).run(
		task,
		inventory,
		updatedTasks,
		cloneGenerationTasks(updatedTasks),
		&assetgeneration.Result{Tasks: cloneGenerationTasks(updatedTasks)},
		nil,
	)

	if result == nil || page == nil {
		t.Fatalf("phase output = (%+v, %+v), want rebuilt result and page", result, page)
	}
	if !reflect.DeepEqual(result.AssetInventorySummary, inventory.Summary) {
		t.Fatalf("asset inventory summary = %+v, want %+v", result.AssetInventorySummary, inventory.Summary)
	}
	if result.Amazon == nil || result.Amazon.ImageBundle == nil || len(result.Amazon.ImageBundle.Auxiliary) != 1 {
		t.Fatalf("amazon image bundle = %+v, want rebuilt bundle from persisted platform state", result.Amazon)
	}
	if result.Amazon.ImageBundle.Auxiliary[0].AssetID != "scene-rendered-nil-request-1" || result.Amazon.ImageBundle.Auxiliary[0].URL != "https://cdn.example.com/scene-rendered-nil-request-1.jpg" {
		t.Fatalf("amazon auxiliary slot = %+v, want rebuilt rendered asset when request is nil", result.Amazon.ImageBundle.Auxiliary[0])
	}
}

func TestTaskGenerationServiceFileDelegatesRetryProjection(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("task_generation_service.go")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	source := string(content)
	if !strings.Contains(source, "buildRetryGenerationProjectionPhase(s.assetRecipeResolver, s.assetBundleBuilder).run(") {
		t.Fatalf("task_generation_service.go should delegate retry projection through local phase helper")
	}
	if !strings.Contains(source, "if err := s.repo.SaveTaskResult(ctx, task.ID, rebuiltResult); err != nil {") {
		t.Fatalf("task_generation_service.go should keep SaveTaskResult at service call site")
	}
}

func TestTaskGenerationActionExecuteRunBranchesByInteractionMode(t *testing.T) {
	t.Parallel()

	t.Run("retryable_targets_use_retry_generation_tasks", func(t *testing.T) {
		t.Parallel()

		fixture := newRetryPersistenceFailureFixture(t, "task-generation-action-execute-retry-1")
		task, err := fixture.repo.GetTask(context.Background(), fixture.taskID)
		if err != nil {
			t.Fatalf("GetTask() error = %v", err)
		}

		originalBuildRetrySelection := fixture.generation.buildRetryGenerationTaskSelection
		var observedRetryRequest *RetryGenerationTasksRequest
		fixture.generation.buildRetryGenerationTaskSelection = func(ctx context.Context, task *Task, inventory *asset.Inventory, existing []assetgeneration.Task, req *RetryGenerationTasksRequest) ([]assetgeneration.Task, error) {
			observedRetryRequest = req
			selectedTasks, err := originalBuildRetrySelection(ctx, task, inventory, existing, req)
			if req != nil {
				req.QualityGrade = "mutated-by-downstream"
				if len(req.Slots) > 0 {
					req.Slots[0] = "mutated-slot"
				}
			}
			return selectedTasks, err
		}

		target := &AssetGenerationActionTarget{
			ActionKey:       "generate_missing_assets",
			InteractionMode: "retryable",
			QueueQuery: &GenerationQueueQuery{
				Platform: "amazon",
				Slot:     "auxiliary",
			},
			RetryRequest: &RetryGenerationTasksRequest{
				Slots:        []string{"auxiliary"},
				QualityGrade: "missing",
			},
		}
		originalRetryRequest := cloneRetryGenerationTasksRequest(target.RetryRequest)

		execution, err := buildTaskGenerationActionExecutePhase(fixture.generation).run(context.Background(), fixture.taskID, task.Result, target)
		if err != nil {
			t.Fatalf("taskGenerationActionExecutePhase.run() error = %v", err)
		}
		if observedRetryRequest == nil {
			t.Fatal("observed retry request = nil, want downstream retry request clone")
		}
		if observedRetryRequest == target.RetryRequest {
			t.Fatal("observed retry request reused original target.RetryRequest, want clone")
		}
		if !reflect.DeepEqual(target.RetryRequest, originalRetryRequest) {
			t.Fatalf("target.RetryRequest = %+v, want original request unchanged as %+v", target.RetryRequest, originalRetryRequest)
		}
		if execution == nil || execution.retryPage == nil {
			t.Fatalf("execution = %+v, want retry page", execution)
		}
		if execution.queuePage != nil {
			t.Fatalf("execution.queuePage = %+v, want nil for retryable path", execution.queuePage)
		}
		if execution.retryPage.ExecutedQueue == nil || execution.retryPage.ExecutedQueue.Summary == nil || execution.retryPage.ExecutedQueue.Summary.TotalItems != 1 {
			t.Fatalf("retry page = %+v, want executed retry queue", execution.retryPage)
		}
		if execution.persistenceSession == nil || execution.persistenceSession.Queue == nil {
			t.Fatalf("persistence session = %+v, want retry execution queue session", execution.persistenceSession)
		}
		if execution.persistenceSession.SelectedPlatform != "amazon" || execution.persistenceSession.SelectedSlot != "auxiliary" {
			t.Fatalf("persistence session selection = %+v, want amazon/auxiliary from retry execution", execution.persistenceSession)
		}
		if execution.persistenceSession.Queue.Summary == nil || execution.persistenceSession.Queue.Summary.TotalItems != execution.retryPage.ExecutedQueue.Summary.TotalItems {
			t.Fatalf("persistence session queue = %+v, want retry executed queue summary", execution.persistenceSession.Queue)
		}
		if execution.persistenceSession.Queue == execution.retryPage.ExecutedQueue {
			t.Fatal("persistence session queue reused retry page executed queue pointer, want page-derived review queue")
		}
	})

	t.Run("non_retryable_targets_use_generation_queue", func(t *testing.T) {
		t.Parallel()

		task, generation := newTaskGenerationActionQueueFixture(t, "task-generation-action-execute-queue-1")
		target := &AssetGenerationActionTarget{
			ActionKey:       "review_missing_slots",
			InteractionMode: "queue_only",
			QueueQuery: &GenerationQueueQuery{
				Platform:     "amazon",
				Slot:         "auxiliary",
				QualityGrade: "missing",
			},
		}
		originalQueueQuery := cloneGenerationQueueQuery(target.QueueQuery)

		execution, err := buildTaskGenerationActionExecutePhase(generation).run(context.Background(), task.ID, task.Result, target)
		if err != nil {
			t.Fatalf("taskGenerationActionExecutePhase.run() error = %v", err)
		}
		if !reflect.DeepEqual(target.QueueQuery, originalQueueQuery) {
			t.Fatalf("target.QueueQuery = %+v, want original query unchanged as %+v", target.QueueQuery, originalQueueQuery)
		}
		if execution == nil || execution.queuePage == nil {
			t.Fatalf("execution = %+v, want queue page", execution)
		}
		if execution.retryPage != nil {
			t.Fatalf("execution.retryPage = %+v, want nil for non-retryable path", execution.retryPage)
		}
		if execution.queuePage.Summary == nil || execution.queuePage.Summary.TotalItems != 1 {
			t.Fatalf("queue page = %+v, want missing-slot queue page", execution.queuePage)
		}
		if execution.persistenceSession == nil || execution.persistenceSession.Queue == nil {
			t.Fatalf("persistence session = %+v, want queue execution session", execution.persistenceSession)
		}
		if execution.persistenceSession.SelectedPlatform != "amazon" || execution.persistenceSession.SelectedSlot != "auxiliary" {
			t.Fatalf("persistence session selection = %+v, want amazon/auxiliary from queue execution", execution.persistenceSession)
		}
		if execution.persistenceSession.Queue.Summary == nil || execution.persistenceSession.Queue.Summary.TotalItems != execution.queuePage.Summary.TotalItems {
			t.Fatalf("persistence session queue = %+v, want queue page summary", execution.persistenceSession.Queue)
		}
	})
}

func TestTaskGenerationActionExecuteRequestHandoffRun(t *testing.T) {
	t.Parallel()

	t.Run("retryable_targets_return_retry_page_and_retry_derived_persistence_queue", func(t *testing.T) {
		t.Parallel()

		fixture := newRetryPersistenceFailureFixture(t, "task-generation-action-handoff-retry-1")
		originalBuildRetrySelection := fixture.generation.buildRetryGenerationTaskSelection
		var observedRetryRequest *RetryGenerationTasksRequest
		fixture.generation.buildRetryGenerationTaskSelection = func(ctx context.Context, task *Task, inventory *asset.Inventory, existing []assetgeneration.Task, req *RetryGenerationTasksRequest) ([]assetgeneration.Task, error) {
			observedRetryRequest = req
			selectedTasks, err := originalBuildRetrySelection(ctx, task, inventory, existing, req)
			if req != nil {
				req.QualityGrade = "mutated-by-handoff-downstream"
				if len(req.Slots) > 0 {
					req.Slots[0] = "mutated-slot"
				}
			}
			return selectedTasks, err
		}
		target := &AssetGenerationActionTarget{
			ActionKey:       "generate_missing_assets",
			InteractionMode: "retryable",
			RetryRequest: &RetryGenerationTasksRequest{
				Slots:        []string{"auxiliary"},
				QualityGrade: "missing",
			},
		}
		originalRetryRequest := cloneRetryGenerationTasksRequest(target.RetryRequest)

		handoff, err := buildTaskGenerationActionExecuteRequestHandoffPhase(fixture.generation).run(context.Background(), fixture.taskID, target)
		if err != nil {
			t.Fatalf("taskGenerationActionExecuteRequestHandoffPhase.run() error = %v", err)
		}
		if observedRetryRequest == nil {
			t.Fatal("observed retry request = nil, want downstream retry request clone")
		}
		if observedRetryRequest == target.RetryRequest {
			t.Fatal("observed retry request reused original target.RetryRequest, want clone")
		}
		if !reflect.DeepEqual(target.RetryRequest, originalRetryRequest) {
			t.Fatalf("target.RetryRequest = %+v, want original request unchanged as %+v", target.RetryRequest, originalRetryRequest)
		}
		if handoff == nil || handoff.retryPage == nil {
			t.Fatalf("handoff = %+v, want retry page handoff", handoff)
		}
		if handoff.queuePage != nil {
			t.Fatalf("handoff.queuePage = %+v, want nil for retryable path", handoff.queuePage)
		}

		wantQueue := generationWorkQueueFromRetryPage(handoff.retryPage)
		if handoff.persistenceQueue != wantQueue {
			t.Fatalf("persistenceQueue = %+v, want retry-derived queue %+v", handoff.persistenceQueue, wantQueue)
		}
		if handoff.retryPage.ExecutedQueue == nil {
			t.Fatalf("retry page = %+v, want executed queue", handoff.retryPage)
		}
		if handoff.persistenceQueue != handoff.retryPage.ExecutedQueue {
			t.Fatalf("persistenceQueue = %+v, want executed queue pointer %+v", handoff.persistenceQueue, handoff.retryPage.ExecutedQueue)
		}
	})

	t.Run("non_retryable_targets_return_queue_page_and_queue_derived_persistence_queue", func(t *testing.T) {
		t.Parallel()

		task, generation := newTaskGenerationActionQueueFixture(t, "task-generation-action-handoff-queue-1")
		target := &AssetGenerationActionTarget{
			ActionKey:       "review_missing_slots",
			InteractionMode: "queue_only",
			QueueQuery: &GenerationQueueQuery{
				Platform:     "amazon",
				Slot:         "auxiliary",
				QualityGrade: "missing",
			},
		}
		originalQueueQuery := cloneGenerationQueueQuery(target.QueueQuery)

		handoff, err := buildTaskGenerationActionExecuteRequestHandoffPhase(generation).run(context.Background(), task.ID, target)
		if err != nil {
			t.Fatalf("taskGenerationActionExecuteRequestHandoffPhase.run() error = %v", err)
		}
		if !reflect.DeepEqual(target.QueueQuery, originalQueueQuery) {
			t.Fatalf("target.QueueQuery = %+v, want original query unchanged as %+v", target.QueueQuery, originalQueueQuery)
		}
		if handoff == nil || handoff.queuePage == nil {
			t.Fatalf("handoff = %+v, want queue page handoff", handoff)
		}
		if handoff.retryPage != nil {
			t.Fatalf("handoff.retryPage = %+v, want nil for non-retryable path", handoff.retryPage)
		}

		wantQueue := generationWorkQueueFromPage(handoff.queuePage)
		if !reflect.DeepEqual(handoff.persistenceQueue, wantQueue) {
			t.Fatalf("persistenceQueue = %+v, want queue-derived queue %+v", handoff.persistenceQueue, wantQueue)
		}
		if handoff.persistenceQueue == nil || handoff.queuePage.Items == nil || len(handoff.queuePage.Items) == 0 {
			t.Fatalf("handoff = %+v, want populated queue page and persistence queue", handoff)
		}
		if &handoff.persistenceQueue.Items[0] == &handoff.queuePage.Items[0] {
			t.Fatal("persistenceQueue.Items reused queuePage.Items backing storage, want page-derived queue copy")
		}
	})

	t.Run("empty_interaction_mode_uses_default_queue_path", func(t *testing.T) {
		t.Parallel()

		task, generation := newTaskGenerationActionQueueFixture(t, "task-generation-action-handoff-default-queue-1")
		target := &AssetGenerationActionTarget{
			ActionKey: "review_missing_slots",
			QueueQuery: &GenerationQueueQuery{
				Platform:     "amazon",
				Slot:         "auxiliary",
				QualityGrade: "missing",
			},
		}
		originalQueueQuery := cloneGenerationQueueQuery(target.QueueQuery)

		handoff, err := buildTaskGenerationActionExecuteRequestHandoffPhase(generation).run(context.Background(), task.ID, target)
		if err != nil {
			t.Fatalf("taskGenerationActionExecuteRequestHandoffPhase.run() error = %v", err)
		}
		if !reflect.DeepEqual(target.QueueQuery, originalQueueQuery) {
			t.Fatalf("target.QueueQuery = %+v, want original query unchanged as %+v", target.QueueQuery, originalQueueQuery)
		}
		if handoff == nil || handoff.queuePage == nil {
			t.Fatalf("handoff = %+v, want queue page handoff for default mode", handoff)
		}
		if handoff.retryPage != nil {
			t.Fatalf("handoff.retryPage = %+v, want nil for default queue path", handoff.retryPage)
		}

		wantQueue := generationWorkQueueFromPage(handoff.queuePage)
		if !reflect.DeepEqual(handoff.persistenceQueue, wantQueue) {
			t.Fatalf("persistenceQueue = %+v, want queue-derived queue %+v", handoff.persistenceQueue, wantQueue)
		}
		if handoff.persistenceQueue == nil || handoff.queuePage.Items == nil || len(handoff.queuePage.Items) == 0 {
			t.Fatalf("handoff = %+v, want populated queue page and persistence queue", handoff)
		}
		if &handoff.persistenceQueue.Items[0] == &handoff.queuePage.Items[0] {
			t.Fatal("persistenceQueue.Items reused queuePage.Items backing storage, want page-derived queue copy")
		}
	})
}

func TestTaskGenerationActionExecuteRequestHandoffRetryPhaseRunClonesRetryRequestBeforeInvocation(t *testing.T) {
	t.Parallel()

	fixture := newRetryPersistenceFailureFixture(t, "task-generation-action-handoff-retry-phase-1")
	originalBuildRetrySelection := fixture.generation.buildRetryGenerationTaskSelection
	var observedRetryRequest *RetryGenerationTasksRequest
	fixture.generation.buildRetryGenerationTaskSelection = func(ctx context.Context, task *Task, inventory *asset.Inventory, existing []assetgeneration.Task, req *RetryGenerationTasksRequest) ([]assetgeneration.Task, error) {
		observedRetryRequest = req
		selectedTasks, err := originalBuildRetrySelection(ctx, task, inventory, existing, req)
		if req != nil {
			req.QualityGrade = "mutated-by-retry-phase-downstream"
			if len(req.Slots) > 0 {
				req.Slots[0] = "mutated-slot"
			}
		}
		return selectedTasks, err
	}

	target := &AssetGenerationActionTarget{
		ActionKey:       "generate_missing_assets",
		InteractionMode: "retryable",
		RetryRequest: &RetryGenerationTasksRequest{
			Slots:        []string{"auxiliary"},
			QualityGrade: "missing",
		},
	}
	originalRetryRequest := cloneRetryGenerationTasksRequest(target.RetryRequest)

	page, err := buildTaskGenerationActionExecuteRequestHandoffRetryPhase(fixture.generation).run(context.Background(), fixture.taskID, target)
	if err != nil {
		t.Fatalf("taskGenerationActionExecuteRequestHandoffRetryPhase.run() error = %v", err)
	}
	if observedRetryRequest == nil {
		t.Fatal("observed retry request = nil, want downstream retry request clone")
	}
	if observedRetryRequest == target.RetryRequest {
		t.Fatal("observed retry request reused original target.RetryRequest, want clone")
	}
	if observedRetryRequest.QualityGrade != "mutated-by-retry-phase-downstream" {
		t.Fatalf("observed retry request quality grade = %q, want downstream mutation recorded", observedRetryRequest.QualityGrade)
	}
	if len(observedRetryRequest.Slots) != 1 || observedRetryRequest.Slots[0] != "mutated-slot" {
		t.Fatalf("observed retry request slots = %+v, want downstream mutation recorded", observedRetryRequest.Slots)
	}
	if !reflect.DeepEqual(target.RetryRequest, originalRetryRequest) {
		t.Fatalf("target.RetryRequest = %+v, want original request unchanged as %+v", target.RetryRequest, originalRetryRequest)
	}

	expectedFixture := newRetryPersistenceFailureFixture(t, "task-generation-action-handoff-retry-phase-1")
	expectedPage, err := expectedFixture.generation.RetryTaskGenerationTasks(context.Background(), expectedFixture.taskID, cloneRetryGenerationTasksRequest(originalRetryRequest))
	if err != nil {
		t.Fatalf("RetryTaskGenerationTasks() error = %v", err)
	}
	comparablePage := *page
	comparablePage.UpdatedAt = time.Time{}
	comparableExpectedPage := *expectedPage
	comparableExpectedPage.UpdatedAt = time.Time{}
	if !reflect.DeepEqual(&comparablePage, &comparableExpectedPage) {
		t.Fatalf("retry phase page = %+v, want direct retry page %+v", &comparablePage, &comparableExpectedPage)
	}
}

func TestTaskGenerationActionExecuteRequestHandoffQueuePhaseRunClonesQueueQueryBeforeInvocation(t *testing.T) {
	t.Parallel()

	task, generation := newTaskGenerationActionQueueFixture(t, "task-generation-action-handoff-queue-phase-1")
	target := &AssetGenerationActionTarget{
		ActionKey:       "review_missing_slots",
		InteractionMode: "queue_only",
		QueueQuery: &GenerationQueueQuery{
			Platform:                      "amazon",
			Slot:                          "auxiliary",
			QualityGrade:                  "missing",
			RenderPreviewAvailable:        true,
			RenderPreviewAvailablePresent: true,
			Page:                          2,
			PageSize:                      1,
			SortBy:                        "slot",
			SortOrder:                     "asc",
		},
	}
	originalQueueQuery := cloneGenerationQueueQuery(target.QueueQuery)

	page, err := buildTaskGenerationActionExecuteRequestHandoffQueuePhase(generation).run(context.Background(), task.ID, target)
	if err != nil {
		t.Fatalf("taskGenerationActionExecuteRequestHandoffQueuePhase.run() error = %v", err)
	}
	if !reflect.DeepEqual(target.QueueQuery, originalQueueQuery) {
		t.Fatalf("target.QueueQuery = %+v, want original query unchanged as %+v", target.QueueQuery, originalQueueQuery)
	}

	expectedPage, err := generation.GetTaskGenerationQueue(context.Background(), task.ID, cloneGenerationQueueQuery(originalQueueQuery))
	if err != nil {
		t.Fatalf("GetTaskGenerationQueue() error = %v", err)
	}
	if !reflect.DeepEqual(page, expectedPage) {
		t.Fatalf("queue phase page = %+v, want direct queue page %+v", page, expectedPage)
	}

	mutatedClone := cloneGenerationQueueQuery(target.QueueQuery)
	mutatedClone.Platform = "mutated-platform"
	mutatedClone.Slot = "mutated-slot"
	mutatedClone.Page = 7
	if !reflect.DeepEqual(target.QueueQuery, originalQueueQuery) {
		t.Fatalf("target.QueueQuery = %+v after clone mutation, want original query unchanged as %+v", target.QueueQuery, originalQueueQuery)
	}
}

func TestTaskGenerationActionExecuteRequestHandoffModeRoutingRun(t *testing.T) {
	t.Parallel()

	t.Run("retryable_mode_pairs_retry_invocation_with_retry_result_handoff", func(t *testing.T) {
		t.Parallel()

		fixture := newRetryPersistenceFailureFixture(t, "task-generation-action-mode-routing-retry-1")
		originalBuildRetrySelection := fixture.generation.buildRetryGenerationTaskSelection
		var observedRetryRequest *RetryGenerationTasksRequest
		fixture.generation.buildRetryGenerationTaskSelection = func(ctx context.Context, task *Task, inventory *asset.Inventory, existing []assetgeneration.Task, req *RetryGenerationTasksRequest) ([]assetgeneration.Task, error) {
			observedRetryRequest = req
			selectedTasks, err := originalBuildRetrySelection(ctx, task, inventory, existing, req)
			if req != nil {
				req.QualityGrade = "mutated-by-mode-routing-downstream"
				if len(req.Slots) > 0 {
					req.Slots[0] = "mutated-slot"
				}
			}
			return selectedTasks, err
		}
		target := &AssetGenerationActionTarget{
			ActionKey:       "generate_missing_assets",
			InteractionMode: "retryable",
			RetryRequest: &RetryGenerationTasksRequest{
				Slots:        []string{"auxiliary"},
				QualityGrade: "missing",
			},
		}
		originalRetryRequest := cloneRetryGenerationTasksRequest(target.RetryRequest)

		handoff, err := buildTaskGenerationActionExecuteRequestHandoffModeRoutingPhase(fixture.generation).run(context.Background(), fixture.taskID, target)
		if err != nil {
			t.Fatalf("taskGenerationActionExecuteRequestHandoffModeRoutingPhase.run() error = %v", err)
		}
		if observedRetryRequest == nil {
			t.Fatal("observed retry request = nil, want downstream retry request clone")
		}
		if observedRetryRequest == target.RetryRequest {
			t.Fatal("observed retry request reused original target.RetryRequest, want clone")
		}
		if !reflect.DeepEqual(target.RetryRequest, originalRetryRequest) {
			t.Fatalf("target.RetryRequest = %+v, want original request unchanged as %+v", target.RetryRequest, originalRetryRequest)
		}
		if handoff == nil || handoff.retryPage == nil {
			t.Fatalf("handoff = %+v, want retry page handoff", handoff)
		}
		if handoff.queuePage != nil {
			t.Fatalf("handoff.queuePage = %+v, want nil for retryable path", handoff.queuePage)
		}

		wantQueue := generationWorkQueueFromRetryPage(handoff.retryPage)
		if handoff.persistenceQueue != wantQueue {
			t.Fatalf("persistenceQueue = %+v, want retry-derived queue %+v", handoff.persistenceQueue, wantQueue)
		}
		if handoff.retryPage.ExecutedQueue == nil {
			t.Fatalf("retry page = %+v, want executed queue", handoff.retryPage)
		}
		if handoff.persistenceQueue != handoff.retryPage.ExecutedQueue {
			t.Fatalf("persistenceQueue = %+v, want executed queue pointer %+v", handoff.persistenceQueue, handoff.retryPage.ExecutedQueue)
		}
	})

	t.Run("non_retryable_mode_pairs_queue_invocation_with_queue_result_handoff", func(t *testing.T) {
		t.Parallel()

		task, generation := newTaskGenerationActionQueueFixture(t, "task-generation-action-mode-routing-queue-1")
		target := &AssetGenerationActionTarget{
			ActionKey:       "review_missing_slots",
			InteractionMode: "queue_only",
			QueueQuery: &GenerationQueueQuery{
				Platform:     "amazon",
				Slot:         "auxiliary",
				QualityGrade: "missing",
			},
		}
		originalQueueQuery := cloneGenerationQueueQuery(target.QueueQuery)

		handoff, err := buildTaskGenerationActionExecuteRequestHandoffModeRoutingPhase(generation).run(context.Background(), task.ID, target)
		if err != nil {
			t.Fatalf("taskGenerationActionExecuteRequestHandoffModeRoutingPhase.run() error = %v", err)
		}
		if !reflect.DeepEqual(target.QueueQuery, originalQueueQuery) {
			t.Fatalf("target.QueueQuery = %+v, want original query unchanged as %+v", target.QueueQuery, originalQueueQuery)
		}
		if handoff == nil || handoff.queuePage == nil {
			t.Fatalf("handoff = %+v, want queue page handoff", handoff)
		}
		if handoff.retryPage != nil {
			t.Fatalf("handoff.retryPage = %+v, want nil for non-retryable path", handoff.retryPage)
		}

		wantQueue := generationWorkQueueFromPage(handoff.queuePage)
		if !reflect.DeepEqual(handoff.persistenceQueue, wantQueue) {
			t.Fatalf("persistenceQueue = %+v, want queue-derived queue %+v", handoff.persistenceQueue, wantQueue)
		}
		if handoff.persistenceQueue == nil || handoff.queuePage.Items == nil || len(handoff.queuePage.Items) == 0 {
			t.Fatalf("handoff = %+v, want populated queue page and persistence queue", handoff)
		}
		if &handoff.persistenceQueue.Items[0] == &handoff.queuePage.Items[0] {
			t.Fatal("persistenceQueue.Items reused queuePage.Items backing storage, want page-derived queue copy")
		}
	})

	t.Run("empty_interaction_mode_pairs_default_queue_invocation_with_queue_result_handoff", func(t *testing.T) {
		t.Parallel()

		task, generation := newTaskGenerationActionQueueFixture(t, "task-generation-action-mode-routing-default-queue-1")
		target := &AssetGenerationActionTarget{
			ActionKey: "review_missing_slots",
			QueueQuery: &GenerationQueueQuery{
				Platform:     "amazon",
				Slot:         "auxiliary",
				QualityGrade: "missing",
			},
		}
		originalQueueQuery := cloneGenerationQueueQuery(target.QueueQuery)

		handoff, err := buildTaskGenerationActionExecuteRequestHandoffModeRoutingPhase(generation).run(context.Background(), task.ID, target)
		if err != nil {
			t.Fatalf("taskGenerationActionExecuteRequestHandoffModeRoutingPhase.run() error = %v", err)
		}
		if !reflect.DeepEqual(target.QueueQuery, originalQueueQuery) {
			t.Fatalf("target.QueueQuery = %+v, want original query unchanged as %+v", target.QueueQuery, originalQueueQuery)
		}
		if handoff == nil || handoff.queuePage == nil {
			t.Fatalf("handoff = %+v, want queue page handoff for default mode", handoff)
		}
		if handoff.retryPage != nil {
			t.Fatalf("handoff.retryPage = %+v, want nil for default queue path", handoff.retryPage)
		}

		wantQueue := generationWorkQueueFromPage(handoff.queuePage)
		if !reflect.DeepEqual(handoff.persistenceQueue, wantQueue) {
			t.Fatalf("persistenceQueue = %+v, want queue-derived queue %+v", handoff.persistenceQueue, wantQueue)
		}
		if handoff.persistenceQueue == nil || handoff.queuePage.Items == nil || len(handoff.queuePage.Items) == 0 {
			t.Fatalf("handoff = %+v, want populated queue page and persistence queue", handoff)
		}
		if &handoff.persistenceQueue.Items[0] == &handoff.queuePage.Items[0] {
			t.Fatal("persistenceQueue.Items reused queuePage.Items backing storage, want page-derived queue copy")
		}
	})
}

func TestTaskGenerationActionExecuteRequestHandoffModePairingRun(t *testing.T) {
	t.Parallel()

	t.Run("retryable_mode_keeps_phase31_retry_pairing_and_retry_normalized_handoff", func(t *testing.T) {
		t.Parallel()

		fixture := newRetryPersistenceFailureFixture(t, "task-generation-action-mode-pairing-retry-1")
		target := &AssetGenerationActionTarget{
			ActionKey:       "generate_missing_assets",
			InteractionMode: "retryable",
			RetryRequest: &RetryGenerationTasksRequest{
				Slots:        []string{"auxiliary"},
				QualityGrade: "missing",
			},
		}
		originalRetryRequest := cloneRetryGenerationTasksRequest(target.RetryRequest)

		handoff, err := buildTaskGenerationActionExecuteRequestHandoffModePairingPhase(fixture.generation).runRetryable(context.Background(), fixture.taskID, target)
		if err != nil {
			t.Fatalf("taskGenerationActionExecuteRequestHandoffModePairingPhase.runRetryable() error = %v", err)
		}
		if !reflect.DeepEqual(target.RetryRequest, originalRetryRequest) {
			t.Fatalf("target.RetryRequest = %+v, want original request unchanged as %+v", target.RetryRequest, originalRetryRequest)
		}
		if handoff == nil || handoff.retryPage == nil {
			t.Fatalf("handoff = %+v, want retry page handoff", handoff)
		}
		if handoff.queuePage != nil {
			t.Fatalf("handoff.queuePage = %+v, want nil for retryable path", handoff.queuePage)
		}

		wantQueue := generationWorkQueueFromRetryPage(handoff.retryPage)
		if handoff.persistenceQueue != wantQueue {
			t.Fatalf("persistenceQueue = %+v, want retry-derived queue %+v", handoff.persistenceQueue, wantQueue)
		}
		if handoff.retryPage.ExecutedQueue == nil {
			t.Fatalf("retry page = %+v, want executed queue", handoff.retryPage)
		}
		if handoff.persistenceQueue != handoff.retryPage.ExecutedQueue {
			t.Fatalf("persistenceQueue = %+v, want executed queue pointer %+v", handoff.persistenceQueue, handoff.retryPage.ExecutedQueue)
		}

		wantHandoff := buildTaskGenerationActionExecuteRequestHandoffRetryResultPhase().run(handoff.retryPage)
		if !reflect.DeepEqual(handoff, wantHandoff) {
			t.Fatalf("handoff = %+v, want retry result normalization applied to retry page %+v", handoff, wantHandoff)
		}
	})

	t.Run("queue_mode_keeps_phase31_queue_pairing_and_queue_normalized_handoff", func(t *testing.T) {
		t.Parallel()

		task, generation := newTaskGenerationActionQueueFixture(t, "task-generation-action-mode-pairing-queue-1")
		target := &AssetGenerationActionTarget{
			ActionKey:       "review_missing_slots",
			InteractionMode: "queue_only",
			QueueQuery: &GenerationQueueQuery{
				Platform:     "amazon",
				Slot:         "auxiliary",
				QualityGrade: "missing",
			},
		}
		originalQueueQuery := cloneGenerationQueueQuery(target.QueueQuery)

		handoff, err := buildTaskGenerationActionExecuteRequestHandoffModePairingPhase(generation).runQueue(context.Background(), task.ID, target)
		if err != nil {
			t.Fatalf("taskGenerationActionExecuteRequestHandoffModePairingPhase.runQueue() error = %v", err)
		}
		if !reflect.DeepEqual(target.QueueQuery, originalQueueQuery) {
			t.Fatalf("target.QueueQuery = %+v, want original query unchanged as %+v", target.QueueQuery, originalQueueQuery)
		}
		if handoff == nil || handoff.queuePage == nil {
			t.Fatalf("handoff = %+v, want queue page handoff", handoff)
		}
		if handoff.retryPage != nil {
			t.Fatalf("handoff.retryPage = %+v, want nil for queue path", handoff.retryPage)
		}

		wantQueue := generationWorkQueueFromPage(handoff.queuePage)
		if !reflect.DeepEqual(handoff.persistenceQueue, wantQueue) {
			t.Fatalf("persistenceQueue = %+v, want queue-derived queue %+v", handoff.persistenceQueue, wantQueue)
		}
		if handoff.persistenceQueue == nil || handoff.queuePage.Items == nil || len(handoff.queuePage.Items) == 0 {
			t.Fatalf("handoff = %+v, want populated queue page and persistence queue", handoff)
		}
		if &handoff.persistenceQueue.Items[0] == &handoff.queuePage.Items[0] {
			t.Fatal("persistenceQueue.Items reused queuePage.Items backing storage, want page-derived queue copy")
		}

		wantHandoff := buildTaskGenerationActionExecuteRequestHandoffQueueResultPhase().run(handoff.queuePage)
		if !reflect.DeepEqual(handoff, wantHandoff) {
			t.Fatalf("handoff = %+v, want queue result normalization applied to queue page %+v", handoff, wantHandoff)
		}
	})
}

func TestTaskGenerationActionExecuteRequestHandoffResultPhase(t *testing.T) {
	t.Parallel()

	t.Run("retry_result_phase_keeps_unified_handoff_shape_and_retry_persistence_queue_mapping", func(t *testing.T) {
		t.Parallel()

		retryQueue := &GenerationWorkQueue{
			Summary: &GenerationWorkQueueSummary{TotalItems: 1, RetryableItems: 1},
			Items: []GenerationWorkQueueItem{
				{TaskID: "retry-result-task-1", Platform: "amazon", Slot: "auxiliary", Retryable: true},
			},
		}
		retryPage := &GenerationTaskPage{
			TaskID:        "retry-result-task-1",
			ExecutedQueue: retryQueue,
		}

		handoff := buildTaskGenerationActionExecuteRequestHandoffRetryResultPhase().run(retryPage)
		if handoff == nil {
			t.Fatal("handoff = nil, want retry result handoff")
		}
		if handoff.retryPage != retryPage {
			t.Fatalf("handoff.retryPage = %+v, want original retry page %+v", handoff.retryPage, retryPage)
		}
		if handoff.queuePage != nil {
			t.Fatalf("handoff.queuePage = %+v, want nil on retry result path", handoff.queuePage)
		}
		if handoff.persistenceQueue != retryQueue {
			t.Fatalf("handoff.persistenceQueue = %+v, want retry-derived queue %+v", handoff.persistenceQueue, retryQueue)
		}

		wantHandoff := buildTaskGenerationActionExecuteRequestHandoffResultShapePhase().fromRetryNormalization(
			buildTaskGenerationActionExecuteRequestHandoffResultNormalizationPhase().fromRetryPage(retryPage),
		)
		if !reflect.DeepEqual(handoff, wantHandoff) {
			t.Fatalf("handoff = %+v, want retry result routed through normalization and result shape %+v", handoff, wantHandoff)
		}
	})

	t.Run("queue_result_phase_keeps_unified_handoff_shape_and_queue_persistence_queue_mapping", func(t *testing.T) {
		t.Parallel()

		queuePage := &GenerationQueuePage{
			TaskID:  "queue-result-task-1",
			Summary: &GenerationWorkQueueSummary{TotalItems: 1, PreviewableItems: 1},
			Items: []GenerationWorkQueueItem{
				{TaskID: "queue-result-task-1", Platform: "amazon", Slot: "auxiliary", State: "queued"},
			},
		}

		handoff := buildTaskGenerationActionExecuteRequestHandoffQueueResultPhase().run(queuePage)
		if handoff == nil {
			t.Fatal("handoff = nil, want queue result handoff")
		}
		if handoff.queuePage != queuePage {
			t.Fatalf("handoff.queuePage = %+v, want original queue page %+v", handoff.queuePage, queuePage)
		}
		if handoff.retryPage != nil {
			t.Fatalf("handoff.retryPage = %+v, want nil on queue result path", handoff.retryPage)
		}

		wantQueue := generationWorkQueueFromPage(queuePage)
		if !reflect.DeepEqual(handoff.persistenceQueue, wantQueue) {
			t.Fatalf("handoff.persistenceQueue = %+v, want queue-derived queue %+v", handoff.persistenceQueue, wantQueue)
		}
		if handoff.persistenceQueue == nil || len(handoff.persistenceQueue.Items) != 1 {
			t.Fatalf("handoff.persistenceQueue = %+v, want cloned queue items", handoff.persistenceQueue)
		}
		if &handoff.persistenceQueue.Items[0] == &queuePage.Items[0] {
			t.Fatal("handoff.persistenceQueue.Items reused queuePage.Items backing storage, want copy")
		}

		wantHandoff := buildTaskGenerationActionExecuteRequestHandoffResultShapePhase().fromQueueNormalization(
			buildTaskGenerationActionExecuteRequestHandoffResultNormalizationPhase().fromQueuePage(queuePage),
		)
		if !reflect.DeepEqual(handoff, wantHandoff) {
			t.Fatalf("handoff = %+v, want queue result routed through normalization and result shape %+v", handoff, wantHandoff)
		}
	})
}

func TestTaskGenerationActionExecuteRequestHandoffResultDispatchPhase(t *testing.T) {
	t.Parallel()

	t.Run("retry_page_dispatch_routes_through_normalization_and_result_shape", func(t *testing.T) {
		t.Parallel()

		retryQueue := &GenerationWorkQueue{
			Summary: &GenerationWorkQueueSummary{TotalItems: 1, RetryableItems: 1},
			Items: []GenerationWorkQueueItem{
				{TaskID: "retry-dispatch-task-1", Platform: "amazon", Slot: "auxiliary", Retryable: true},
			},
		}
		retryPage := &GenerationTaskPage{
			TaskID:        "retry-dispatch-task-1",
			ExecutedQueue: retryQueue,
		}

		handoff := buildTaskGenerationActionExecuteRequestHandoffResultDispatchPhase().fromRetryPage(retryPage)
		wantHandoff := buildTaskGenerationActionExecuteRequestHandoffResultShapePhase().fromRetryNormalization(
			buildTaskGenerationActionExecuteRequestHandoffResultNormalizationPhase().fromRetryPage(retryPage),
		)
		if !reflect.DeepEqual(handoff, wantHandoff) {
			t.Fatalf("handoff = %+v, want retry dispatch handoff %+v", handoff, wantHandoff)
		}
	})

	t.Run("queue_page_dispatch_routes_through_normalization_and_result_shape", func(t *testing.T) {
		t.Parallel()

		queuePage := &GenerationQueuePage{
			TaskID:  "queue-dispatch-task-1",
			Summary: &GenerationWorkQueueSummary{TotalItems: 1, PreviewableItems: 1},
			Items: []GenerationWorkQueueItem{
				{TaskID: "queue-dispatch-task-1", Platform: "amazon", Slot: "auxiliary", State: "queued"},
			},
		}

		handoff := buildTaskGenerationActionExecuteRequestHandoffResultDispatchPhase().fromQueuePage(queuePage)
		wantHandoff := buildTaskGenerationActionExecuteRequestHandoffResultShapePhase().fromQueueNormalization(
			buildTaskGenerationActionExecuteRequestHandoffResultNormalizationPhase().fromQueuePage(queuePage),
		)
		if !reflect.DeepEqual(handoff, wantHandoff) {
			t.Fatalf("handoff = %+v, want queue dispatch handoff %+v", handoff, wantHandoff)
		}
	})
}

func TestTaskGenerationActionExecuteRequestHandoffResultShapePhase(t *testing.T) {
	t.Parallel()

	t.Run("retry_normalization_results_keep_unified_handoff_shape", func(t *testing.T) {
		t.Parallel()

		retryQueue := &GenerationWorkQueue{
			Summary: &GenerationWorkQueueSummary{TotalItems: 1, RetryableItems: 1},
			Items: []GenerationWorkQueueItem{
				{TaskID: "retry-task-1", Platform: "amazon", Slot: "auxiliary", Retryable: true},
			},
		}
		retryPage := &GenerationTaskPage{
			TaskID:        "retry-task-1",
			ExecutedQueue: retryQueue,
		}

		normalized := buildTaskGenerationActionExecuteRequestHandoffResultNormalizationPhase().fromRetryPage(retryPage)
		handoff := buildTaskGenerationActionExecuteRequestHandoffResultShapePhase().fromRetryNormalization(normalized)
		if handoff == nil {
			t.Fatal("handoff = nil, want retry handoff result")
		}
		if handoff.retryPage != retryPage {
			t.Fatalf("handoff.retryPage = %+v, want original retry page %+v", handoff.retryPage, retryPage)
		}
		if handoff.queuePage != nil {
			t.Fatalf("handoff.queuePage = %+v, want nil on retry adaptation path", handoff.queuePage)
		}
		if handoff.persistenceQueue != retryQueue {
			t.Fatalf("handoff.persistenceQueue = %+v, want retry-derived queue %+v", handoff.persistenceQueue, retryQueue)
		}
	})

	t.Run("queue_normalization_results_keep_unified_handoff_shape", func(t *testing.T) {
		t.Parallel()

		queuePage := &GenerationQueuePage{
			TaskID:  "queue-task-1",
			Summary: &GenerationWorkQueueSummary{TotalItems: 1, PreviewableItems: 1},
			Items: []GenerationWorkQueueItem{
				{TaskID: "queue-task-1", Platform: "amazon", Slot: "auxiliary", State: "queued"},
			},
		}

		normalized := buildTaskGenerationActionExecuteRequestHandoffResultNormalizationPhase().fromQueuePage(queuePage)
		handoff := buildTaskGenerationActionExecuteRequestHandoffResultShapePhase().fromQueueNormalization(normalized)
		if handoff == nil {
			t.Fatal("handoff = nil, want queue handoff result")
		}
		if handoff.queuePage != queuePage {
			t.Fatalf("handoff.queuePage = %+v, want original queue page %+v", handoff.queuePage, queuePage)
		}
		if handoff.retryPage != nil {
			t.Fatalf("handoff.retryPage = %+v, want nil on queue adaptation path", handoff.retryPage)
		}

		wantQueue := generationWorkQueueFromPage(queuePage)
		if !reflect.DeepEqual(handoff.persistenceQueue, wantQueue) {
			t.Fatalf("handoff.persistenceQueue = %+v, want queue-derived queue %+v", handoff.persistenceQueue, wantQueue)
		}
		if handoff.persistenceQueue == nil || len(handoff.persistenceQueue.Items) != 1 {
			t.Fatalf("handoff.persistenceQueue = %+v, want cloned queue items", handoff.persistenceQueue)
		}
		if &handoff.persistenceQueue.Items[0] == &queuePage.Items[0] {
			t.Fatal("handoff.persistenceQueue.Items reused queuePage.Items backing storage, want copy")
		}
	})
}

func TestTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase(t *testing.T) {
	t.Parallel()

	t.Run("retry_page_adaptation_only_maps_page_to_persistence_queue", func(t *testing.T) {
		t.Parallel()

		retryQueue := &GenerationWorkQueue{
			Summary: &GenerationWorkQueueSummary{TotalItems: 1, RetryableItems: 1},
			Items: []GenerationWorkQueueItem{
				{TaskID: "retry-task-2", Platform: "amazon", Slot: "auxiliary", Retryable: true},
			},
		}
		retryPage := &GenerationTaskPage{
			TaskID:        "retry-task-2",
			ExecutedQueue: retryQueue,
		}

		persistenceQueue := buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase().persistenceQueueFromRetryPage(retryPage)
		if persistenceQueue != retryQueue {
			t.Fatalf("persistenceQueue = %+v, want retry-derived queue %+v", persistenceQueue, retryQueue)
		}
	})

	t.Run("queue_page_adaptation_only_maps_page_to_persistence_queue", func(t *testing.T) {
		t.Parallel()

		queuePage := &GenerationQueuePage{
			TaskID:  "queue-task-2",
			Summary: &GenerationWorkQueueSummary{TotalItems: 1, PreviewableItems: 1},
			Items: []GenerationWorkQueueItem{
				{TaskID: "queue-task-2", Platform: "amazon", Slot: "auxiliary", State: "queued"},
			},
		}

		persistenceQueue := buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase().persistenceQueueFromQueuePage(queuePage)
		wantQueue := generationWorkQueueFromPage(queuePage)
		if !reflect.DeepEqual(persistenceQueue, wantQueue) {
			t.Fatalf("persistenceQueue = %+v, want queue-derived queue %+v", persistenceQueue, wantQueue)
		}
		if persistenceQueue == nil || len(persistenceQueue.Items) != 1 {
			t.Fatalf("persistenceQueue = %+v, want cloned queue items", persistenceQueue)
		}
		if &persistenceQueue.Items[0] == &queuePage.Items[0] {
			t.Fatal("persistenceQueue.Items reused queuePage.Items backing storage, want copy")
		}
	})
}

func TestTaskGenerationActionExecuteRequestHandoffResultNormalizationPhase(t *testing.T) {
	t.Parallel()

	t.Run("retry_page_normalization_keeps_retry_page_and_derives_persistence_queue_through_adaptation", func(t *testing.T) {
		t.Parallel()

		retryQueue := &GenerationWorkQueue{
			Summary: &GenerationWorkQueueSummary{TotalItems: 1, RetryableItems: 1},
			Items: []GenerationWorkQueueItem{
				{TaskID: "retry-normalization-task-1", Platform: "amazon", Slot: "auxiliary", Retryable: true},
			},
		}
		retryPage := &GenerationTaskPage{
			TaskID:        "retry-normalization-task-1",
			ExecutedQueue: retryQueue,
		}

		normalized := buildTaskGenerationActionExecuteRequestHandoffResultNormalizationPhase().fromRetryPage(retryPage)
		if normalized == nil {
			t.Fatal("normalized = nil, want retry normalization")
		}
		if normalized.retryPage != retryPage {
			t.Fatalf("normalized.retryPage = %+v, want original retry page %+v", normalized.retryPage, retryPage)
		}
		if normalized.queuePage != nil {
			t.Fatalf("normalized.queuePage = %+v, want nil for retry normalization", normalized.queuePage)
		}
		if normalized.persistenceQueue != retryQueue {
			t.Fatalf("normalized.persistenceQueue = %+v, want retry-derived queue %+v", normalized.persistenceQueue, retryQueue)
		}
	})

	t.Run("queue_page_normalization_keeps_queue_page_and_derives_persistence_queue_through_adaptation", func(t *testing.T) {
		t.Parallel()

		queuePage := &GenerationQueuePage{
			TaskID:  "queue-normalization-task-1",
			Summary: &GenerationWorkQueueSummary{TotalItems: 1, PreviewableItems: 1},
			Items: []GenerationWorkQueueItem{
				{TaskID: "queue-normalization-task-1", Platform: "amazon", Slot: "auxiliary", State: "queued"},
			},
		}

		normalized := buildTaskGenerationActionExecuteRequestHandoffResultNormalizationPhase().fromQueuePage(queuePage)
		if normalized == nil {
			t.Fatal("normalized = nil, want queue normalization")
		}
		if normalized.queuePage != queuePage {
			t.Fatalf("normalized.queuePage = %+v, want original queue page %+v", normalized.queuePage, queuePage)
		}
		if normalized.retryPage != nil {
			t.Fatalf("normalized.retryPage = %+v, want nil for queue normalization", normalized.retryPage)
		}

		wantQueue := generationWorkQueueFromPage(queuePage)
		if !reflect.DeepEqual(normalized.persistenceQueue, wantQueue) {
			t.Fatalf("normalized.persistenceQueue = %+v, want queue-derived queue %+v", normalized.persistenceQueue, wantQueue)
		}
		if normalized.persistenceQueue == nil || len(normalized.persistenceQueue.Items) != 1 {
			t.Fatalf("normalized.persistenceQueue = %+v, want cloned queue items", normalized.persistenceQueue)
		}
		if &normalized.persistenceQueue.Items[0] == &queuePage.Items[0] {
			t.Fatal("normalized.persistenceQueue.Items reused queuePage.Items backing storage, want copy")
		}
	})
}

func TestTaskGenerationActionProjectionSessionUsesRetryQueueForRetryableTargets(t *testing.T) {
	t.Parallel()

	target := newTaskGenerationActionProjectionTarget("retryable")
	currentResult := newTaskGenerationActionProjectionResult("task-generation-action-session-retry-1", "asset-rev-retry", "preview-rev-retry", "task-rev-retry")
	retryQueue := newTaskGenerationActionProjectionQueue("task-generation-action-session-retry-1", &GenerationWorkQueueSummary{
		TotalItems:       1,
		CompletedItems:   1,
		PreviewableItems: 1,
		ApprovedSections: 1,
	}, "completed")

	session := buildTaskGenerationActionProjectionSessionPhase().run(&taskGenerationActionProjectionInput{
		target:        target,
		currentResult: currentResult,
		execution: &taskGenerationActionExecution{
			retryPage: &GenerationTaskPage{
				MatchedQueue: newTaskGenerationActionProjectionQueue("task-generation-action-session-retry-1", &GenerationWorkQueueSummary{
					TotalItems:       1,
					ReadyItems:       1,
					PreviewableItems: 1,
				}, "ready"),
				ExecutedQueue: retryQueue,
			},
		},
	})

	if session == nil {
		t.Fatal("session = nil, want retry session result")
	}
	if session.currentResult != currentResult {
		t.Fatalf("currentResult = %+v, want base current result when refresh is unavailable", session.currentResult)
	}
	if !reflect.DeepEqual(session.reviewQueue, retryQueue) {
		t.Fatalf("reviewQueue = %+v, want retry executed queue", session.reviewQueue)
	}

	wantSession := buildGenerationReviewSession(currentResult, retryQueue, projectionQueueQuery(target))
	if !reflect.DeepEqual(session.reviewSession, wantSession) {
		t.Fatalf("reviewSession = %+v, want %+v", session.reviewSession, wantSession)
	}
}

func TestTaskGenerationActionProjectionSessionUsesQueuePageForNonRetryableTargets(t *testing.T) {
	t.Parallel()

	target := newTaskGenerationActionProjectionTarget("queue_only")
	currentResult := newTaskGenerationActionProjectionResult("task-generation-action-session-queue-1", "asset-rev-queue", "preview-rev-queue", "task-rev-queue")
	queue := newTaskGenerationActionProjectionQueue("task-generation-action-session-queue-1", &GenerationWorkQueueSummary{
		TotalItems:       1,
		CompletedItems:   1,
		PreviewableItems: 1,
		ApprovedSections: 1,
	}, "completed")

	session := buildTaskGenerationActionProjectionSessionPhase().run(&taskGenerationActionProjectionInput{
		target:        target,
		currentResult: currentResult,
		execution: &taskGenerationActionExecution{
			queuePage: &GenerationQueuePage{
				Summary: queue.Summary,
				Items:   queue.Items,
			},
		},
	})

	if session == nil {
		t.Fatal("session = nil, want queue-backed session result")
	}

	wantQueue := generationWorkQueueFromPage(&GenerationQueuePage{
		Summary: queue.Summary,
		Items:   queue.Items,
	})
	if !reflect.DeepEqual(session.reviewQueue, wantQueue) {
		t.Fatalf("reviewQueue = %+v, want %+v", session.reviewQueue, wantQueue)
	}

	wantSession := buildGenerationReviewSession(currentResult, wantQueue, projectionQueueQuery(target))
	if !reflect.DeepEqual(session.reviewSession, wantSession) {
		t.Fatalf("reviewSession = %+v, want %+v", session.reviewSession, wantSession)
	}
}

func TestTaskGenerationActionProjectionSessionPrefersRefreshedCurrentResult(t *testing.T) {
	t.Parallel()

	target := newTaskGenerationActionProjectionTarget("queue_only")
	baseResult := newTaskGenerationActionProjectionResult("task-generation-action-session-refresh-1", "asset-rev-base", "preview-rev-base", "task-rev-base")
	refreshedResult := newTaskGenerationActionProjectionResult("task-generation-action-session-refresh-1", "asset-rev-refresh", "preview-rev-refresh", "task-rev-refresh")
	queue := newTaskGenerationActionProjectionQueue("task-generation-action-session-refresh-1", &GenerationWorkQueueSummary{
		TotalItems:       1,
		CompletedItems:   1,
		PreviewableItems: 1,
		ApprovedSections: 1,
	}, "completed")

	session := buildTaskGenerationActionProjectionSessionPhase().run(&taskGenerationActionProjectionInput{
		target:        target,
		currentResult: baseResult,
		refresh: &taskGenerationActionRefreshResult{
			currentResult: refreshedResult,
		},
		execution: &taskGenerationActionExecution{
			queuePage: &GenerationQueuePage{
				Summary: queue.Summary,
				Items:   queue.Items,
			},
		},
	})

	if session == nil {
		t.Fatal("session = nil, want refreshed session result")
	}
	if session.currentResult != refreshedResult {
		t.Fatalf("currentResult = %+v, want refreshed current result", session.currentResult)
	}
	if session.reviewSession == nil || session.reviewSession.FocusedRenderPreview == nil {
		t.Fatalf("reviewSession = %+v, want focused render preview from refreshed result", session.reviewSession)
	}
	if session.reviewSession.FocusedRenderPreview.AssetRevision != "asset-rev-refresh" || session.reviewSession.FocusedRenderPreview.PreviewRevision != "preview-rev-refresh" || session.reviewSession.FocusedRenderPreview.TaskRevision != "task-rev-refresh" {
		t.Fatalf("focused render preview = %+v, want refreshed revisions", session.reviewSession.FocusedRenderPreview)
	}
}

func TestTaskGenerationActionProjectionFinalizeBuildsWorkflowAndPatch(t *testing.T) {
	t.Parallel()

	target := newTaskGenerationActionProjectionTarget("queue_only")
	previousQueue := newTaskGenerationActionProjectionQueue("task-generation-action-finalize-1", &GenerationWorkQueueSummary{
		TotalItems:            1,
		ReadyItems:            1,
		PreviewableItems:      1,
		ReviewPendingSections: 1,
	}, "ready")
	previousSession := buildGenerationReviewSession(
		newTaskGenerationActionProjectionResult("task-generation-action-finalize-1", "asset-rev-old", "preview-rev-old", "task-rev-old"),
		previousQueue,
		target.QueueQuery,
	)
	currentResult := newTaskGenerationActionProjectionResult("task-generation-action-finalize-1", "asset-rev-new", "preview-rev-new", "task-rev-new")
	currentQueue := newTaskGenerationActionProjectionQueue("task-generation-action-finalize-1", &GenerationWorkQueueSummary{
		TotalItems:       1,
		CompletedItems:   1,
		PreviewableItems: 1,
		ApprovedSections: 1,
	}, "completed")
	reviewSession := buildGenerationReviewSession(currentResult, currentQueue, target.QueueQuery)
	result := &GenerationActionExecutionResult{
		ActionKey:              "stale-action",
		ResponseMode:           "full",
		PlatformRenderPreviews: []PlatformAssetRenderPreviews{{Platform: "shein", Main: &AssetRenderPreviewSlot{AssetID: "asset-preview-1"}}},
		DeltaToken:             "stale-delta-token",
	}

	finalized := buildTaskGenerationActionProjectionFinalizePhase().run(&taskGenerationActionProjectionInput{
		actionKey:             target.ActionKey,
		target:                target,
		previousReviewSession: previousSession,
	}, result, reviewSession)

	if finalized != result {
		t.Fatalf("finalized result = %+v, want in-place mutation of input result", finalized)
	}
	if finalized.ReviewWorkflow == nil || finalized.ReviewWorkflow.ActionKey != target.ActionKey || finalized.ReviewWorkflow.Platform != "shein" || finalized.ReviewWorkflow.Slot != "main" || finalized.ReviewWorkflow.Capability != "detail_preview" {
		t.Fatalf("review workflow = %+v, want action-and-target-derived workflow", finalized.ReviewWorkflow)
	}
	if finalized.ReviewSession == nil || finalized.ReviewSession.LastWorkflowResult == nil || finalized.ReviewSession.LastWorkflowResult.ActionKey != target.ActionKey {
		t.Fatalf("review session = %+v, want workflow applied to session", finalized.ReviewSession)
	}
	if len(finalized.ReviewSession.Sections) == 0 {
		t.Fatalf("review session sections = %+v, want populated review sections", finalized.ReviewSession.Sections)
	}
	if finalized.ReviewPatch == nil || finalized.ReviewPatch.LastWorkflowResult == nil || finalized.ReviewPatch.LastWorkflowResult.ActionKey != target.ActionKey {
		t.Fatalf("review patch = %+v, want workflow attached to patch", finalized.ReviewPatch)
	}
	if len(finalized.ReviewPatch.ChangedSections) == 0 {
		t.Fatalf("changed sections = %+v, want patch generated from updated session", finalized.ReviewPatch.ChangedSections)
	}
	if finalized.ReviewPatch.ChangedSections[0].WorkflowState != finalized.ReviewSession.Sections[0].WorkflowState || finalized.ReviewPatch.ChangedSections[0].WorkflowMessage != finalized.ReviewWorkflow.Message {
		t.Fatalf("changed section = %+v, want workflow-applied section state before patching", finalized.ReviewPatch.ChangedSections[0])
	}
	if finalized.DeltaToken == "" || finalized.DeltaToken != finalized.ReviewPatch.DeltaToken || finalized.DeltaToken == "stale-delta-token" {
		t.Fatalf("delta token = %q, review patch = %+v, want patch delta token to win", finalized.DeltaToken, finalized.ReviewPatch)
	}
}

func TestTaskGenerationActionProjectionFinalizePreservesDeltaTokenFallbackOrder(t *testing.T) {
	t.Parallel()

	target := newTaskGenerationActionProjectionTarget("queue_only")
	result := &GenerationActionExecutionResult{
		ResponseMode: "full",
		DeltaToken:   "keep-existing-delta",
	}

	finalized := buildTaskGenerationActionProjectionFinalizePhase().run(&taskGenerationActionProjectionInput{
		actionKey: target.ActionKey,
		target:    target,
	}, result, nil)

	if finalized.ReviewPatch != nil {
		t.Fatalf("review patch = %+v, want nil without projected session", finalized.ReviewPatch)
	}
	if finalized.DeltaToken != "keep-existing-delta" {
		t.Fatalf("delta token = %q, want existing delta token preserved when finalize has no patch/session fallback", finalized.DeltaToken)
	}
}

func TestTaskGenerationActionProjectionFinalizeSupportsPatchOnlyResponses(t *testing.T) {
	t.Parallel()

	target := newTaskGenerationActionProjectionTarget("review_only")
	previousSession := buildGenerationReviewSession(
		newTaskGenerationActionProjectionResult("task-generation-action-finalize-patch-1", "asset-rev-old", "preview-rev-old", "task-rev-old"),
		newTaskGenerationActionProjectionQueue("task-generation-action-finalize-patch-1", &GenerationWorkQueueSummary{
			TotalItems:            1,
			ReadyItems:            1,
			PreviewableItems:      1,
			ReviewPendingSections: 1,
		}, "ready"),
		target.QueueQuery,
	)
	currentResult := newTaskGenerationActionProjectionResult("task-generation-action-finalize-patch-1", "asset-rev-new", "preview-rev-new", "task-rev-new")
	currentQueue := newTaskGenerationActionProjectionQueue("task-generation-action-finalize-patch-1", &GenerationWorkQueueSummary{
		TotalItems:       1,
		CompletedItems:   1,
		PreviewableItems: 1,
		ApprovedSections: 1,
	}, "completed")
	reviewSession := buildGenerationReviewSession(currentResult, currentQueue, target.QueueQuery)

	finalized := buildTaskGenerationActionProjectionFinalizePhase().run(&taskGenerationActionProjectionInput{
		actionKey:             target.ActionKey,
		target:                target,
		responseMode:          "patch_only",
		previousReviewSession: previousSession,
	}, &GenerationActionExecutionResult{
		ResponseMode:           "patch_only",
		PlatformRenderPreviews: []PlatformAssetRenderPreviews{{Platform: "shein", Main: &AssetRenderPreviewSlot{AssetID: "asset-preview-1"}}},
	}, reviewSession)

	if finalized == nil {
		t.Fatal("finalized result = nil, want patch-only finalization result")
	}
	if finalized.ReviewSession != nil {
		t.Fatalf("review session = %+v, want patch_only response to omit session", finalized.ReviewSession)
	}
	if len(finalized.PlatformRenderPreviews) != 0 {
		t.Fatalf("platform render previews = %+v, want patch_only response to omit previews", finalized.PlatformRenderPreviews)
	}
	if finalized.ReviewPatch == nil || finalized.ReviewPatch.LastWorkflowResult == nil || finalized.ReviewPatch.LastWorkflowResult.ActionKey != target.ActionKey {
		t.Fatalf("review patch = %+v, want workflow-attached patch payload", finalized.ReviewPatch)
	}
	if finalized.DeltaToken == "" || finalized.DeltaToken != finalized.ReviewPatch.DeltaToken {
		t.Fatalf("delta token = %q, review patch = %+v, want patch delta token preserved", finalized.DeltaToken, finalized.ReviewPatch)
	}
}

func TestTaskGenerationActionFinalizeCopiesProjectionAndAppliesConditionalState(t *testing.T) {
	t.Parallel()

	resolvedTarget := newTaskGenerationActionProjectionTarget("queue_only")
	resolvedTarget.NavigationTarget = &GenerationReviewNavigationTarget{DispatchKind: "review_session"}
	result := &GenerationActionExecutionResult{
		ActionKey:       "approve_section_review",
		InteractionMode: "queue_only",
		ResponseMode:    "full",
		ResolvedTarget:  resolvedTarget,
		Audit: &GenerationActionAudit{
			RequestedActionKey: "approve_section_review",
			ResolvedActionKey:  "approve_section_review",
			ResolutionSource:   "request_target",
			ExecutionPath:      "queue_only",
		},
		Overview:   &AssetGenerationOverview{PrimaryActionKey: "stale-overview"},
		DeltaToken: "stale-delta-token",
	}

	projection := &GenerationActionExecutionResult{
		Overview: &AssetGenerationOverview{PrimaryActionKey: "review_ready_assets"},
		Queue: &GenerationQueuePage{
			TaskID:     "task-generation-action-finalize-copyback-1",
			DeltaToken: "queue-delta-token",
			Summary:    &GenerationWorkQueueSummary{TotalItems: 1, PreviewableItems: 1},
		},
		Retry: &GenerationTaskPage{
			TaskID: "task-generation-action-finalize-copyback-1",
			Total:  1,
		},
		ReviewWorkflow: &GenerationReviewWorkflowResult{
			ActionKey:  "approve_section_review",
			Status:     "applied",
			Platform:   "shein",
			Slot:       "main",
			Capability: "detail_preview",
		},
		ReviewSession: &GenerationReviewSession{
			SelectedPlatform: "shein",
			SelectedSlot:     "main",
		},
		ReviewPatch: &GenerationReviewSessionPatch{
			DeltaToken: "projection-delta-token",
		},
		PlatformRenderPreviews: []PlatformAssetRenderPreviews{{
			Platform: "shein",
			Main:     &AssetRenderPreviewSlot{AssetID: "asset-preview-1"},
		}},
		DeltaToken: "projection-delta-token",
	}

	finalized := buildTaskGenerationActionFinalizePhase().run(result, projection)

	if finalized != result {
		t.Fatalf("finalized result = %+v, want in-place mutation of input result", finalized)
	}
	if finalized.Overview != projection.Overview || finalized.Queue != projection.Queue || finalized.Retry != projection.Retry {
		t.Fatalf("finalized = %+v, want projection overview/queue/retry copied back", finalized)
	}
	if finalized.ReviewWorkflow != projection.ReviewWorkflow || finalized.ReviewSession != projection.ReviewSession || finalized.ReviewPatch != projection.ReviewPatch {
		t.Fatalf("finalized = %+v, want projection review payload copied back", finalized)
	}
	if !reflect.DeepEqual(finalized.PlatformRenderPreviews, projection.PlatformRenderPreviews) {
		t.Fatalf("platform render previews = %+v, want projection previews %+v", finalized.PlatformRenderPreviews, projection.PlatformRenderPreviews)
	}
	if finalized.DeltaToken != projection.DeltaToken {
		t.Fatalf("delta token = %q, want projection delta token %q", finalized.DeltaToken, projection.DeltaToken)
	}
	if finalized.Conditional == nil || finalized.Conditional.DeltaToken != projection.DeltaToken {
		t.Fatalf("conditional = %+v, want conditional state from copied-back delta token", finalized.Conditional)
	}
	if finalized.ResolvedTarget == nil || finalized.ResolvedTarget.NavigationTarget == nil || finalized.ResolvedTarget.NavigationTarget.Conditional == nil {
		t.Fatalf("resolved target = %+v, want conditional state applied to preserved navigation target", finalized.ResolvedTarget)
	}
	if finalized.ResolvedTarget.NavigationTarget.Conditional.DeltaToken != projection.DeltaToken {
		t.Fatalf("navigation conditional = %+v, want projection delta token", finalized.ResolvedTarget.NavigationTarget.Conditional)
	}
}

func TestTaskGenerationActionFinalizePreservesEarlierOutwardFields(t *testing.T) {
	t.Parallel()

	resolvedTarget := newTaskGenerationActionProjectionTarget("retryable")
	audit := &GenerationActionAudit{
		RequestedActionKey: "generate_missing_assets",
		ResolvedActionKey:  "generate_missing_assets",
		ResolutionSource:   "request_target",
		ExecutionPath:      "retryable",
	}
	result := &GenerationActionExecutionResult{
		ActionKey:       "generate_missing_assets",
		InteractionMode: "retryable",
		ResponseMode:    "patch_only",
		ResolvedTarget:  resolvedTarget,
		Audit:           audit,
	}
	projection := &GenerationActionExecutionResult{
		ActionKey:       "projected-action",
		InteractionMode: "queue_only",
		ResponseMode:    "full",
		ResolvedTarget: &AssetGenerationActionTarget{
			ActionKey:       "projected-action",
			InteractionMode: "queue_only",
		},
		Audit: &GenerationActionAudit{
			RequestedActionKey: "projected-action",
			ResolvedActionKey:  "projected-action",
			ResolutionSource:   "projection",
			ExecutionPath:      "queue_only",
		},
		DeltaToken: "projection-delta-token",
	}

	finalized := buildTaskGenerationActionFinalizePhase().run(result, projection)

	if finalized.ActionKey != result.ActionKey {
		t.Fatalf("action key = %q, want preserved original %q", finalized.ActionKey, result.ActionKey)
	}
	if finalized.InteractionMode != result.InteractionMode {
		t.Fatalf("interaction mode = %q, want preserved original %q", finalized.InteractionMode, result.InteractionMode)
	}
	if finalized.ResponseMode != result.ResponseMode {
		t.Fatalf("response mode = %q, want preserved original %q", finalized.ResponseMode, result.ResponseMode)
	}
	if finalized.ResolvedTarget != resolvedTarget {
		t.Fatalf("resolved target = %+v, want preserved original target %+v", finalized.ResolvedTarget, resolvedTarget)
	}
	if finalized.Audit != audit {
		t.Fatalf("audit = %+v, want preserved original audit %+v", finalized.Audit, audit)
	}
}

func TestTaskGenerationActionRefreshRehydratesOverviewAndRenderPreviews(t *testing.T) {
	t.Parallel()

	taskID := "task-generation-action-refresh-1"
	repo := &sequencedTaskSnapshotsRepo{snapshots: []*Task{
		{
			ID:        taskID,
			Status:    TaskStatusCompleted,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Request:   &GenerateRequest{Platforms: []string{"amazon"}},
			Result: &ListingKitResult{
				TaskID: taskID,
				AssetRenderPreviews: []AssetRenderPreview{{
					AssetID:       "asset-overview-1",
					PreviewFormat: "svg",
					PreviewSVG:    "<svg>overview</svg>",
					VisualMode:    "selling_point",
					LayerTypes:    []string{"detail"},
				}},
				Amazon: &AmazonPackage{ImageBundle: &common.PublishImageBundle{
					Platform: "amazon",
					MissingSlots: []common.MissingSlot{{
						Slot:          "auxiliary",
						Purpose:       "scene",
						RecipeID:      "amazon-lifestyle",
						TemplateLabel: "Amazon Lifestyle Scene",
						RenderProfile: "amazon_lifestyle_scene",
						StateLabel:    "missing",
					}},
				}},
			},
		},
		{
			ID:        taskID,
			Status:    TaskStatusCompleted,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Request:   &GenerateRequest{Platforms: []string{"amazon"}},
			Result: &ListingKitResult{
				TaskID: taskID,
				AssetRenderPreviews: []AssetRenderPreview{{
					AssetID:       "asset-preview-2",
					PreviewFormat: "svg",
					PreviewSVG:    "<svg>preview-2</svg>",
					VisualMode:    "selling_point",
					LayerTypes:    []string{"detail"},
				}},
				Amazon: &AmazonPackage{ImageBundle: &common.PublishImageBundle{
					Platform: "amazon",
					Main: &common.BundleSlot{
						Key:           "main",
						AssetID:       "asset-preview-2",
						StateLabel:    "ready",
						TemplateLabel: "Amazon Main",
					},
				}},
			},
		},
		{
			ID:        taskID,
			Status:    TaskStatusCompleted,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Request:   &GenerateRequest{Platforms: []string{"amazon"}},
			Result: &ListingKitResult{
				TaskID: taskID,
				AssetRenderPreviews: []AssetRenderPreview{{
					AssetID:       "asset-current-3",
					PreviewFormat: "svg",
					PreviewSVG:    "<svg>current</svg>",
					VisualMode:    "selling_point",
					LayerTypes:    []string{"detail"},
				}},
				Amazon: &AmazonPackage{ImageBundle: &common.PublishImageBundle{
					Platform: "amazon",
					Main: &common.BundleSlot{
						Key:           "main",
						AssetID:       "asset-current-3",
						StateLabel:    "ready",
						TemplateLabel: "Amazon Main",
					},
				}},
			},
		},
	}}

	taskCall := 0

	generation := newTaskGenerationService(taskGenerationServiceConfig{
		repo: repo,
		listAssetGenerationTasks: func(ctx context.Context, requestedTaskID string) ([]assetgeneration.Task, error) {
			taskCall++
			switch taskCall {
			case 1:
				return []assetgeneration.Task{{
					TaskID:          requestedTaskID,
					ID:              "amazon:amazon-lifestyle",
					Platform:        "amazon",
					RecipeID:        "amazon-lifestyle",
					AssetKind:       asset.KindSceneImage,
					Slot:            "auxiliary",
					Purpose:         "scene",
					Status:          "planned",
					ExecutionStatus: "planned",
					ExecutionMode:   assetgeneration.ExecutionModeRendererBacked,
					CanExecute:      true,
					SourceAssetIDs:  []string{"asset-overview-1"},
				}}, nil
			default:
				return []assetgeneration.Task{{
					TaskID:          requestedTaskID,
					ID:              "amazon:amazon-main",
					Platform:        "amazon",
					RecipeID:        "amazon-main",
					AssetKind:       asset.KindModelImage,
					Slot:            "main",
					Purpose:         "main",
					Status:          "completed",
					ExecutionStatus: "completed",
					ExecutionMode:   assetgeneration.ExecutionModeDeferredStub,
					CanExecute:      true,
					SatisfiedBy:     assetgeneration.ExecutionModeGeneratedAsset,
				}}, nil
			}
		},
		listGenerationReviews: func(ctx context.Context, requestedTaskID string) ([]GenerationReviewRecord, error) {
			return nil, nil
		},
	})

	refresh, err := buildTaskGenerationActionRefreshPhase(generation).run(
		context.Background(),
		taskID,
		&ListingKitResult{
			TaskID: taskID,
			AssetGenerationOverview: &AssetGenerationOverview{
				PrimaryActionKey: "base_result_overview",
			},
			PlatformAssetRenderPreviews: []PlatformAssetRenderPreviews{{
				Platform: "amazon",
				Main: &AssetRenderPreviewSlot{
					Slot:    "main",
					AssetID: "asset-base-1",
				},
				Summary: &PlatformAssetRenderPreviewSummary{TotalPreviews: 1, MainAvailable: true},
			}},
		},
		&GenerationQueueQuery{Platform: "amazon"},
	)
	if err != nil {
		t.Fatalf("taskGenerationActionRefreshPhase.run() error = %v", err)
	}
	if refresh == nil {
		t.Fatal("refresh = nil, want refreshed action payload")
	}
	if refresh.currentResult == nil || refresh.currentResult.AssetGenerationOverview == nil {
		t.Fatalf("currentResult = %+v, want refreshed listing kit result snapshot", refresh.currentResult)
	}
	if refresh.overview == nil || refresh.overview.PrimaryActionKey != refresh.currentResult.AssetGenerationOverview.PrimaryActionKey {
		t.Fatalf("overview = %+v, current overview = %+v, want overview derived from same refreshed snapshot", refresh.overview, refresh.currentResult.AssetGenerationOverview)
	}
	if len(refresh.currentResult.PlatformAssetRenderPreviews) != 1 || len(refresh.platformRenderPreviews) != 1 {
		t.Fatalf("platformRenderPreviews = %+v, current previews = %+v, want synced refreshed previews", refresh.platformRenderPreviews, refresh.currentResult.PlatformAssetRenderPreviews)
	}
	if !reflect.DeepEqual(refresh.platformRenderPreviews, refresh.currentResult.PlatformAssetRenderPreviews) {
		t.Fatalf("platformRenderPreviews = %+v, current previews = %+v, want previews derived from same refreshed snapshot", refresh.platformRenderPreviews, refresh.currentResult.PlatformAssetRenderPreviews)
	}
	if refresh.currentResult.AssetGenerationOverview.PrimaryActionKey != "upgrade_fallback_assets" {
		t.Fatalf("current overview = %+v, want overview from refreshed snapshot", refresh.currentResult.AssetGenerationOverview)
	}
	if len(refresh.currentResult.PlatformAssetRenderPreviews[0].Auxiliary) != 1 || refresh.currentResult.PlatformAssetRenderPreviews[0].Auxiliary[0].AssetID != "asset-overview-1" {
		t.Fatalf("current previews = %+v, want preview payload from refreshed snapshot", refresh.currentResult.PlatformAssetRenderPreviews)
	}
}

func TestTaskGenerationActionRefreshHydratesCurrentResultFallbacks(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	task := &Task{
		ID:        "task-generation-action-refresh-fallbacks-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"amazon"}},
		Result: &ListingKitResult{
			TaskID: "task-generation-action-refresh-fallbacks-1",
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	generation := newTaskGenerationService(taskGenerationServiceConfig{
		repo: repo,
		listAssetGenerationTasks: func(ctx context.Context, requestedTaskID string) ([]assetgeneration.Task, error) {
			return nil, nil
		},
		listGenerationReviews: func(ctx context.Context, requestedTaskID string) ([]GenerationReviewRecord, error) {
			return nil, nil
		},
	})

	baseResult := &ListingKitResult{
		TaskID: "task-generation-action-refresh-fallbacks-1",
		AssetRenderPreviews: []AssetRenderPreview{{
			AssetID:       "asset-fallback-1",
			PreviewFormat: "svg",
			PreviewSVG:    "<svg>fallback</svg>",
			VisualMode:    "selling_point",
			LayerTypes:    []string{"detail"},
		}},
		PlatformAssetRenderPreviews: []PlatformAssetRenderPreviews{{
			Platform: "amazon",
			Main: &AssetRenderPreviewSlot{
				Slot:          "main",
				AssetID:       "asset-fallback-1",
				PreviewFormat: "svg",
				PreviewSVG:    "<svg>fallback</svg>",
			},
			Summary: &PlatformAssetRenderPreviewSummary{TotalPreviews: 1, MainAvailable: true},
		}},
	}

	refresh, err := buildTaskGenerationActionRefreshPhase(generation).run(
		context.Background(),
		task.ID,
		baseResult,
		&GenerationQueueQuery{Platform: "amazon"},
	)
	if err != nil {
		t.Fatalf("taskGenerationActionRefreshPhase.run() error = %v", err)
	}
	if refresh == nil {
		t.Fatal("refresh = nil, want fallback hydration payload")
	}
	if len(refresh.platformRenderPreviews) != 1 || refresh.platformRenderPreviews[0].Main == nil || refresh.platformRenderPreviews[0].Main.AssetID != "asset-fallback-1" {
		t.Fatalf("platformRenderPreviews = %+v, want fallback render previews from base result", refresh.platformRenderPreviews)
	}
	if refresh.currentResult == nil {
		t.Fatal("currentResult = nil, want hydrated listing kit result")
	}
	if len(refresh.currentResult.PlatformAssetRenderPreviews) != 1 || refresh.currentResult.PlatformAssetRenderPreviews[0].Main == nil || refresh.currentResult.PlatformAssetRenderPreviews[0].Main.AssetID != "asset-fallback-1" {
		t.Fatalf("currentResult.PlatformAssetRenderPreviews = %+v, want fallback hydration preserved", refresh.currentResult.PlatformAssetRenderPreviews)
	}
	if len(refresh.currentResult.AssetRenderPreviews) != 1 || refresh.currentResult.AssetRenderPreviews[0].AssetID != "asset-fallback-1" {
		t.Fatalf("currentResult.AssetRenderPreviews = %+v, want base asset render previews hydrated", refresh.currentResult.AssetRenderPreviews)
	}
}

func TestTaskGenerationActionRefreshHydrationFallsBackToBasePlatformRenderPreviews(t *testing.T) {
	t.Parallel()

	baseResult := &ListingKitResult{
		TaskID: "task-generation-action-refresh-hydration-fallback-1",
		PlatformAssetRenderPreviews: []PlatformAssetRenderPreviews{{
			Platform: "amazon",
			Main: &AssetRenderPreviewSlot{
				Slot:          "main",
				AssetID:       "asset-base-1",
				PreviewFormat: "svg",
				PreviewSVG:    "<svg>base</svg>",
			},
			Summary: &PlatformAssetRenderPreviewSummary{TotalPreviews: 1, MainAvailable: true},
		}},
	}
	refresh := &taskGenerationActionRefreshExtractResult{
		overview: &AssetGenerationOverview{PrimaryActionKey: "refresh"},
		currentResult: &ListingKitResult{
			TaskID: "task-generation-action-refresh-hydration-fallback-1",
		},
	}

	result := buildTaskGenerationActionRefreshHydrationPhase().run(baseResult, refresh)
	if result == nil {
		t.Fatal("result = nil, want hydrated refresh result")
	}
	if len(result.platformRenderPreviews) != 1 || result.platformRenderPreviews[0].Main == nil || result.platformRenderPreviews[0].Main.AssetID != "asset-base-1" {
		t.Fatalf("platformRenderPreviews = %+v, want base platform previews when refreshed state is sparse", result.platformRenderPreviews)
	}
}

func TestTaskGenerationActionRefreshHydrationBackfillsCurrentPlatformRenderPreviews(t *testing.T) {
	t.Parallel()

	baseResult := &ListingKitResult{
		TaskID: "task-generation-action-refresh-hydration-platform-backfill-1",
		PlatformAssetRenderPreviews: []PlatformAssetRenderPreviews{{
			Platform: "amazon",
			Main: &AssetRenderPreviewSlot{
				Slot:          "main",
				AssetID:       "asset-base-platform-1",
				PreviewFormat: "svg",
				PreviewSVG:    "<svg>platform</svg>",
			},
			Summary: &PlatformAssetRenderPreviewSummary{TotalPreviews: 1, MainAvailable: true},
		}},
	}
	refresh := &taskGenerationActionRefreshExtractResult{
		overview: &AssetGenerationOverview{PrimaryActionKey: "refresh"},
		currentResult: &ListingKitResult{
			TaskID: "task-generation-action-refresh-hydration-platform-backfill-1",
		},
	}

	result := buildTaskGenerationActionRefreshHydrationPhase().run(baseResult, refresh)
	if result == nil || result.currentResult == nil {
		t.Fatalf("result = %+v, want hydrated current result", result)
	}
	if len(result.currentResult.PlatformAssetRenderPreviews) != 1 || result.currentResult.PlatformAssetRenderPreviews[0].Main == nil || result.currentResult.PlatformAssetRenderPreviews[0].Main.AssetID != "asset-base-platform-1" {
		t.Fatalf("currentResult.PlatformAssetRenderPreviews = %+v, want platform render previews backfilled from hydrated result", result.currentResult.PlatformAssetRenderPreviews)
	}
}

func TestTaskGenerationActionRefreshHydrationBackfillsCurrentAssetRenderPreviewsFromBase(t *testing.T) {
	t.Parallel()

	baseResult := &ListingKitResult{
		TaskID: "task-generation-action-refresh-hydration-asset-backfill-1",
		AssetRenderPreviews: []AssetRenderPreview{{
			AssetID:       "asset-base-render-1",
			PreviewFormat: "svg",
			PreviewSVG:    "<svg>asset</svg>",
			VisualMode:    "selling_point",
			LayerTypes:    []string{"detail"},
		}},
	}
	refresh := &taskGenerationActionRefreshExtractResult{
		overview: &AssetGenerationOverview{PrimaryActionKey: "refresh"},
		currentResult: &ListingKitResult{
			TaskID: "task-generation-action-refresh-hydration-asset-backfill-1",
		},
	}

	result := buildTaskGenerationActionRefreshHydrationPhase().run(baseResult, refresh)
	if result == nil || result.currentResult == nil {
		t.Fatalf("result = %+v, want hydrated current result", result)
	}
	if len(result.currentResult.AssetRenderPreviews) != 1 || result.currentResult.AssetRenderPreviews[0].AssetID != "asset-base-render-1" {
		t.Fatalf("currentResult.AssetRenderPreviews = %+v, want base asset render previews backfilled", result.currentResult.AssetRenderPreviews)
	}
}

func TestTaskGenerationServiceFileDelegatesActionExecution(t *testing.T) {
	t.Parallel()

	actionSource := readExecuteTaskGenerationActionSource(t)

	required := []string{
		"buildTaskGenerationActionExecutePhase(s).run(",
		"buildTaskGenerationActionRefreshPhase(s).run(",
	}
	for _, needle := range required {
		if !strings.Contains(actionSource, needle) {
			t.Fatalf("ExecuteTaskGenerationAction should contain %q", needle)
		}
	}

	forbidden := []string{
		"result.Overview, err = s.getCurrentAssetGenerationOverview(ctx, taskID)",
		"result.PlatformRenderPreviews, err = s.getCurrentActionRenderPreviews(ctx, taskID, target.QueueQuery)",
		"buildActionPlatformRenderPreviews(baseResult, target.QueueQuery)",
	}
	for _, needle := range forbidden {
		if strings.Contains(actionSource, needle) {
			t.Fatalf("ExecuteTaskGenerationAction should not inline execution branching %q", needle)
		}
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
	if len(assetRepository.calls) != 0 {
		t.Fatalf("persistence calls = %+v, want no persistence when dispatch result is nil", assetRepository.calls)
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

func TestRetryTaskGenerationTasksPersistenceFailureStopsRetry(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("save generation tasks failed")
	fixture := newRetryPersistenceFailureFixture(t, "task-generation-retry-persist-fail-1")
	fixture.assetRepository.saveGenerationTasksErr = wantErr
	fixture.assetRepository.resetCalls()

	page, err := fixture.generation.RetryTaskGenerationTasks(context.Background(), fixture.taskID, &RetryGenerationTasksRequest{
		Slots: []string{"auxiliary"},
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("RetryTaskGenerationTasks() error = %v, want %v", err, wantErr)
	}
	if page != nil {
		t.Fatalf("page = %+v, want nil on persistence failure", page)
	}
	if !reflect.DeepEqual(fixture.assetRepository.calls, []string{"save_inventory", "save_generation_tasks"}) {
		t.Fatalf("persistence calls = %+v, want inventory then generation tasks before failing", fixture.assetRepository.calls)
	}
	if fixture.repo.saveTaskResultCalls != 0 {
		t.Fatalf("SaveTaskResult() calls = %d, want 0 after persistence failure", fixture.repo.saveTaskResultCalls)
	}
}

func TestRetryTaskGenerationTasksInventoryPersistenceFailureStopsRetry(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("save inventory failed")
	fixture := newRetryPersistenceFailureFixture(t, "task-generation-retry-inventory-persist-fail-1")
	fixture.assetRepository.saveInventoryErr = wantErr
	fixture.assetRepository.resetCalls()

	page, err := fixture.generation.RetryTaskGenerationTasks(context.Background(), fixture.taskID, &RetryGenerationTasksRequest{
		Slots: []string{"auxiliary"},
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("RetryTaskGenerationTasks() error = %v, want %v", err, wantErr)
	}
	if page != nil {
		t.Fatalf("page = %+v, want nil on inventory persistence failure", page)
	}
	if !reflect.DeepEqual(fixture.assetRepository.calls, []string{"save_inventory"}) {
		t.Fatalf("persistence calls = %+v, want inventory save only before failing", fixture.assetRepository.calls)
	}
	if fixture.repo.saveTaskResultCalls != 0 {
		t.Fatalf("SaveTaskResult() calls = %d, want 0 after inventory persistence failure", fixture.repo.saveTaskResultCalls)
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
		repo: repo, mirrors: serviceDependencyMirrors{assetRepo: assetRepository, assetRecipeResolver: assetrecipe.NewStaticResolver(), assetBundleBuilder: assetbundle.NewBuilder(), assetGenerator: assetgeneration.NewService(assetgeneration.Config{})},
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
		repo: repo, mirrors: serviceDependencyMirrors{assetRepo: assetRepository, assetRecipeResolver: assetrecipe.NewStaticResolver(), assetBundleBuilder: assetbundle.NewBuilder(), assetGenerator: assetgeneration.NewService(assetgeneration.Config{
			DeferredRenderer: renderer,
		})},
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

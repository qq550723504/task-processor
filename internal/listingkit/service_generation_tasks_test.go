package listingkit

import (
	"context"
	"testing"
	"time"

	"task-processor/internal/asset"
	assetbundle "task-processor/internal/asset/bundle"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
	assetrepo "task-processor/internal/asset/repository"
	"task-processor/internal/catalog"
)

func TestGetTaskPreviewIncludesGenerationTasks(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	assetRepository := assetrepo.NewMemRepository()
	svc := &service{
		repo: repo, mirrors: serviceDependencyMirrors{assetRepo: assetRepository, assetRecipeResolver: assetrecipe.NewStaticResolver(), assetBundleBuilder: assetbundle.NewBuilder(), assetGenerator: assetgeneration.NewService(assetgeneration.Config{})},
	}

	task := &Task{
		ID:        "task-preview-generation-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"amazon"}},
		Result: &ListingKitResult{
			TaskID:         "task-preview-generation-1",
			Platforms:      []string{"amazon"},
			CatalogProduct: &catalog.Product{Title: "Portable Speaker"},
			Amazon:         &AmazonPackage{},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	tasks := []assetgeneration.Task{{
		TaskID:          task.ID,
		ID:              "amazon:amazon-lifestyle",
		Platform:        "amazon",
		RecipeID:        "amazon-lifestyle",
		AssetKind:       asset.KindSceneImage,
		ExecutionStatus: "completed",
		ExecutionMode:   assetgeneration.ExecutionModeRendererBacked,
		CanExecute:      true,
	}}
	if err := assetRepository.SaveGenerationTasks(context.Background(), task.ID, tasks); err != nil {
		t.Fatalf("SaveGenerationTasks() error = %v", err)
	}

	preview, err := svc.GetTaskPreview(context.Background(), task.ID, "amazon")
	if err != nil {
		t.Fatalf("GetTaskPreview() error = %v", err)
	}
	if preview.AssetGenerationSummary == nil || preview.AssetGenerationSummary.RendererBackedTasks != 1 {
		t.Fatalf("preview asset generation summary = %+v, want renderer_backed_tasks=1", preview.AssetGenerationSummary)
	}
	if len(preview.AssetGenerationTasks) != 1 || preview.AssetGenerationTasks[0].ID != "amazon:amazon-lifestyle" {
		t.Fatalf("preview asset generation tasks = %+v, want persisted generation task", preview.AssetGenerationTasks)
	}
}

func TestGetTaskGenerationTasksAppliesQueryFiltersAndRebuildsSummary(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	assetRepository := assetrepo.NewMemRepository()
	svc := &service{
		repo: repo, mirrors: serviceDependencyMirrors{assetRepo: assetRepository},
	}

	task := &Task{
		ID:        "task-generation-query-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"amazon", "shein"}},
		Result:    &ListingKitResult{TaskID: "task-generation-query-1"},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	tasks := []assetgeneration.Task{
		{TaskID: task.ID, ID: "amazon:amazon-lifestyle", Platform: "amazon", Slot: "auxiliary", ExecutionMode: assetgeneration.ExecutionModeRendererBacked, ExecutionStatus: "completed", SatisfiedBy: assetgeneration.ExecutionModeGeneratedAsset, CanExecute: true},
		{TaskID: task.ID, ID: "shein:shein-main-model", Platform: "shein", Slot: "main", ExecutionMode: assetgeneration.ExecutionModeDeferredStub, ExecutionStatus: "completed", SatisfiedBy: "fallback_asset", CanExecute: true},
	}
	if err := assetRepository.SaveGenerationTasks(context.Background(), task.ID, tasks); err != nil {
		t.Fatalf("SaveGenerationTasks() error = %v", err)
	}

	page, err := svc.GetTaskGenerationTasks(context.Background(), task.ID, &GenerationTaskQuery{
		Platform:    "shein",
		Slot:        "main",
		SatisfiedBy: "fallback_asset",
	})
	if err != nil {
		t.Fatalf("GetTaskGenerationTasks() error = %v", err)
	}
	if len(page.Tasks) != 1 {
		t.Fatalf("tasks = %+v, want 1 filtered task", page.Tasks)
	}
	if page.Tasks[0].ID != "shein:shein-main-model" {
		t.Fatalf("task = %+v, want shein main", page.Tasks[0])
	}
	if page.Summary == nil || page.Summary.TotalTasks != 1 || page.Summary.FallbackTasks != 1 {
		t.Fatalf("summary = %+v, want filtered fallback summary", page.Summary)
	}
}

func TestGetTaskGenerationTasksAppliesPaginationAndSorting(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	assetRepository := assetrepo.NewMemRepository()
	svc := &service{
		repo: repo, mirrors: serviceDependencyMirrors{assetRepo: assetRepository},
	}

	task := &Task{
		ID:        "task-generation-query-page-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"amazon", "shein"}},
		Result:    &ListingKitResult{TaskID: "task-generation-query-page-1"},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	tasks := []assetgeneration.Task{
		{TaskID: task.ID, ID: "shein:shein-main-model", Platform: "shein", Slot: "main", ExecutionMode: assetgeneration.ExecutionModeDeferredStub, ExecutionStatus: "completed", SatisfiedBy: "fallback_asset", CanExecute: true},
		{TaskID: task.ID, ID: "amazon:amazon-lifestyle", Platform: "amazon", Slot: "auxiliary", ExecutionMode: assetgeneration.ExecutionModeRendererBacked, ExecutionStatus: "completed", SatisfiedBy: assetgeneration.ExecutionModeGeneratedAsset, CanExecute: true},
		{TaskID: task.ID, ID: "amazon:amazon-main-white-bg", Platform: "amazon", Slot: "main", ExecutionMode: assetgeneration.ExecutionModePipelineBacked, ExecutionStatus: "planned", SatisfiedBy: "", CanExecute: true},
	}
	if err := assetRepository.SaveGenerationTasks(context.Background(), task.ID, tasks); err != nil {
		t.Fatalf("SaveGenerationTasks() error = %v", err)
	}

	page, err := svc.GetTaskGenerationTasks(context.Background(), task.ID, &GenerationTaskQuery{
		Page:      2,
		PageSize:  1,
		SortBy:    "platform",
		SortOrder: "asc",
	})
	if err != nil {
		t.Fatalf("GetTaskGenerationTasks() error = %v", err)
	}
	if page.Total != 3 {
		t.Fatalf("total = %d, want 3", page.Total)
	}
	if page.Page != 2 || page.PageSize != 1 {
		t.Fatalf("page meta = %+v, want page=2 page_size=1", page)
	}
	if len(page.Tasks) != 1 {
		t.Fatalf("tasks = %+v, want 1 paged task", page.Tasks)
	}
	if page.Tasks[0].ID != "amazon:amazon-main-white-bg" {
		t.Fatalf("task = %+v, want second task in sorted order", page.Tasks[0])
	}
	if page.Summary == nil || page.Summary.TotalTasks != 3 {
		t.Fatalf("summary = %+v, want full filtered summary", page.Summary)
	}
}

func TestTaskGenerationTasksServiceBoundaryGuardrails(t *testing.T) {
	t.Parallel()

	source := readExactMethodSource(t, "task_generation_service.go", "func (s *taskGenerationService) GetTaskGenerationTasks(")

	assertSourceOccurrenceCount(t, source, "buildTaskGenerationTasksReadSnapshotPhase(s).run(", 1)
	assertSourceExcludesAll(t, source, []string{
		"repo.GetTask(",
		"listAssetGenerationTasks(",
		"getCurrentListingKitResult(",
		"withListingKitResultGenerationAndReview(",
	})
}

func TestTaskGenerationTasksReadSnapshotPhaseBoundary(t *testing.T) {
	t.Parallel()

	source := readExactMethodSource(t, "task_generation_tasks_read_snapshot.go", "func (p *taskGenerationTasksReadSnapshotPhase) run(")

	assertSourceContainsAll(t, source, []string{
		"p.service.repo.GetTask(",
		"p.service.listAssetGenerationTasks(",
		"task:  task,",
		"tasks: tasks,",
	})
	assertSourceExcludesAll(t, source, []string{
		"filterGenerationTasks(",
		"sortGenerationTasks(",
		"paginateGenerationTasks(",
		"buildGenerationTaskPage(",
		"getCurrentListingKitResult(",
		"withListingKitResultGenerationAndReview(",
	})
}

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
	"task-processor/internal/productenrich"
	common "task-processor/internal/publishing/common"
)

type stubServiceDeferredRenderer struct {
	result *asset.AssetRecord
}

type stubGenerationRepo struct {
	task *Task
}

func (r *stubGenerationRepo) CreateTask(ctx context.Context, task *Task) error {
	copied := *task
	r.task = &copied
	return nil
}

func (r *stubGenerationRepo) GetTask(ctx context.Context, taskID string) (*Task, error) {
	if r.task == nil || r.task.ID != taskID {
		return nil, ErrTaskNotFound
	}
	copied := *r.task
	return &copied, nil
}

func (r *stubGenerationRepo) ListTasks(ctx context.Context, query *TaskListQuery) ([]Task, int64, error) {
	if r.task == nil {
		return []Task{}, 0, nil
	}
	copied := *r.task
	return []Task{copied}, 1, nil
}

func (r *stubGenerationRepo) MarkProcessing(ctx context.Context, taskID string) error { return nil }
func (r *stubGenerationRepo) MarkCompleted(ctx context.Context, taskID string, result *ListingKitResult) error {
	return r.SaveTaskResult(ctx, taskID, result)
}
func (r *stubGenerationRepo) MarkNeedsReview(ctx context.Context, taskID string, result *ListingKitResult, reason string) error {
	if err := r.SaveTaskResult(ctx, taskID, result); err != nil {
		return err
	}
	r.task.Status = TaskStatusNeedsReview
	r.task.Error = reason
	return nil
}
func (r *stubGenerationRepo) MarkFailed(ctx context.Context, taskID string, errorMsg string) error {
	return nil
}
func (r *stubGenerationRepo) PrepareRetry(ctx context.Context, taskID string) error { return nil }
func (r *stubGenerationRepo) IncrementRetryCount(ctx context.Context, taskID string) error {
	return nil
}
func (r *stubGenerationRepo) SaveTaskResult(ctx context.Context, taskID string, result *ListingKitResult) error {
	if r.task == nil || r.task.ID != taskID {
		return ErrTaskNotFound
	}
	r.task.Result = result
	r.task.UpdatedAt = time.Now()
	return nil
}

func (s *stubServiceDeferredRenderer) Render(ctx context.Context, req assetgeneration.DeferredRenderRequest) (*asset.AssetRecord, error) {
	return s.result, nil
}

func TestGetTaskPreviewIncludesGenerationTasks(t *testing.T) {
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
		repo:      repo,
		assetRepo: assetRepository,
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
		repo:      repo,
		assetRepo: assetRepository,
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

func TestGetTaskGenerationQueueReturnsNotModifiedWhenDeltaMatches(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	assetRepository := assetrepo.NewMemRepository()
	svc := &service{
		repo:      repo,
		assetRepo: assetRepository,
	}

	task := &Task{
		ID:        "task-generation-queue-delta-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"shein"}},
		Result: &ListingKitResult{
			TaskID: "task-generation-queue-delta-1",
			Shein: &SheinPackage{ImageBundle: &common.PublishImageBundle{
				Platform: "shein",
				Main: &common.BundleSlot{
					Key:           "main",
					AssetID:       "asset-preview-1",
					StateLabel:    "ready",
					TemplateLabel: "SHEIN Main",
				},
			}},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	first, err := svc.GetTaskGenerationQueue(context.Background(), task.ID, &GenerationQueueQuery{
		Platform: "shein",
	})
	if err != nil {
		t.Fatalf("GetTaskGenerationQueue() first call error = %v", err)
	}
	if first == nil || first.DeltaToken == "" {
		t.Fatalf("first response = %+v, want queue page with delta token", first)
	}

	second, err := svc.GetTaskGenerationQueue(context.Background(), task.ID, &GenerationQueueQuery{
		Platform:   "shein",
		DeltaToken: first.DeltaToken,
	})
	if err != nil {
		t.Fatalf("GetTaskGenerationQueue() second call error = %v", err)
	}
	if second == nil || !second.NotModified || second.DeltaToken != first.DeltaToken || second.Summary != nil || len(second.Items) != 0 {
		t.Fatalf("second response = %+v, want not_modified queue response", second)
	}
}

func TestBuildGenerationWorkQueueBuildsReadyFallbackAndStubbedStates(t *testing.T) {
	t.Parallel()

	result := &ListingKitResult{
		AssetRenderPreviews: []AssetRenderPreview{
			{
				AssetID:             "fallback-main-1",
				PreviewFormat:       "svg",
				PreviewSVG:          "<svg/>",
				VisualMode:          "selling_point",
				LayoutEngine:        "selling_point_output_v2",
				RenderOutputVersion: "v2",
				LayerTypes:          []string{"background", "badge", "text"},
				Regions:             []string{"full_canvas", "title_band", "body_copy"},
				StyleTokens:         []string{"bg-soft", "badge-dark", "copy-primary"},
			},
		},
		AssetGenerationTasks: []assetgeneration.Task{
			{
				ID:              "shein:shein-main-model",
				Platform:        "shein",
				RecipeID:        "shein-main-model",
				Slot:            "main",
				Purpose:         "main",
				AssetKind:       asset.KindModelImage,
				TemplateLabel:   "SHEIN Editorial Main",
				RenderProfile:   "shein_model_editorial",
				ExecutionStatus: "completed",
				ExecutionMode:   assetgeneration.ExecutionModeDeferredStub,
				SatisfiedBy:     assetgeneration.ExecutionModeGeneratedAsset,
				CanExecute:      true,
			},
		},
		Amazon: &AmazonPackage{
			ImageBundle: &common.PublishImageBundle{
				Platform: "amazon",
				Main: &common.BundleSlot{
					Key:             "main",
					Purpose:         "main",
					IdealKind:       string(asset.KindWhiteBgImage),
					TemplateLabel:   "Amazon White Background Main",
					AssetID:         "white-1",
					RecipeID:        "amazon-main-white-bg",
					StateLabel:      "ready",
					SatisfiedBy:     "exact_asset",
					ExecutionStatus: "ready",
				},
				MissingSlots: []common.MissingSlot{{
					Slot:          "auxiliary",
					Purpose:       "scene",
					RecipeID:      "amazon-lifestyle",
					TemplateLabel: "Amazon Lifestyle Scene",
					RenderProfile: "amazon_lifestyle_scene",
					StateLabel:    "missing",
				}},
			},
		},
		Shein: &SheinPackage{
			ImageBundle: &common.PublishImageBundle{
				Platform: "shein",
				Main: &common.BundleSlot{
					Key:             "main",
					Purpose:         "main",
					IdealKind:       string(asset.KindModelImage),
					TemplateLabel:   "SHEIN Editorial Main",
					AssetID:         "fallback-main-1",
					RecipeID:        "shein-main-model",
					StateLabel:      "fallback_in_use",
					SatisfiedBy:     "fallback_asset",
					FallbackFrom:    string(asset.KindModelImage),
					ExecutionStatus: "fallback",
				},
			},
		},
	}

	queue := buildGenerationWorkQueue(result)
	if queue == nil {
		t.Fatal("expected generation work queue")
	}
	if queue.Summary == nil || queue.Summary.TotalItems != 3 {
		t.Fatalf("queue summary = %+v, want 3 items", queue.Summary)
	}
	if len(queue.Items) != 3 {
		t.Fatalf("queue items = %+v, want 3", queue.Items)
	}
	if queue.Items[0].State != "ready" {
		t.Fatalf("first queue item = %+v, want ready", queue.Items[0])
	}
	if queue.Items[1].State != "missing" {
		t.Fatalf("second queue item = %+v, want missing", queue.Items[1])
	}
	if queue.Items[2].State != "stubbed" || !queue.Items[2].Retryable {
		t.Fatalf("third queue item = %+v, want stubbed retryable", queue.Items[2])
	}
	if !queue.Items[2].RenderPreviewAvailable || queue.Items[2].RenderPreviewFormat != "svg" {
		t.Fatalf("third queue item = %+v, want render preview summary", queue.Items[2])
	}
	if queue.Summary.PreviewableItems != 1 || queue.Summary.PlatformPreviewableCounts["shein"] != 1 {
		t.Fatalf("queue summary = %+v, want previewable summary", queue.Summary)
	}
}

func TestBuildGenerationWorkQueueUsesPendingGenerationWithoutPersistedTask(t *testing.T) {
	t.Parallel()

	result := &ListingKitResult{
		Amazon: &AmazonPackage{
			ImageBundle: &common.PublishImageBundle{
				Platform: "amazon",
				MissingSlots: []common.MissingSlot{{
					Slot:          "auxiliary",
					Purpose:       "scene",
					RecipeID:      "amazon-lifestyle",
					TemplateLabel: "Amazon Lifestyle Scene",
					RenderProfile: "amazon_lifestyle_scene",
					StateLabel:    "missing",
				}},
				PendingGeneration: []assetgeneration.Task{{
					ID:              "amazon:amazon-lifestyle",
					Platform:        "amazon",
					RecipeID:        "amazon-lifestyle",
					AssetKind:       asset.KindSceneImage,
					Slot:            "auxiliary",
					Purpose:         "scene",
					TemplateLabel:   "Amazon Lifestyle Scene",
					RenderProfile:   "amazon_lifestyle_scene",
					ExecutionStatus: "planned",
					ExecutionMode:   assetgeneration.ExecutionModeRendererBacked,
					CanExecute:      true,
				}},
			},
		},
	}

	queue := buildGenerationWorkQueue(result)
	if queue == nil || len(queue.Items) != 1 {
		t.Fatalf("queue = %+v, want 1 queued item", queue)
	}
	if queue.Items[0].State != "queued" || !queue.Items[0].Retryable {
		t.Fatalf("queue item = %+v, want queued retryable", queue.Items[0])
	}
}

func TestGetTaskGenerationQueueAppliesFilteringSortingAndPaging(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	assetRepository := assetrepo.NewMemRepository()
	svc := &service{
		repo:      repo,
		assetRepo: assetRepository,
	}

	task := &Task{
		ID:        "task-generation-queue-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"amazon", "shein"}},
		Result: &ListingKitResult{
			TaskID: "task-generation-queue-1",
			Amazon: &AmazonPackage{
				ImageBundle: &common.PublishImageBundle{
					Platform: "amazon",
					Main: &common.BundleSlot{
						Key:             "main",
						Purpose:         "main",
						RecipeID:        "amazon-main-white-bg",
						IdealKind:       string(asset.KindWhiteBgImage),
						StateLabel:      "ready",
						SatisfiedBy:     "exact_asset",
						ExecutionStatus: "ready",
					},
				},
			},
			Shein: &SheinPackage{
				ImageBundle: &common.PublishImageBundle{
					Platform: "shein",
					Main: &common.BundleSlot{
						Key:             "main",
						Purpose:         "main",
						RecipeID:        "shein-main-model",
						IdealKind:       string(asset.KindModelImage),
						StateLabel:      "fallback_in_use",
						SatisfiedBy:     "fallback_asset",
						ExecutionStatus: "fallback",
					},
				},
			},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	tasks := []assetgeneration.Task{
		{TaskID: task.ID, ID: "shein:shein-main-model", Platform: "shein", RecipeID: "shein-main-model", Slot: "main", ExecutionMode: assetgeneration.ExecutionModeDeferredStub, ExecutionStatus: "completed", CanExecute: true},
		{TaskID: task.ID, ID: "amazon:amazon-main-white-bg", Platform: "amazon", RecipeID: "amazon-main-white-bg", Slot: "main", ExecutionMode: assetgeneration.ExecutionModePipelineBacked, ExecutionStatus: "completed", CanExecute: true},
	}
	if err := assetRepository.SaveGenerationTasks(context.Background(), task.ID, tasks); err != nil {
		t.Fatalf("SaveGenerationTasks() error = %v", err)
	}

	page, err := svc.GetTaskGenerationQueue(context.Background(), task.ID, &GenerationQueueQuery{
		State:     "stubbed",
		Page:      1,
		PageSize:  10,
		SortBy:    "platform",
		SortOrder: "asc",
	})
	if err != nil {
		t.Fatalf("GetTaskGenerationQueue() error = %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("page = %+v, want one filtered queue item", page)
	}
	if page.Items[0].Platform != "shein" || page.Items[0].State != "stubbed" {
		t.Fatalf("queue item = %+v, want shein stubbed item", page.Items[0])
	}
	if page.Summary == nil || page.Summary.StubbedItems != 1 || page.Summary.TotalItems != 1 {
		t.Fatalf("summary = %+v, want filtered stubbed summary", page.Summary)
	}
}

func TestGetTaskGenerationQueueFiltersByExecutionQuality(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	assetRepository := assetrepo.NewMemRepository()
	svc := &service{
		repo:      repo,
		assetRepo: assetRepository,
	}

	task := &Task{
		ID:        "task-generation-queue-quality-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"amazon", "shein"}},
		Result: &ListingKitResult{
			TaskID: "task-generation-queue-quality-1",
			Amazon: &AmazonPackage{
				ImageBundle: &common.PublishImageBundle{
					Platform: "amazon",
					Main: &common.BundleSlot{
						Key:             "main",
						Purpose:         "main",
						RecipeID:        "amazon-main-white-bg",
						IdealKind:       string(asset.KindWhiteBgImage),
						StateLabel:      "ready",
						SatisfiedBy:     "exact_asset",
						ExecutionStatus: "ready",
					},
				},
			},
			Shein: &SheinPackage{
				ImageBundle: &common.PublishImageBundle{
					Platform: "shein",
					Main: &common.BundleSlot{
						Key:             "main",
						Purpose:         "main",
						RecipeID:        "shein-main-model",
						IdealKind:       string(asset.KindModelImage),
						StateLabel:      "fallback_in_use",
						SatisfiedBy:     "fallback_asset",
						ExecutionStatus: "fallback",
					},
				},
			},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	tasks := []assetgeneration.Task{
		{TaskID: task.ID, ID: "shein:shein-main-model", Platform: "shein", RecipeID: "shein-main-model", Slot: "main", ExecutionMode: assetgeneration.ExecutionModeDeferredStub, ExecutionStatus: "completed", CanExecute: true},
		{TaskID: task.ID, ID: "amazon:amazon-main-white-bg", Platform: "amazon", RecipeID: "amazon-main-white-bg", Slot: "main", ExecutionMode: assetgeneration.ExecutionModePipelineBacked, ExecutionStatus: "completed", CanExecute: true},
	}
	if err := assetRepository.SaveGenerationTasks(context.Background(), task.ID, tasks); err != nil {
		t.Fatalf("SaveGenerationTasks() error = %v", err)
	}

	page, err := svc.GetTaskGenerationQueue(context.Background(), task.ID, &GenerationQueueQuery{
		ExecutionQuality: "stub_fallback",
		SortBy:           "execution_quality",
		SortOrder:        "asc",
	})
	if err != nil {
		t.Fatalf("GetTaskGenerationQueue() error = %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("page = %+v, want one filtered queue item", page)
	}
	if page.Items[0].ExecutionQuality != "stub_fallback" {
		t.Fatalf("queue item = %+v, want stub_fallback quality", page.Items[0])
	}
}

func TestGetTaskGenerationQueueFiltersByRenderPreviewAvailability(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	assetRepository := assetrepo.NewMemRepository()
	svc := &service{
		repo:      repo,
		assetRepo: assetRepository,
	}

	task := &Task{
		ID:        "task-generation-queue-preview-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"amazon", "shein"}},
		Result: &ListingKitResult{
			TaskID: "task-generation-queue-preview-1",
			AssetRenderPreviews: []AssetRenderPreview{
				{
					AssetID:             "fallback-main-1",
					PreviewFormat:       "svg",
					PreviewSVG:          "<svg/>",
					VisualMode:          "selling_point",
					LayoutEngine:        "selling_point_output_v2",
					RenderOutputVersion: "v2",
					LayerTypes:          []string{"background", "badge", "text", "spec", "detail"},
					Regions:             []string{"full_canvas", "title_band", "body_copy"},
					StyleTokens:         []string{"bg-soft", "badge-dark", "copy-primary"},
				},
			},
			Amazon: &AmazonPackage{
				ImageBundle: &common.PublishImageBundle{
					Platform: "amazon",
					Main: &common.BundleSlot{
						Key:             "main",
						Purpose:         "main",
						RecipeID:        "amazon-main-white-bg",
						IdealKind:       string(asset.KindWhiteBgImage),
						StateLabel:      "ready",
						SatisfiedBy:     "exact_asset",
						ExecutionStatus: "ready",
						AssetID:         "white-1",
					},
				},
			},
			Shein: &SheinPackage{
				ImageBundle: &common.PublishImageBundle{
					Platform: "shein",
					Main: &common.BundleSlot{
						Key:             "main",
						Purpose:         "main",
						RecipeID:        "shein-main-model",
						IdealKind:       string(asset.KindModelImage),
						StateLabel:      "fallback_in_use",
						SatisfiedBy:     "fallback_asset",
						ExecutionStatus: "fallback",
						AssetID:         "fallback-main-1",
					},
				},
			},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	page, err := svc.GetTaskGenerationQueue(context.Background(), task.ID, &GenerationQueueQuery{
		RenderPreviewAvailable:        true,
		RenderPreviewAvailablePresent: true,
		PreviewCapability:             "detail_preview",
	})
	if err != nil {
		t.Fatalf("GetTaskGenerationQueue() error = %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("page = %+v, want one previewable queue item", page)
	}
	if !page.Items[0].RenderPreviewAvailable {
		t.Fatalf("queue item = %+v, want render_preview_available", page.Items[0])
	}
	if len(page.Items[0].PreviewCapabilities) == 0 || page.Items[0].PreviewCapabilities[0] == "" {
		t.Fatalf("queue item = %+v, want preview capabilities", page.Items[0])
	}
	if page.Summary == nil || page.Summary.PreviewableItems != 1 || page.Summary.PreviewCapabilityCounts["detail_preview"] != 1 {
		t.Fatalf("summary = %+v, want preview capability summary", page.Summary)
	}
}

func TestGetTaskGenerationQueueBuildsOperationalSummaryAndTemplateSort(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	assetRepository := assetrepo.NewMemRepository()
	svc := &service{
		repo:      repo,
		assetRepo: assetRepository,
	}

	task := &Task{
		ID:        "task-generation-queue-summary-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"amazon", "shein"}},
		Result: &ListingKitResult{
			TaskID: "task-generation-queue-summary-1",
			Amazon: &AmazonPackage{
				ImageBundle: &common.PublishImageBundle{
					Platform: "amazon",
					Main: &common.BundleSlot{
						Key:             "main",
						Purpose:         "main",
						RecipeID:        "amazon-main-white-bg",
						IdealKind:       string(asset.KindWhiteBgImage),
						TemplateLabel:   "A Main",
						StateLabel:      "ready",
						SatisfiedBy:     "exact_asset",
						ExecutionStatus: "ready",
						AssetID:         "white-1",
						RetryHint:       "",
					},
					MissingSlots: []common.MissingSlot{{
						Slot:          "auxiliary",
						Purpose:       "scene",
						RecipeID:      "amazon-lifestyle",
						TemplateLabel: "Z Lifestyle",
						RenderProfile: "amazon_lifestyle_scene",
						StateLabel:    "missing",
						Reason:        "scene asset not generated yet",
					}},
				},
			},
			Shein: &SheinPackage{
				ImageBundle: &common.PublishImageBundle{
					Platform: "shein",
					Main: &common.BundleSlot{
						Key:             "main",
						Purpose:         "main",
						RecipeID:        "shein-main-model",
						IdealKind:       string(asset.KindModelImage),
						TemplateLabel:   "B Editorial",
						StateLabel:      "fallback_in_use",
						SatisfiedBy:     "fallback_asset",
						FallbackFrom:    string(asset.KindModelImage),
						ExecutionStatus: "fallback",
						AssetID:         "fallback-main-1",
						RetryHint:       "retry renderer-backed generation",
					},
				},
			},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	page, err := svc.GetTaskGenerationQueue(context.Background(), task.ID, &GenerationQueueQuery{
		Page:      1,
		PageSize:  10,
		SortBy:    "template_label",
		SortOrder: "asc",
	})
	if err != nil {
		t.Fatalf("GetTaskGenerationQueue() error = %v", err)
	}
	if page.Summary == nil {
		t.Fatal("expected queue summary")
	}
	if page.Summary.PlatformCounts["amazon"] != 2 || page.Summary.PlatformCounts["shein"] != 1 {
		t.Fatalf("platform counts = %+v, want amazon=2 shein=1", page.Summary.PlatformCounts)
	}
	if page.Summary.PlatformStateCounts["amazon"]["ready"] != 1 || page.Summary.PlatformStateCounts["amazon"]["missing"] != 1 || page.Summary.PlatformStateCounts["shein"]["fallback_in_use"] != 1 {
		t.Fatalf("platform state counts = %+v, want grouped platform/state counts", page.Summary.PlatformStateCounts)
	}
	if page.Summary.StateCounts["ready"] != 1 || page.Summary.StateCounts["fallback_in_use"] != 1 || page.Summary.StateCounts["missing"] != 1 {
		t.Fatalf("state counts = %+v, want ready=1 fallback_in_use=1 missing=1", page.Summary.StateCounts)
	}
	if len(page.Items) != 3 {
		t.Fatalf("items = %+v, want 3", page.Items)
	}
	if page.Items[0].TemplateLabel != "A Main" || page.Items[1].TemplateLabel != "B Editorial" || page.Items[2].TemplateLabel != "Z Lifestyle" {
		t.Fatalf("items = %+v, want template_label ascending order", page.Items)
	}
	if page.Items[1].RetryHint == "" || page.Items[1].SelectedAssetID != "fallback-main-1" || page.Items[1].TargetAssetKind != string(asset.KindModelImage) {
		t.Fatalf("fallback item = %+v, want operational fields populated", page.Items[1])
	}
	if page.Items[2].StateReason != "scene asset not generated yet" {
		t.Fatalf("missing item = %+v, want state reason", page.Items[2])
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
			CanonicalProduct: &productenrich.CanonicalProduct{CategoryPath: []string{"Electronics", "Audio"}},
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

func TestGetTaskGenerationQueueFiltersByQualityGrade(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	assetRepository := assetrepo.NewMemRepository()
	svc := &service{
		repo:      repo,
		assetRepo: assetRepository,
	}

	task := &Task{
		ID:        "task-generation-queue-grade-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"amazon", "shein"}},
		Result: &ListingKitResult{
			TaskID: "task-generation-queue-grade-1",
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
			Shein: &SheinPackage{
				ImageBundle: &common.PublishImageBundle{
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
				},
			},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	page, err := svc.GetTaskGenerationQueue(context.Background(), task.ID, &GenerationQueueQuery{
		QualityGrade: "provisional",
		SortBy:       "quality_grade",
		SortOrder:    "asc",
	})
	if err != nil {
		t.Fatalf("GetTaskGenerationQueue() error = %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("page = %+v, want one provisional queue item", page)
	}
	if page.Items[0].QualityGrade != "provisional" {
		t.Fatalf("queue item = %+v, want provisional grade", page.Items[0])
	}
	if page.Summary == nil || page.Summary.QualityGradeCounts["provisional"] != 1 {
		t.Fatalf("summary = %+v, want provisional grade summary", page.Summary)
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
			CanonicalProduct: &productenrich.CanonicalProduct{CategoryPath: []string{"Electronics", "Audio"}},
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
			CanonicalProduct: &productenrich.CanonicalProduct{CategoryPath: []string{"Home", "Cushions"}},
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

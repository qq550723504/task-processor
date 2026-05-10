package listingkit

import (
	"context"
	"testing"
	"time"

	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
	assetrepo "task-processor/internal/asset/repository"
	common "task-processor/internal/publishing/common"
)

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

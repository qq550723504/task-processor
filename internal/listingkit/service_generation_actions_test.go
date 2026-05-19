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
	"task-processor/internal/listingkit/reviewstore"
	common "task-processor/internal/publishing/common"
)

type stubStandardProductWorkflowClient struct {
	calls []StandardProductWorkflowStartInput
	err   error
}

func (s *stubStandardProductWorkflowClient) StartStandardProduct(_ context.Context, in StandardProductWorkflowStartInput) error {
	s.calls = append(s.calls, in)
	return s.err
}

type stubPlatformAdaptWorkflowClient struct {
	calls []PlatformAdaptWorkflowStartInput
	err   error
}

func (s *stubPlatformAdaptWorkflowClient) StartPlatformAdaptation(_ context.Context, in PlatformAdaptWorkflowStartInput) error {
	s.calls = append(s.calls, in)
	return s.err
}

func TestExecuteTaskGenerationActionStartsStandardProductTemporalWorkflow(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	client := &stubStandardProductWorkflowClient{}
	svc := &service{
		repo:                           repo,
		standardProductWorkflowClient:  client,
		standardProductWorkflowEnabled: true,
	}

	task := &Task{
		ID:        "task-generation-action-standard-temporal-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"shein"}},
		Result:    &ListingKitResult{TaskID: "task-generation-action-standard-temporal-1"},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	result, err := svc.ExecuteTaskGenerationAction(context.Background(), task.ID, &ExecuteGenerationActionRequest{
		ActionKey: assetGenerationActionRunStandardProductTemporal,
	})
	if err != nil {
		t.Fatalf("ExecuteTaskGenerationAction() error = %v", err)
	}
	if len(client.calls) != 1 || client.calls[0].TaskID != task.ID {
		t.Fatalf("standard product temporal calls = %+v, want single call for task", client.calls)
	}
	if result == nil || result.ActionKey != assetGenerationActionRunStandardProductTemporal {
		t.Fatalf("result = %+v, want standard temporal action result", result)
	}
	if result.Audit == nil || result.Audit.ResolutionSource != "layer_temporal" {
		t.Fatalf("audit = %+v, want layer_temporal audit", result.Audit)
	}
}

func TestExecuteTaskGenerationActionStartsPlatformAdaptTemporalWorkflow(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	client := &stubPlatformAdaptWorkflowClient{}
	svc := &service{
		repo:                         repo,
		platformAdaptWorkflowClient:  client,
		platformAdaptWorkflowEnabled: true,
	}

	task := &Task{
		ID:        "task-generation-action-platform-temporal-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"shein"}},
		Result:    &ListingKitResult{TaskID: "task-generation-action-platform-temporal-1"},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	result, err := svc.ExecuteTaskGenerationAction(context.Background(), task.ID, &ExecuteGenerationActionRequest{
		ActionKey: assetGenerationActionRunPlatformAdaptTemporal,
		Target: &AssetGenerationActionTarget{
			QueueQuery: &GenerationQueueQuery{Platform: "amazon"},
		},
	})
	if err != nil {
		t.Fatalf("ExecuteTaskGenerationAction() error = %v", err)
	}
	if len(client.calls) != 1 || client.calls[0].TaskID != task.ID || client.calls[0].Platform != "amazon" {
		t.Fatalf("platform adapt temporal calls = %+v, want single amazon call for task", client.calls)
	}
	if result == nil || result.ActionKey != assetGenerationActionRunPlatformAdaptTemporal {
		t.Fatalf("result = %+v, want platform temporal action result", result)
	}
	if result.ResolvedTarget == nil || result.ResolvedTarget.QueueQuery == nil || result.ResolvedTarget.QueueQuery.Platform != "amazon" {
		t.Fatalf("resolved target = %+v, want amazon queue query", result.ResolvedTarget)
	}
}

func TestExecuteTaskGenerationActionRunsRetryableTarget(t *testing.T) {
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
		ID:        "task-generation-action-retry-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"amazon"}},
		Result: &ListingKitResult{
			TaskID:         "task-generation-action-retry-1",
			CatalogProduct: &catalog.Product{Title: "Portable Speaker"},
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
				PendingGeneration: []assetgeneration.Task{{
					ID:              "amazon:amazon-lifestyle",
					Platform:        "amazon",
					RecipeID:        "amazon-lifestyle",
					AssetKind:       asset.KindSceneImage,
					Slot:            "auxiliary",
					Purpose:         "scene",
					ExecutionStatus: "planned",
					ExecutionMode:   assetgeneration.ExecutionModeRendererBacked,
					CanExecute:      true,
				}},
			}},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if err := assetRepository.SaveInventory(context.Background(), &asset.Inventory{
		Ref: asset.InventoryRef{TaskID: task.ID},
		Records: []asset.AssetRecord{{
			ID:   "asset-preview-1",
			Kind: asset.KindSellingPointImage,
			URL:  "https://cdn.example.com/preview.svg",
			Metadata: map[string]string{
				"draw_preview_format":     "svg",
				"layout_draw_preview_svg": "<svg/>",
				"layout_engine":           "selling_point_output_v2",
				"visual_mode":             "selling_point",
			},
		}},
	}); err != nil {
		t.Fatalf("SaveInventory() error = %v", err)
	}
	if err := assetRepository.SaveGenerationTasks(context.Background(), task.ID, task.Result.Amazon.ImageBundle.PendingGeneration); err != nil {
		t.Fatalf("SaveGenerationTasks() error = %v", err)
	}

	result, err := svc.ExecuteTaskGenerationAction(context.Background(), task.ID, &ExecuteGenerationActionRequest{
		ActionKey: "generate_missing_assets",
		Target: &AssetGenerationActionTarget{
			ActionKey:       "generate_missing_assets",
			InteractionMode: "retryable",
			QueueQuery:      &GenerationQueueQuery{QualityGrade: "missing"},
			RetryRequest:    &RetryGenerationTasksRequest{QualityGrade: "missing"},
		},
	})
	if err != nil {
		t.Fatalf("ExecuteTaskGenerationAction() error = %v", err)
	}
	if result == nil || result.Retry == nil {
		t.Fatalf("result = %+v, want retry payload", result)
	}
	if result.InteractionMode != "retryable" {
		t.Fatalf("result = %+v, want retryable interaction mode", result)
	}
	if result.ResolvedTarget == nil || result.ResolvedTarget.RetryRequest == nil || result.ResolvedTarget.RetryRequest.QualityGrade != "missing" {
		t.Fatalf("resolved target = %+v, want missing retry target", result.ResolvedTarget)
	}
	if result.Audit == nil || result.Audit.ResolutionSource != "request_target" || result.Audit.ExecutionPath != "retryable" {
		t.Fatalf("audit = %+v, want request_target retryable audit", result.Audit)
	}
	if result.ResolvedTarget.ExpectedImpact == nil {
		t.Fatalf("resolved target = %+v, want expected impact", result.ResolvedTarget)
	}
}

func TestExecuteTaskGenerationActionRunsQueueOnlyTarget(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	assetRepository := assetrepo.NewMemRepository()
	svc := &service{
		repo:       repo,
		assetRepo:  assetRepository,
		reviewRepo: reviewstore.NewMemRepository(),
	}

	task := &Task{
		ID:        "task-generation-action-queue-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"amazon"}},
		Result: &ListingKitResult{
			TaskID: "task-generation-action-queue-1",
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

	result, err := svc.ExecuteTaskGenerationAction(context.Background(), task.ID, &ExecuteGenerationActionRequest{
		ActionKey: "review_missing_slots",
	})
	if err != nil {
		t.Fatalf("ExecuteTaskGenerationAction() error = %v", err)
	}
	if result == nil || result.Queue == nil {
		t.Fatalf("result = %+v, want queue payload", result)
	}
	if result.InteractionMode != "queue_only" {
		t.Fatalf("result = %+v, want queue_only interaction mode", result)
	}
	if result.ResolvedTarget == nil || result.ResolvedTarget.QueueQuery == nil || result.ResolvedTarget.QueueQuery.QualityGrade != "missing" {
		t.Fatalf("resolved target = %+v, want missing queue target", result.ResolvedTarget)
	}
	if result.Audit == nil || result.Audit.ExecutionPath != "queue_only" {
		t.Fatalf("audit = %+v, want queue_only audit", result.Audit)
	}
	if result.ReviewSession == nil {
		t.Fatalf("result = %+v, want review session", result)
	}
	if result.ReviewSession.DefaultTarget == nil || result.ReviewSession.DefaultTarget.FocusKey == "" || result.ReviewSession.DefaultTarget.SectionKey == "" {
		t.Fatalf("review session default target = %+v, want focus and section keys", result.ReviewSession.DefaultTarget)
	}
	if result.ReviewSession.DefaultTarget.SessionQuery == nil || result.ReviewSession.DefaultTarget.SessionQuery.ResponseMode != "patch_only" || result.ReviewSession.DefaultTarget.SessionQuery.Platform != "amazon" || result.ReviewSession.DefaultTarget.SessionQuery.Slot != "auxiliary" {
		t.Fatalf("review session default target = %+v, want session query payload", result.ReviewSession.DefaultTarget)
	}
	if result.ReviewSession.DefaultTarget.NavigationTarget == nil || result.ReviewSession.DefaultTarget.NavigationTarget.SessionQuery == nil || result.ReviewSession.DefaultTarget.NavigationTarget.SessionQuery.Platform != "amazon" || result.ReviewSession.DefaultTarget.NavigationTarget.PreviewQuery == nil || result.ReviewSession.DefaultTarget.NavigationTarget.PreviewQuery.Slot != "auxiliary" {
		t.Fatalf("review session default target = %+v, want unified navigation target", result.ReviewSession.DefaultTarget)
	}
	if result.ReviewSession.DefaultTarget.NavigationTarget.QueueQuery == nil || result.ReviewSession.DefaultTarget.NavigationTarget.QueueQuery.Platform != "amazon" || result.ReviewSession.DefaultTarget.NavigationTarget.QueueQuery.Slot != "auxiliary" {
		t.Fatalf("review session default target = %+v, want queue navigation target", result.ReviewSession.DefaultTarget)
	}
	if result.ReviewPatch == nil {
		t.Fatalf("result = %+v, want review patch", result)
	}
	if result.ReviewPatch.SelectedPlatform != "" || result.ReviewPatch.SelectedSlot != "" || result.ReviewPatch.FocusedSectionKey != "" || result.ReviewPatch.FocusCapability != "" {
		t.Fatalf("review patch = %+v, want unchanged root focus fields omitted", result.ReviewPatch)
	}
	if result.ReviewPatch.Focus != nil {
		t.Fatalf("review patch focus = %+v, want no focus subpatch when focus is unchanged", result.ReviewPatch.Focus)
	}
	if result.ReviewPatch.FocusedTarget != nil || result.ReviewPatch.FocusedRenderPreview != nil || result.ReviewPatch.FocusedToolbar != nil {
		t.Fatalf("review patch = %+v, want unchanged root focused payload omitted", result.ReviewPatch)
	}
	if result.ReviewPatch.Queue != nil {
		t.Fatalf("review patch queue = %+v, want no queue subpatch when queue state is unchanged", result.ReviewPatch.Queue)
	}
	if result.ReviewPatch.QueueSummary != nil || result.ReviewPatch.ReviewSummary != nil || result.ReviewPatch.Overview != nil {
		t.Fatalf("review patch = %+v, want unchanged root summary fields omitted", result.ReviewPatch)
	}
	if result.ReviewSession.DefaultTarget.PanelState == nil || result.ReviewSession.DefaultTarget.PanelState.SelectedPlatform != "amazon" || result.ReviewSession.DefaultTarget.PanelState.SelectedSlot != "auxiliary" {
		t.Fatalf("review session default target = %+v, want panel state", result.ReviewSession.DefaultTarget)
	}
	if result.ReviewSession.DefaultTarget.NavigationDelta == nil {
		t.Fatalf("review session default target = %+v, want navigation delta", result.ReviewSession.DefaultTarget)
	}
	if result.ReviewSession.SelectedPlatform != "amazon" || result.ReviewSession.SelectedSlot != "auxiliary" {
		t.Fatalf("review session selection = %+v, want amazon/auxiliary", result.ReviewSession)
	}
	if result.ReviewSession.Queue == nil || result.ReviewSession.Queue.Summary == nil || result.ReviewSession.Queue.Summary.MissingItems != 1 {
		t.Fatalf("review session = %+v, want missing queue summary", result.ReviewSession)
	}
	if len(result.ReviewSession.PlatformCards) != 1 || result.ReviewSession.PlatformCards[0].Platform != "amazon" {
		t.Fatalf("review session platform cards = %+v, want amazon review card", result.ReviewSession)
	}
	if result.ReviewSession.PlatformCards[0].ReviewTarget == nil || result.ReviewSession.PlatformCards[0].ReviewTarget.Platform != "amazon" || result.ReviewSession.PlatformCards[0].ReviewTarget.Slot != "auxiliary" {
		t.Fatalf("review session platform card target = %+v, want amazon/auxiliary target", result.ReviewSession.PlatformCards[0])
	}
	if result.ReviewSession.PlatformCards[0].ReviewTarget.SessionQuery == nil || result.ReviewSession.PlatformCards[0].ReviewTarget.SessionQuery.ResponseMode != "patch_only" {
		t.Fatalf("review session platform card target = %+v, want session query", result.ReviewSession.PlatformCards[0].ReviewTarget)
	}
	if len(result.ReviewSession.SlotNavigation) != 1 || result.ReviewSession.SlotNavigation[0].Slot != "auxiliary" || !result.ReviewSession.SlotNavigation[0].Selected {
		t.Fatalf("review session slot navigation = %+v, want selected auxiliary slot", result.ReviewSession)
	}
	if result.ReviewSession.SlotNavigation[0].ReviewTarget == nil || result.ReviewSession.SlotNavigation[0].ReviewTarget.Platform != "amazon" || result.ReviewSession.SlotNavigation[0].ReviewTarget.Slot != "auxiliary" {
		t.Fatalf("review session slot target = %+v, want auxiliary review target", result.ReviewSession.SlotNavigation[0])
	}
	if result.ReviewSession.SlotNavigation[0].ReviewTarget.FocusKey == "" {
		t.Fatalf("review session slot target = %+v, want focus key", result.ReviewSession.SlotNavigation[0].ReviewTarget)
	}
	if result.ReviewSession.SlotNavigation[0].ReviewTarget.SessionQuery == nil || result.ReviewSession.SlotNavigation[0].ReviewTarget.SessionQuery.ResponseMode != "patch_only" {
		t.Fatalf("review session slot target = %+v, want session query payload", result.ReviewSession.SlotNavigation[0].ReviewTarget)
	}
}

func TestExecuteTaskGenerationActionSupportsPatchOnlyResponseMode(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	assetRepository := assetrepo.NewMemRepository()
	svc := &service{
		repo:       repo,
		assetRepo:  assetRepository,
		reviewRepo: reviewstore.NewMemRepository(),
	}

	task := &Task{
		ID:        "task-generation-action-patch-only-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"shein"}},
		Result: &ListingKitResult{
			TaskID: "task-generation-action-patch-only-1",
			AssetRenderPreviews: []AssetRenderPreview{{
				AssetID:         "asset-preview-1",
				AssetRevision:   "asset-rev-1",
				PreviewRevision: "preview-rev-1",
				TaskRevision:    "task-rev-1",
				PreviewFormat:   "svg",
				PreviewSVG:      "<svg/>",
				VisualMode:      "selling_point",
				LayerTypes:      []string{"detail", "text"},
			}},
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

	result, err := svc.ExecuteTaskGenerationAction(context.Background(), task.ID, &ExecuteGenerationActionRequest{
		ActionKey:    "approve_section_review",
		ResponseMode: "patch_only",
		Target: &AssetGenerationActionTarget{
			ActionKey:       "approve_section_review",
			InteractionMode: "review_only",
			QueueQuery: &GenerationQueueQuery{
				Platform:          "shein",
				Slot:              "main",
				PreviewCapability: "detail_preview",
			},
		},
	})
	if err != nil {
		t.Fatalf("ExecuteTaskGenerationAction() error = %v", err)
	}
	if result.ResponseMode != "patch_only" {
		t.Fatalf("result = %+v, want patch_only response mode", result)
	}
	if result.ReviewSession != nil || len(result.PlatformRenderPreviews) != 0 {
		t.Fatalf("result = %+v, want patch-only response without full review session/previews", result)
	}
	if result.ReviewPatch == nil || result.ReviewPatch.DeltaToken == "" || result.DeltaToken == "" {
		t.Fatalf("result = %+v, want delta token on patch-only response", result)
	}
	if result.ReviewPatch.Focus != nil {
		t.Fatalf("review patch = %+v, want no focus subpatch when focus is unchanged", result.ReviewPatch)
	}
	if result.ReviewPatch.FocusedRenderPreview != nil || result.ReviewPatch.FocusedTarget != nil || result.ReviewPatch.FocusedToolbar != nil {
		t.Fatalf("review patch = %+v, want unchanged root focused payload omitted", result.ReviewPatch)
	}
}

func TestGetTaskGenerationReviewSessionReturnsNotModifiedWhenDeltaMatches(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	assetRepository := assetrepo.NewMemRepository()
	svc := &service{
		repo:       repo,
		assetRepo:  assetRepository,
		reviewRepo: reviewstore.NewMemRepository(),
	}

	task := &Task{
		ID:        "task-generation-review-session-delta-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"shein"}},
		Result: &ListingKitResult{
			TaskID: "task-generation-review-session-delta-1",
			AssetRenderPreviews: []AssetRenderPreview{{
				AssetID:         "asset-preview-1",
				AssetRevision:   "asset-rev-1",
				PreviewRevision: "preview-rev-1",
				TaskRevision:    "task-rev-1",
				PreviewFormat:   "svg",
				PreviewSVG:      "<svg/>",
				VisualMode:      "selling_point",
				LayerTypes:      []string{"detail", "text"},
			}},
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

	first, err := svc.GetTaskGenerationReviewSession(context.Background(), task.ID, &GenerationQueueQuery{
		Platform: "shein",
		Slot:     "main",
	})
	if err != nil {
		t.Fatalf("GetTaskGenerationReviewSession() first call error = %v", err)
	}
	if first == nil || first.Session == nil || first.DeltaToken == "" {
		t.Fatalf("first response = %+v, want session with delta token", first)
	}

	second, err := svc.GetTaskGenerationReviewSession(context.Background(), task.ID, &GenerationQueueQuery{
		Platform:   "shein",
		Slot:       "main",
		DeltaToken: first.DeltaToken,
	})
	if err != nil {
		t.Fatalf("GetTaskGenerationReviewSession() second call error = %v", err)
	}
	if second == nil || !second.NotModified || second.DeltaToken != first.DeltaToken || second.Session != nil {
		t.Fatalf("second response = %+v, want not_modified with matching delta token", second)
	}
}

func TestGetTaskGenerationReviewPreviewReturnsNotModifiedWhenDeltaMatches(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	assetRepository := assetrepo.NewMemRepository()
	svc := &service{
		repo:       repo,
		assetRepo:  assetRepository,
		reviewRepo: reviewstore.NewMemRepository(),
	}

	task := &Task{
		ID:        "task-generation-review-preview-delta-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"shein"}},
		Result: &ListingKitResult{
			TaskID: "task-generation-review-preview-delta-1",
			AssetRenderPreviews: []AssetRenderPreview{{
				AssetID:         "asset-preview-1",
				AssetRevision:   "asset-rev-1",
				PreviewRevision: "preview-rev-1",
				TaskRevision:    "task-rev-1",
				PreviewFormat:   "svg",
				PreviewSVG:      "<svg/>",
				VisualMode:      "selling_point",
				LayerTypes:      []string{"detail", "text"},
			}},
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

	first, err := svc.GetTaskGenerationReviewPreview(context.Background(), task.ID, &GenerationQueueQuery{
		Platform: "shein",
		Slot:     "main",
	})
	if err != nil {
		t.Fatalf("GetTaskGenerationReviewPreview() first call error = %v", err)
	}
	if first == nil || first.Preview == nil || first.DeltaToken == "" {
		t.Fatalf("first response = %+v, want preview with delta token", first)
	}

	second, err := svc.GetTaskGenerationReviewPreview(context.Background(), task.ID, &GenerationQueueQuery{
		Platform:   "shein",
		Slot:       "main",
		DeltaToken: first.DeltaToken,
	})
	if err != nil {
		t.Fatalf("GetTaskGenerationReviewPreview() second call error = %v", err)
	}
	if second == nil || !second.NotModified || second.DeltaToken != first.DeltaToken || second.Preview != nil || second.Toolbar != nil {
		t.Fatalf("second response = %+v, want not_modified preview response", second)
	}
}

func TestGetTaskGenerationReviewSessionSupportsPatchOnlyNavigationRead(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	assetRepository := assetrepo.NewMemRepository()
	svc := &service{
		repo:       repo,
		assetRepo:  assetRepository,
		reviewRepo: reviewstore.NewMemRepository(),
	}

	task := &Task{
		ID:        "task-generation-review-session-patch-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"shein"}},
		Result: &ListingKitResult{
			TaskID: "task-generation-review-session-patch-1",
			AssetRenderPreviews: []AssetRenderPreview{{
				AssetID:         "asset-preview-main",
				AssetRevision:   "asset-rev-main",
				PreviewRevision: "preview-rev-main",
				TaskRevision:    "task-rev-1",
				PreviewFormat:   "svg",
				PreviewSVG:      "<svg/>",
				VisualMode:      "selling_point",
				LayerTypes:      []string{"detail", "text"},
			}, {
				AssetID:         "asset-preview-gallery",
				AssetRevision:   "asset-rev-gallery",
				PreviewRevision: "preview-rev-gallery",
				TaskRevision:    "task-rev-1",
				PreviewFormat:   "svg",
				PreviewSVG:      "<svg/>",
				VisualMode:      "selling_point",
				LayerTypes:      []string{"badge", "text"},
			}},
			Shein: &SheinPackage{ImageBundle: &common.PublishImageBundle{
				Platform: "shein",
				Main: &common.BundleSlot{
					Key:           "main",
					AssetID:       "asset-preview-main",
					StateLabel:    "ready",
					TemplateLabel: "SHEIN Main",
				},
				Gallery: []common.BundleSlot{{
					Key:           "gallery",
					AssetID:       "asset-preview-gallery",
					StateLabel:    "ready",
					TemplateLabel: "SHEIN Gallery",
				}},
			}},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	response, err := svc.GetTaskGenerationReviewSession(context.Background(), task.ID, &GenerationQueueQuery{
		Platform:          "shein",
		Slot:              "gallery",
		PreviewCapability: "badge_preview",
		ResponseMode:      "patch_only",
		FromPlatform:      "shein",
		FromSlot:          "main",
		FromCapability:    "detail_preview",
	})
	if err != nil {
		t.Fatalf("GetTaskGenerationReviewSession() error = %v", err)
	}
	if response == nil || response.ResponseMode != "patch_only" || response.Session != nil {
		t.Fatalf("response = %+v, want patch_only review session response", response)
	}
	if response.Patch == nil || !response.Patch.FocusChanged {
		t.Fatalf("response patch = %+v, want focus-changing patch", response.Patch)
	}
	if response.Patch.Focus == nil || response.Patch.Focus.SelectedSlot != "gallery" || response.Patch.Focus.FocusCapability != "badge_preview" {
		t.Fatalf("response patch focus = %+v, want gallery/badge focus", response.Patch.Focus)
	}
}

func TestExecuteTaskGenerationActionBuildsRetryReviewSessionFromExecutedQueue(t *testing.T) {
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
		ID:        "task-generation-action-retry-review-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"shein"}},
		Result: &ListingKitResult{
			TaskID: "task-generation-action-retry-review-1",
			AssetRenderPreviews: []AssetRenderPreview{{
				AssetID:       "asset-preview-1",
				PreviewFormat: "svg",
				PreviewSVG:    "<svg/>",
				VisualMode:    "selling_point",
				LayerTypes:    []string{"detail", "text"},
			}},
			Shein: &SheinPackage{ImageBundle: &common.PublishImageBundle{
				Platform: "shein",
				Main: &common.BundleSlot{
					Key:           "main",
					AssetID:       "asset-preview-1",
					RecipeID:      "shein-main-model",
					StateLabel:    "fallback_in_use",
					SatisfiedBy:   "fallback_asset",
					TemplateLabel: "SHEIN Main",
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
	if err := assetRepository.SaveGenerationTasks(context.Background(), task.ID, []assetgeneration.Task{{
		TaskID:          task.ID,
		ID:              "shein:shein-main-model",
		Platform:        "shein",
		RecipeID:        "shein-main-model",
		Slot:            "main",
		AssetKind:       asset.KindModelImage,
		ExecutionStatus: "completed",
		ExecutionMode:   assetgeneration.ExecutionModeDeferredStub,
		SatisfiedBy:     "generated_asset",
		CanExecute:      true,
	}}); err != nil {
		t.Fatalf("SaveGenerationTasks() error = %v", err)
	}

	result, err := svc.ExecuteTaskGenerationAction(context.Background(), task.ID, &ExecuteGenerationActionRequest{
		ActionKey: "upgrade_fallback_assets",
	})
	if err != nil {
		t.Fatalf("ExecuteTaskGenerationAction() error = %v", err)
	}
	if result.ReviewSession == nil || result.ReviewSession.Queue == nil {
		t.Fatalf("result = %+v, want review session queue", result)
	}
	if result.ReviewSession.SelectedPlatform != "shein" || result.ReviewSession.SelectedSlot != "main" {
		t.Fatalf("review session selection = %+v, want shein/main", result.ReviewSession)
	}
	if result.ReviewSession.FocusCapability != "detail_preview" {
		t.Fatalf("review session focus capability = %+v, want detail_preview", result.ReviewSession)
	}
	if result.ReviewSession.FocusedSectionKey == "" {
		t.Fatalf("review session = %+v, want focused section key", result.ReviewSession)
	}
	if result.ReviewSession.FocusedTarget == nil || result.ReviewSession.FocusedTarget.FocusKey == "" || result.ReviewSession.FocusedTarget.ActionKey != "review_detail_previews" {
		t.Fatalf("review session focused target = %+v, want detail review focus target", result.ReviewSession.FocusedTarget)
	}
	if result.ReviewSession.FocusedTarget.SessionQuery == nil || result.ReviewSession.FocusedTarget.SessionQuery.ResponseMode != "patch_only" || result.ReviewSession.FocusedTarget.SessionQuery.FromPlatform != "shein" {
		t.Fatalf("review session focused target = %+v, want focused navigation query", result.ReviewSession.FocusedTarget)
	}
	if result.ReviewSession.FocusedTarget.PanelState == nil || result.ReviewSession.FocusedTarget.PanelState.SelectedPlatform != "shein" || result.ReviewSession.FocusedTarget.PanelState.SelectedSlot != "main" || result.ReviewSession.FocusedTarget.PanelState.FocusedSectionKey != "detail_preview" {
		t.Fatalf("review session focused target = %+v, want panel state", result.ReviewSession.FocusedTarget)
	}
	if result.ReviewSession.FocusedRenderPreview == nil || result.ReviewSession.FocusedRenderPreview.Slot != "main" || result.ReviewSession.FocusedRenderPreview.VisualMode != "selling_point" {
		t.Fatalf("review session focused render preview = %+v, want shein main selling_point preview", result.ReviewSession.FocusedRenderPreview)
	}
	if result.ReviewSession.FocusedToolbar == nil || result.ReviewSession.FocusedToolbar.Platform != "shein" || result.ReviewSession.FocusedToolbar.Slot != "main" || result.ReviewSession.FocusedToolbar.Capability != "detail_preview" {
		t.Fatalf("review session focused toolbar = %+v, want shein/main/detail toolbar", result.ReviewSession.FocusedToolbar)
	}
	if result.ReviewSession.FocusedToolbar.PreviewViewer == nil || result.ReviewSession.FocusedToolbar.PreviewViewer.AssetID != "asset-preview-1" || result.ReviewSession.FocusedToolbar.PreviewViewer.PreviewFormat != "svg" {
		t.Fatalf("review session focused toolbar = %+v, want preview viewer target", result.ReviewSession.FocusedToolbar)
	}
	if result.ReviewSession.FocusedToolbar.PreviewViewer.PreviewQuery == nil || result.ReviewSession.FocusedToolbar.PreviewViewer.PreviewQuery.AssetID != "asset-preview-1" || result.ReviewSession.FocusedToolbar.PreviewViewer.PreviewQuery.PreviewCapability != "detail_preview" {
		t.Fatalf("review session focused toolbar = %+v, want preview query contract", result.ReviewSession.FocusedToolbar)
	}
	if result.ReviewSession.FocusedToolbar.PreviewViewer.NavigationTarget == nil || result.ReviewSession.FocusedToolbar.PreviewViewer.NavigationTarget.PreviewQuery == nil || result.ReviewSession.FocusedToolbar.PreviewViewer.NavigationTarget.PreviewQuery.AssetID != "asset-preview-1" || result.ReviewSession.FocusedToolbar.PreviewViewer.NavigationTarget.SessionQuery == nil || result.ReviewSession.FocusedToolbar.PreviewViewer.NavigationTarget.SessionQuery.ResponseMode != "patch_only" {
		t.Fatalf("review session focused toolbar = %+v, want unified preview navigation target", result.ReviewSession.FocusedToolbar)
	}
	if len(result.ReviewSession.FocusedToolbar.SectionActions) < 2 || !result.ReviewSession.FocusedToolbar.SectionActions[0].Selected {
		t.Fatalf("review session focused toolbar = %+v, want selected section action set", result.ReviewSession.FocusedToolbar)
	}
	if result.ReviewSession.FocusedToolbar.SectionActions[0].Target == nil || result.ReviewSession.FocusedToolbar.SectionActions[0].Target.PanelState == nil {
		t.Fatalf("review session focused toolbar action = %+v, want target panel state", result.ReviewSession.FocusedToolbar.SectionActions[0])
	}
	if len(result.ReviewSession.FocusedToolbar.PreviewActions) < 3 {
		t.Fatalf("review session focused toolbar = %+v, want preview workflow actions", result.ReviewSession.FocusedToolbar)
	}
	if result.ReviewSession.FocusedToolbar.PreviewActions[0].ViewerTarget == nil || result.ReviewSession.FocusedToolbar.PreviewActions[0].Key != "open_preview_svg" {
		t.Fatalf("review session focused toolbar preview action = %+v, want viewer action", result.ReviewSession.FocusedToolbar.PreviewActions[0])
	}
	if result.ReviewSession.FocusedToolbar.PreviewActions[0].PreviewQuery == nil || result.ReviewSession.FocusedToolbar.PreviewActions[0].PreviewQuery.AssetID != "asset-preview-1" {
		t.Fatalf("review session focused toolbar preview action = %+v, want preview query", result.ReviewSession.FocusedToolbar.PreviewActions[0])
	}
	if result.ReviewSession.FocusedToolbar.PreviewActions[0].NavigationTarget == nil || result.ReviewSession.FocusedToolbar.PreviewActions[0].NavigationTarget.PreviewQuery == nil || result.ReviewSession.FocusedToolbar.PreviewActions[0].NavigationTarget.PreviewQuery.AssetID != "asset-preview-1" {
		t.Fatalf("review session focused toolbar preview action = %+v, want unified navigation target", result.ReviewSession.FocusedToolbar.PreviewActions[0])
	}
	if result.ReviewSession.FocusedToolbar.PreviewActions[1].NavigationTarget == nil || result.ReviewSession.FocusedToolbar.PreviewActions[1].NavigationTarget.ActionTarget == nil || result.ReviewSession.FocusedToolbar.PreviewActions[1].NavigationTarget.ActionTarget.ActionKey != "retry_section_generation" {
		t.Fatalf("review session focused toolbar preview action = %+v, want workflow navigation target", result.ReviewSession.FocusedToolbar.PreviewActions[1])
	}
	if result.ReviewSession.FocusedToolbar.PreviewActions[1].ActionTarget == nil || result.ReviewSession.FocusedToolbar.PreviewActions[1].ActionTarget.ActionKey != "retry_section_generation" {
		t.Fatalf("review session focused toolbar preview action = %+v, want retry section action", result.ReviewSession.FocusedToolbar.PreviewActions[1])
	}
	if len(result.ReviewSession.PlatformRenderPreviews) != 1 || result.ReviewSession.PlatformRenderPreviews[0].Platform != "shein" {
		t.Fatalf("review session platform render previews = %+v, want shein previews", result.ReviewSession)
	}
	if len(result.ReviewSession.PlatformCards) != 1 || result.ReviewSession.PlatformCards[0].PreviewSummary == nil {
		t.Fatalf("review session platform cards = %+v, want preview summary", result.ReviewSession)
	}
	if result.ReviewSession.PlatformCards[0].ReviewTarget == nil || result.ReviewSession.PlatformCards[0].ReviewTarget.Platform != "shein" || result.ReviewSession.PlatformCards[0].ReviewTarget.Capability != "detail_preview" {
		t.Fatalf("review session platform card target = %+v, want shein detail target", result.ReviewSession.PlatformCards[0])
	}
	if len(result.ReviewSession.Sections) == 0 || result.ReviewSession.Sections[0].Capability != "detail_preview" || !result.ReviewSession.Sections[0].Selected {
		t.Fatalf("review session sections = %+v, want selected detail section", result.ReviewSession)
	}
	if result.ReviewSession.Sections[0].SectionKey == "" || result.ReviewSession.Sections[0].Title == "" || result.ReviewSession.Sections[0].Description == "" || result.ReviewSession.Sections[0].ReviewTarget == nil {
		t.Fatalf("review session section = %+v, want section metadata and target", result.ReviewSession.Sections[0])
	}
	if result.ReviewSession.Sections[0].ReviewTarget.SessionQuery == nil || result.ReviewSession.Sections[0].ReviewTarget.SessionQuery.ResponseMode != "patch_only" {
		t.Fatalf("review session section = %+v, want section navigation query", result.ReviewSession.Sections[0].ReviewTarget)
	}
	if result.ReviewSession.Sections[0].ReviewTarget.NavigationTarget == nil || result.ReviewSession.Sections[0].ReviewTarget.NavigationTarget.QueueQuery == nil || result.ReviewSession.Sections[0].ReviewTarget.NavigationTarget.QueueQuery.Platform != "shein" {
		t.Fatalf("review session section = %+v, want section queue navigation target", result.ReviewSession.Sections[0].ReviewTarget)
	}
	if len(result.ReviewSession.Sections[0].ToolbarActions) == 0 {
		t.Fatalf("review session section = %+v, want toolbar actions", result.ReviewSession.Sections[0])
	}
	if result.ReviewSession.Sections[0].ToolbarActions[0].Key != "review_detail_previews" || result.ReviewSession.Sections[0].ToolbarActions[1].Key != "open_preview_svg" {
		t.Fatalf("review session section toolbar = %+v, want capability and viewer actions", result.ReviewSession.Sections[0].ToolbarActions)
	}
	if len(result.ReviewSession.Sections[0].WorkflowActions) < 2 {
		t.Fatalf("review session section = %+v, want workflow actions", result.ReviewSession.Sections[0])
	}
	if result.ReviewSession.Sections[0].WorkflowActions[0].ActionTarget == nil || result.ReviewSession.Sections[0].WorkflowActions[0].ActionTarget.ActionKey != "retry_section_generation" {
		t.Fatalf("review session section workflow = %+v, want retry section action", result.ReviewSession.Sections[0].WorkflowActions)
	}
	if result.ReviewSession.Sections[0].WorkflowActions[0].NavigationTarget == nil || result.ReviewSession.Sections[0].WorkflowActions[0].NavigationTarget.ActionTarget == nil || result.ReviewSession.Sections[0].WorkflowActions[0].NavigationTarget.ActionTarget.ActionKey != "retry_section_generation" {
		t.Fatalf("review session section workflow = %+v, want unified workflow navigation target", result.ReviewSession.Sections[0].WorkflowActions)
	}
	if result.ReviewWorkflow == nil || result.ReviewWorkflow.ActionKey != "upgrade_fallback_assets" {
		t.Fatalf("review workflow = %+v, want workflow result", result.ReviewWorkflow)
	}
	if result.ReviewSession.Sections[0].PrimaryActionKey != "review_detail_previews" || result.ReviewSession.Sections[0].PrimaryActionTarget == nil || result.ReviewSession.Sections[0].PrimaryActionTarget.Capability != "detail_preview" {
		t.Fatalf("review session section target = %+v, want detail review action target", result.ReviewSession.Sections[0])
	}
	if len(result.ReviewSession.SlotNavigation) == 0 || !result.ReviewSession.SlotNavigation[0].Selected || result.ReviewSession.SlotNavigation[0].PreviewCapabilities[0] != "detail_preview" {
		t.Fatalf("review session slot navigation = %+v, want preview-aware slot navigation", result.ReviewSession)
	}
	if len(result.ReviewSession.SlotNavigation[0].FocusRegions) == 0 || len(result.ReviewSession.SlotNavigation[0].FocusLayerTypes) == 0 || result.ReviewSession.SlotNavigation[0].FocusCapability != "detail_preview" {
		t.Fatalf("review session slot focus = %+v, want populated focus hints", result.ReviewSession.SlotNavigation[0])
	}
	if result.ReviewSession.SlotNavigation[0].ReviewTarget == nil || result.ReviewSession.SlotNavigation[0].ReviewTarget.Capability != "detail_preview" || result.ReviewSession.SlotNavigation[0].ReviewTarget.Slot != "main" {
		t.Fatalf("review session slot target = %+v, want detail slot target", result.ReviewSession.SlotNavigation[0])
	}
}

func TestExecuteTaskGenerationActionAppliesSectionReviewOutcome(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	assetRepository := assetrepo.NewMemRepository()
	svc := &service{
		repo:       repo,
		assetRepo:  assetRepository,
		reviewRepo: reviewstore.NewMemRepository(),
	}

	task := &Task{
		ID:        "task-generation-action-section-review-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"shein"}},
		Result: &ListingKitResult{
			TaskID: "task-generation-action-section-review-1",
			AssetRenderPreviews: []AssetRenderPreview{{
				AssetID:         "asset-preview-1",
				AssetRevision:   "asset-rev-1",
				PreviewRevision: "preview-rev-1",
				TaskRevision:    "task-rev-1",
				PreviewFormat:   "svg",
				PreviewSVG:      "<svg/>",
				VisualMode:      "selling_point",
				LayerTypes:      []string{"detail", "text"},
			}},
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

	result, err := svc.ExecuteTaskGenerationAction(context.Background(), task.ID, &ExecuteGenerationActionRequest{
		ActionKey: "approve_section_review",
		Target: &AssetGenerationActionTarget{
			ActionKey:       "approve_section_review",
			InteractionMode: "review_only",
			QueueQuery: &GenerationQueueQuery{
				Platform:          "shein",
				Slot:              "main",
				PreviewCapability: "detail_preview",
			},
		},
	})
	if err != nil {
		t.Fatalf("ExecuteTaskGenerationAction() error = %v", err)
	}
	if result.ReviewWorkflow == nil || result.ReviewWorkflow.ActionKey != "approve_section_review" || result.ReviewWorkflow.Status != "applied" {
		t.Fatalf("review workflow = %+v, want applied approve workflow", result.ReviewWorkflow)
	}
	if result.ReviewPatch == nil {
		t.Fatalf("result = %+v, want review patch", result)
	}
	if result.ReviewPatch.LastWorkflowResult == nil || result.ReviewPatch.LastWorkflowResult.ActionKey != "approve_section_review" {
		t.Fatalf("review patch = %+v, want workflow result attached", result.ReviewPatch)
	}
	if result.ReviewPatch.Focus != nil {
		t.Fatalf("review patch = %+v, want no focus subpatch when focus is unchanged", result.ReviewPatch)
	}
	if result.ReviewPatch.FocusedTarget != nil || result.ReviewPatch.FocusedRenderPreview != nil || result.ReviewPatch.FocusedToolbar != nil {
		t.Fatalf("review patch = %+v, want unchanged root focused payload omitted", result.ReviewPatch)
	}
	if result.ReviewPatch.Queue == nil {
		t.Fatalf("review patch = %+v, want structured queue patch", result.ReviewPatch)
	}
	if result.ReviewSession == nil || result.ReviewSession.LastWorkflowResult == nil {
		t.Fatalf("review session = %+v, want last workflow result", result.ReviewSession)
	}
	records, err := svc.listGenerationReviews(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("listGenerationReviews() error = %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("records = %+v, want one persisted review record", records)
	}
	if result.ReviewSession.LastWorkflowResult.ActionKey != "approve_section_review" {
		t.Fatalf("review session workflow = %+v, want approve workflow", result.ReviewSession.LastWorkflowResult)
	}
	if len(result.ReviewSession.Sections) == 0 || result.ReviewSession.Sections[0].WorkflowState != "approved" {
		t.Fatalf("review session sections = %+v, want approved workflow state", result.ReviewSession.Sections)
	}
	if result.ReviewSession.Sections[0].ReviewDecision != "approve" || result.ReviewSession.Sections[0].ReviewStatus != "approved" {
		t.Fatalf("review session section = %+v, want persisted approve state", result.ReviewSession.Sections[0])
	}
	if len(result.ReviewPatch.ChangedSections) == 0 || result.ReviewPatch.ChangedSections[0].ReviewDecision != "approve" || result.ReviewPatch.ChangedSections[0].ReviewStatus != "approved" {
		t.Fatalf("review patch changed sections = %+v, want approved section diff", result.ReviewPatch.ChangedSections)
	}
	if result.ReviewPatch.FocusCapability != "" || result.ReviewPatch.FocusedSectionKey != "" || result.ReviewPatch.SelectedPlatform != "" || result.ReviewPatch.SelectedSlot != "" {
		t.Fatalf("review patch = %+v, want unchanged root focus fields omitted", result.ReviewPatch)
	}
	if result.ReviewPatch.ReviewSummary == nil || result.ReviewPatch.ReviewSummary.ApprovedSections != 1 {
		t.Fatalf("review patch summary = %+v, want changed review summary", result.ReviewPatch.ReviewSummary)
	}
	if result.ReviewPatch.QueueSummary == nil || result.ReviewPatch.QueueSummary.ApprovedSections != 1 {
		t.Fatalf("review patch queue summary = %+v, want changed queue summary", result.ReviewPatch.QueueSummary)
	}
	if result.ReviewPatch.Queue.Summary == nil || result.ReviewPatch.Queue.Summary.ApprovedSections != 1 {
		t.Fatalf("review patch queue = %+v, want structured approved queue summary", result.ReviewPatch.Queue)
	}
	if len(result.ReviewPatch.Queue.ChangedSections) == 0 || result.ReviewPatch.Queue.ChangedSections[0].ReviewDecision != "approve" {
		t.Fatalf("review patch queue sections = %+v, want changed section in structured queue patch", result.ReviewPatch.Queue.ChangedSections)
	}
	if result.ReviewPatch.PlatformCards != nil {
		t.Fatalf("review patch platform cards = %+v, want no changed platform cards for section-only approval", result.ReviewPatch.PlatformCards)
	}
	if result.ReviewSession.ReviewSummary == nil || result.ReviewSession.ReviewSummary.ApprovedSections != 1 {
		t.Fatalf("review summary = %+v, want approved section count", result.ReviewSession.ReviewSummary)
	}
	queuePage, err := svc.GetTaskGenerationQueue(context.Background(), task.ID, &GenerationQueueQuery{Platform: "shein"})
	if err != nil {
		t.Fatalf("GetTaskGenerationQueue() error = %v", err)
	}
	if queuePage.Summary == nil || queuePage.Summary.ApprovedSections != 1 {
		t.Fatalf("queue summary = %+v, want approved section count", queuePage.Summary)
	}
	if len(queuePage.Items) == 0 || queuePage.Items[0].ReviewDecision != "approve" || queuePage.Items[0].ReviewStatus != "pending" {
		t.Fatalf("queue items = %+v, want slot-level pending state with approved review decision", queuePage.Items)
	}
}

func TestExecuteTaskGenerationActionRejectsUnknownActionKey(t *testing.T) {
	t.Parallel()

	_, _, err := resolveAssetGenerationActionTarget(nil, &ExecuteGenerationActionRequest{
		ActionKey: "delete_everything",
	})
	if err == nil {
		t.Fatal("expected error for unknown action key")
	}
}

func TestGetTaskGenerationReviewPreviewReportsRevisionMismatch(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	svc := &service{repo: repo}
	task := &Task{
		ID:        "task-generation-preview-mismatch-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"shein"}},
		Result: &ListingKitResult{
			TaskID: "task-generation-preview-mismatch-1",
			AssetRenderPreviews: []AssetRenderPreview{{
				AssetID:         "asset-preview-1",
				AssetRevision:   "asset-rev-1",
				PreviewRevision: "preview-rev-1",
				TaskRevision:    "task-rev-1",
				PreviewFormat:   "svg",
				PreviewSVG:      "<svg/>",
				VisualMode:      "selling_point",
				LayerTypes:      []string{"detail", "text"},
			}},
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

	response, err := svc.GetTaskGenerationReviewPreview(context.Background(), task.ID, &GenerationQueueQuery{
		Platform:        "shein",
		Slot:            "main",
		AssetID:         "asset-preview-1",
		AssetRevision:   "asset-rev-1",
		PreviewRevision: "preview-rev-other",
		TaskRevision:    "task-rev-1",
	})
	if err != nil {
		t.Fatalf("GetTaskGenerationReviewPreview() error = %v", err)
	}
	if response.RevisionStatus != "mismatch" {
		t.Fatalf("response = %+v, want mismatch revision status", response)
	}
	if response.RevisionMismatchReason == "" {
		t.Fatalf("response = %+v, want mismatch reason", response)
	}
}

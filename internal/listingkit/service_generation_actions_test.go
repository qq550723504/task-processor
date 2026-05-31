package listingkit

import (
	"context"
	"errors"
	"go/ast"
	"reflect"
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

func newTaskGenerationActionProjectionResult(taskID, assetRevision, previewRevision, taskRevision string) *ListingKitResult {
	return &ListingKitResult{
		TaskID: taskID,
		AssetRenderPreviews: []AssetRenderPreview{{
			AssetID:         "asset-preview-1",
			AssetRevision:   assetRevision,
			PreviewRevision: previewRevision,
			TaskRevision:    taskRevision,
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
	}
}

func newTaskGenerationActionProjectionQueue(taskID string, summary *GenerationWorkQueueSummary, state string) *GenerationWorkQueue {
	return &GenerationWorkQueue{
		Summary: summary,
		Items: []GenerationWorkQueueItem{{
			TaskID:                  taskID,
			Platform:                "shein",
			Slot:                    "main",
			Purpose:                 "main",
			State:                   state,
			AssetID:                 "asset-preview-1",
			ExecutionMode:           assetgeneration.ExecutionModeRendererBacked,
			RenderPreviewAvailable:  true,
			RenderPreviewFormat:     "svg",
			RenderPreviewVisualMode: "selling_point",
			RenderPreviewLayerTypes: []string{"detail", "text"},
			PreviewCapabilities:     []string{"detail_preview"},
			ReviewStatus:            "pending",
		}},
	}
}

func newTaskGenerationActionProjectionTarget(interactionMode string) *AssetGenerationActionTarget {
	return &AssetGenerationActionTarget{
		ActionKey:       "approve_section_review",
		InteractionMode: interactionMode,
		QueueQuery: &GenerationQueueQuery{
			Platform:          "shein",
			Slot:              "main",
			PreviewCapability: "detail_preview",
		},
	}
}

func newTaskGenerationActionEntryReviewFixture(t *testing.T, taskID string) (*Task, *taskGenerationService) {
	t.Helper()

	repo := &stubGenerationRepo{}
	task := &Task{
		ID:        taskID,
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"shein"}},
		Result: &ListingKitResult{
			TaskID: taskID,
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

	return task, newTaskGenerationService(taskGenerationServiceConfig{
		repo: repo,
		listAssetGenerationTasks: func(context.Context, string) ([]assetgeneration.Task, error) {
			return nil, nil
		},
		listGenerationReviews: func(context.Context, string) ([]GenerationReviewRecord, error) {
			return nil, nil
		},
	})
}

func TestTaskGenerationActionProjectionBuildsReviewSessionAndPatch(t *testing.T) {
	t.Parallel()

	t.Run("queue_only", func(t *testing.T) {
		t.Parallel()

		target := newTaskGenerationActionProjectionTarget("queue_only")
		previousQueue := newTaskGenerationActionProjectionQueue("task-generation-action-projection-queue-1", &GenerationWorkQueueSummary{
			TotalItems:            1,
			ReadyItems:            1,
			PreviewableItems:      1,
			ReviewPendingSections: 1,
		}, "ready")
		previousSession := buildGenerationReviewSession(
			newTaskGenerationActionProjectionResult("task-generation-action-projection-queue-1", "asset-rev-old", "preview-rev-old", "task-rev-old"),
			previousQueue,
			target.QueueQuery,
		)
		currentResult := newTaskGenerationActionProjectionResult("task-generation-action-projection-queue-1", "asset-rev-new", "preview-rev-new", "task-rev-new")
		currentQueue := newTaskGenerationActionProjectionQueue("task-generation-action-projection-queue-1", &GenerationWorkQueueSummary{
			TotalItems:       1,
			CompletedItems:   1,
			PreviewableItems: 1,
			ApprovedSections: 1,
		}, "completed")

		result := buildTaskGenerationActionProjectionPhase().run(&taskGenerationActionProjectionInput{
			actionKey:             target.ActionKey,
			target:                target,
			responseMode:          "full",
			previousReviewSession: previousSession,
			currentResult:         currentResult,
			refresh: &taskGenerationActionRefreshResult{
				overview:               &AssetGenerationOverview{},
				platformRenderPreviews: []PlatformAssetRenderPreviews{{Platform: "shein", Main: &AssetRenderPreviewSlot{AssetID: "asset-preview-1"}}},
				currentResult:          currentResult,
			},
			execution: &taskGenerationActionExecution{
				queuePage: &GenerationQueuePage{
					Summary: currentQueue.Summary,
					Items:   currentQueue.Items,
				},
			},
		})

		if result == nil {
			t.Fatal("projection result = nil, want assembled action result")
		}
		if result.ReviewSession == nil {
			t.Fatalf("projection result = %+v, want review session", result)
		}
		if result.ReviewSession.FocusedRenderPreview == nil || result.ReviewSession.FocusedRenderPreview.AssetRevision != "asset-rev-new" || result.ReviewSession.FocusedRenderPreview.PreviewRevision != "preview-rev-new" || result.ReviewSession.FocusedRenderPreview.TaskRevision != "task-rev-new" {
			t.Fatalf("review session focused preview = %+v, want refreshed current result revisions", result.ReviewSession.FocusedRenderPreview)
		}
		if result.ReviewSession.Queue == nil || result.ReviewSession.Queue.Summary == nil || result.ReviewSession.Queue.Summary.CompletedItems != 1 || result.ReviewSession.Queue.Summary.ReadyItems != 0 {
			t.Fatalf("review session queue = %+v, want queue page-backed queue summary", result.ReviewSession.Queue)
		}
		if result.ReviewWorkflow == nil || result.ReviewWorkflow.ActionKey != target.ActionKey || result.ReviewWorkflow.Platform != "shein" || result.ReviewWorkflow.Slot != "main" || result.ReviewWorkflow.Capability != "detail_preview" {
			t.Fatalf("review workflow = %+v, want resolved target workflow metadata", result.ReviewWorkflow)
		}
		if result.ReviewSession.LastWorkflowResult == nil || result.ReviewSession.LastWorkflowResult.ActionKey != target.ActionKey {
			t.Fatalf("review session workflow = %+v, want workflow applied to session", result.ReviewSession.LastWorkflowResult)
		}
		if result.ReviewPatch == nil || result.ReviewPatch.LastWorkflowResult == nil || result.ReviewPatch.LastWorkflowResult.ActionKey != target.ActionKey {
			t.Fatalf("review patch = %+v, want workflow attached to patch", result.ReviewPatch)
		}
		if !result.ReviewPatch.FocusChanged || result.ReviewPatch.FocusedRenderPreview == nil || result.ReviewPatch.FocusedRenderPreview.AssetRevision != "asset-rev-new" {
			t.Fatalf("review patch = %+v, want refreshed focus patch payload", result.ReviewPatch)
		}
		if result.DeltaToken == "" || result.DeltaToken != result.ReviewPatch.DeltaToken {
			t.Fatalf("delta token = %q, review patch = %+v, want patch delta token fallback", result.DeltaToken, result.ReviewPatch)
		}
		if len(result.PlatformRenderPreviews) != 1 || result.PlatformRenderPreviews[0].Platform != "shein" {
			t.Fatalf("platform render previews = %+v, want refresh previews preserved", result.PlatformRenderPreviews)
		}
	})

	t.Run("retryable", func(t *testing.T) {
		t.Parallel()

		target := newTaskGenerationActionProjectionTarget("retryable")
		previousQueue := newTaskGenerationActionProjectionQueue("task-generation-action-projection-retry-1", &GenerationWorkQueueSummary{
			TotalItems:            1,
			ReadyItems:            1,
			PreviewableItems:      1,
			ReviewPendingSections: 1,
		}, "ready")
		previousSession := buildGenerationReviewSession(
			newTaskGenerationActionProjectionResult("task-generation-action-projection-retry-1", "asset-rev-old", "preview-rev-old", "task-rev-old"),
			previousQueue,
			target.QueueQuery,
		)
		currentResult := newTaskGenerationActionProjectionResult("task-generation-action-projection-retry-1", "asset-rev-new", "preview-rev-new", "task-rev-new")

		result := buildTaskGenerationActionProjectionPhase().run(&taskGenerationActionProjectionInput{
			actionKey:             target.ActionKey,
			target:                target,
			responseMode:          "full",
			previousReviewSession: previousSession,
			currentResult:         currentResult,
			refresh: &taskGenerationActionRefreshResult{
				currentResult: currentResult,
			},
			execution: &taskGenerationActionExecution{
				retryPage: &GenerationTaskPage{
					MatchedQueue: newTaskGenerationActionProjectionQueue("task-generation-action-projection-retry-1", &GenerationWorkQueueSummary{
						TotalItems:       1,
						ReadyItems:       1,
						PreviewableItems: 1,
					}, "ready"),
					ExecutedQueue: newTaskGenerationActionProjectionQueue("task-generation-action-projection-retry-1", &GenerationWorkQueueSummary{
						TotalItems:       1,
						CompletedItems:   1,
						PreviewableItems: 1,
						ApprovedSections: 1,
					}, "completed"),
				},
			},
		})

		if result == nil || result.ReviewSession == nil {
			t.Fatalf("projection result = %+v, want retry review session", result)
		}
		if result.ReviewSession.Queue == nil || result.ReviewSession.Queue.Summary == nil || result.ReviewSession.Queue.Summary.CompletedItems != 1 || result.ReviewSession.Queue.Summary.ReadyItems != 0 {
			t.Fatalf("review session queue = %+v, want retry executed queue source", result.ReviewSession.Queue)
		}
	})
}

func TestTaskGenerationActionProjectionSupportsPatchOnlyResponses(t *testing.T) {
	t.Parallel()

	target := newTaskGenerationActionProjectionTarget("review_only")
	previousSession := buildGenerationReviewSession(
		newTaskGenerationActionProjectionResult("task-generation-action-projection-patch-1", "asset-rev-old", "preview-rev-old", "task-rev-old"),
		newTaskGenerationActionProjectionQueue("task-generation-action-projection-patch-1", &GenerationWorkQueueSummary{
			TotalItems:            1,
			ReadyItems:            1,
			PreviewableItems:      1,
			ReviewPendingSections: 1,
		}, "ready"),
		target.QueueQuery,
	)
	currentResult := newTaskGenerationActionProjectionResult("task-generation-action-projection-patch-1", "asset-rev-new", "preview-rev-new", "task-rev-new")

	result := buildTaskGenerationActionProjectionPhase().run(&taskGenerationActionProjectionInput{
		actionKey:             target.ActionKey,
		target:                target,
		responseMode:          "patch_only",
		previousReviewSession: previousSession,
		currentResult:         currentResult,
		refresh: &taskGenerationActionRefreshResult{
			platformRenderPreviews: []PlatformAssetRenderPreviews{{Platform: "shein", Main: &AssetRenderPreviewSlot{AssetID: "asset-preview-1"}}},
			currentResult:          currentResult,
		},
		execution: &taskGenerationActionExecution{
			queuePage: &GenerationQueuePage{
				Summary: newTaskGenerationActionProjectionQueue("task-generation-action-projection-patch-1", &GenerationWorkQueueSummary{
					TotalItems:       1,
					CompletedItems:   1,
					PreviewableItems: 1,
					ApprovedSections: 1,
				}, "completed").Summary,
				Items: newTaskGenerationActionProjectionQueue("task-generation-action-projection-patch-1", &GenerationWorkQueueSummary{
					TotalItems:       1,
					CompletedItems:   1,
					PreviewableItems: 1,
					ApprovedSections: 1,
				}, "completed").Items,
			},
		},
	})

	if result == nil {
		t.Fatal("projection result = nil, want patch-only action result")
	}
	if result.ResponseMode != "patch_only" {
		t.Fatalf("response mode = %q, want patch_only", result.ResponseMode)
	}
	if result.ReviewSession != nil {
		t.Fatalf("review session = %+v, want patch-only response to omit session", result.ReviewSession)
	}
	if len(result.PlatformRenderPreviews) != 0 {
		t.Fatalf("platform render previews = %+v, want patch-only response to omit previews", result.PlatformRenderPreviews)
	}
	if result.ReviewPatch == nil || result.ReviewPatch.LastWorkflowResult == nil || result.ReviewPatch.LastWorkflowResult.ActionKey != target.ActionKey {
		t.Fatalf("review patch = %+v, want workflow-attached patch payload", result.ReviewPatch)
	}
	if result.DeltaToken == "" || result.DeltaToken != result.ReviewPatch.DeltaToken {
		t.Fatalf("delta token = %q, review patch = %+v, want patch delta token preserved", result.DeltaToken, result.ReviewPatch)
	}
}

func TestTaskGenerationActionProjectionServiceDelegatesActionProjection(t *testing.T) {
	t.Parallel()

	actionSource := readExecuteTaskGenerationActionSource(t)

	assertSourceOccurrenceCount(t, actionSource, "buildTaskGenerationActionProjectionPhase()", 1)
	assertSourceContainsAll(t, actionSource, []string{
		"taskGenerationActionProjectionInput",
	})
	assertSourceOccurrenceCount(t, actionSource, "buildGenerationReviewSession(", 1)
	assertSourceExcludesAll(t, actionSource, []string{
		"buildGenerationReviewWorkflowResult(",
		"applyGenerationReviewWorkflow(",
		"buildGenerationReviewSessionPatch(",
		`"patch_only"`,
		"buildGenerationReviewDeltaToken(",
	})
}

func TestTaskGenerationActionEntryServiceDelegatesBootstrapPhase(t *testing.T) {
	t.Parallel()

	actionSource := readExecuteTaskGenerationActionSource(t)

	assertSourceOccurrenceCount(t, actionSource, "buildTaskGenerationActionEntryPhase(s).run(", 1)
	assertSourceContainsAll(t, actionSource, []string{
		"entry, err := buildTaskGenerationActionEntryPhase(s).run(ctx, taskID, req)",
		"entry.baseResult",
		"entry.target",
		"entry.previousReviewSession",
		"entry.result",
	})
	assertSourceExcludesAll(t, actionSource, []string{
		"queue, err := s.getCurrentAssetGenerationQueue(ctx, taskID)",
		"baseResult, err := s.getCurrentListingKitResult(ctx, taskID)",
		"overview := buildAssetGenerationOverview(queue)",
		"target, source, err := resolveAssetGenerationActionTarget(overview, req)",
		"target.ExpectedImpact = buildAssetGenerationActionImpact(queue, target.QueueQuery)",
		"result := &GenerationActionExecutionResult{",
	})
}

func TestTaskGenerationActionEntryPhaseBuildsRequestTargetBootstrapState(t *testing.T) {
	t.Parallel()

	task, generation := newTaskGenerationActionEntryReviewFixture(t, "task-generation-action-entry-request-target-1")

	entry, err := buildTaskGenerationActionEntryPhase(generation).run(context.Background(), task.ID, &ExecuteGenerationActionRequest{
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
		t.Fatalf("taskGenerationActionEntryPhase.run() error = %v", err)
	}
	if entry == nil || entry.result == nil || entry.target == nil {
		t.Fatalf("entry = %+v, want bootstrap state", entry)
	}
	if entry.result.ResponseMode != "patch_only" {
		t.Fatalf("response mode = %q, want patch_only", entry.result.ResponseMode)
	}
	if entry.result.ResolvedTarget != entry.target {
		t.Fatalf("resolved target = %+v, want entry target pointer %+v", entry.result.ResolvedTarget, entry.target)
	}
	wantImpact := buildAssetGenerationActionImpact(entry.baseResult.AssetGenerationQueue, entry.target.QueueQuery)
	if !reflect.DeepEqual(entry.target.ExpectedImpact, wantImpact) {
		t.Fatalf("expected impact = %+v, want %+v", entry.target.ExpectedImpact, wantImpact)
	}
	if entry.result.Audit == nil {
		t.Fatalf("audit = %+v, want audit payload", entry.result.Audit)
	}
	if entry.result.Audit.RequestedActionKey != "approve_section_review" || entry.result.Audit.ResolvedActionKey != "approve_section_review" || entry.result.Audit.ResolutionSource != "request_target" || entry.result.Audit.ExecutionPath != "review_only" {
		t.Fatalf("audit = %+v, want request_target review_only audit", entry.result.Audit)
	}
	if entry.result.Audit.ExecutedAt.IsZero() {
		t.Fatalf("audit = %+v, want executed timestamp", entry.result.Audit)
	}
	wantSession := buildGenerationReviewSession(entry.baseResult, entry.baseResult.AssetGenerationQueue, entry.target.QueueQuery)
	if !reflect.DeepEqual(entry.previousReviewSession, wantSession) {
		t.Fatalf("previousReviewSession = %+v, want %+v", entry.previousReviewSession, wantSession)
	}
}

func TestTaskGenerationActionEntryPhaseBuildsOverviewResolvedBootstrapState(t *testing.T) {
	t.Parallel()

	task, generation := newTaskGenerationActionQueueFixture(t, "task-generation-action-entry-overview-1")

	entry, err := buildTaskGenerationActionEntryPhase(generation).run(context.Background(), task.ID, &ExecuteGenerationActionRequest{
		ActionKey: "review_missing_slots",
	})
	if err != nil {
		t.Fatalf("taskGenerationActionEntryPhase.run() error = %v", err)
	}
	if entry == nil || entry.result == nil || entry.target == nil {
		t.Fatalf("entry = %+v, want bootstrap state", entry)
	}
	if entry.result.ResponseMode != "full" {
		t.Fatalf("response mode = %q, want full", entry.result.ResponseMode)
	}
	if entry.target.QueueQuery == nil || entry.target.QueueQuery.QualityGrade != "missing" {
		t.Fatalf("target = %+v, want overview-resolved missing queue query", entry.target)
	}
	if entry.result.Audit == nil {
		t.Fatalf("audit = %+v, want audit payload", entry.result.Audit)
	}
	if entry.result.Audit.RequestedActionKey != "review_missing_slots" || entry.result.Audit.ResolvedActionKey != "review_missing_slots" || entry.result.Audit.ResolutionSource != "overview" || entry.result.Audit.ExecutionPath != "queue_only" {
		t.Fatalf("audit = %+v, want overview queue_only audit", entry.result.Audit)
	}
	wantSession := buildGenerationReviewSession(entry.baseResult, entry.baseResult.AssetGenerationQueue, entry.target.QueueQuery)
	if !reflect.DeepEqual(entry.previousReviewSession, wantSession) {
		t.Fatalf("previousReviewSession = %+v, want %+v", entry.previousReviewSession, wantSession)
	}
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
	if result.InteractionMode != "queue_only" {
		t.Fatalf("result = %+v, want queue_only interaction mode", result)
	}
	if result.ResponseMode != "full" {
		t.Fatalf("response mode = %q, want normalized full mode", result.ResponseMode)
	}
	if result.ResolvedTarget == nil || result.ResolvedTarget.ActionKey != assetGenerationActionRunStandardProductTemporal || result.ResolvedTarget.InteractionMode != "queue_only" {
		t.Fatalf("resolved target = %+v, want queue_only standard temporal target", result.ResolvedTarget)
	}
	if result.ResolvedTarget.QueueQuery != nil {
		t.Fatalf("resolved target = %+v, want standard temporal target without queue query", result.ResolvedTarget)
	}
	if result.Audit == nil || result.Audit.RequestedActionKey != assetGenerationActionRunStandardProductTemporal || result.Audit.ResolvedActionKey != assetGenerationActionRunStandardProductTemporal || result.Audit.ResolutionSource != "layer_temporal" || result.Audit.ExecutionPath != "queue_only" {
		t.Fatalf("audit = %+v, want queue_only layer_temporal audit", result.Audit)
	}
	if result.Audit.ExecutedAt.IsZero() {
		t.Fatalf("audit = %+v, want executed timestamp", result.Audit)
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
	if result.InteractionMode != "queue_only" {
		t.Fatalf("result = %+v, want queue_only interaction mode", result)
	}
	if result.ResponseMode != "full" {
		t.Fatalf("response mode = %q, want normalized full mode", result.ResponseMode)
	}
	if result.ResolvedTarget == nil || result.ResolvedTarget.QueueQuery == nil || result.ResolvedTarget.QueueQuery.Platform != "amazon" {
		t.Fatalf("resolved target = %+v, want amazon queue query", result.ResolvedTarget)
	}
	if result.ResolvedTarget.ActionKey != assetGenerationActionRunPlatformAdaptTemporal || result.ResolvedTarget.InteractionMode != "queue_only" {
		t.Fatalf("resolved target = %+v, want queue_only platform temporal target", result.ResolvedTarget)
	}
	if result.Audit == nil || result.Audit.RequestedActionKey != assetGenerationActionRunPlatformAdaptTemporal || result.Audit.ResolvedActionKey != assetGenerationActionRunPlatformAdaptTemporal || result.Audit.ResolutionSource != "layer_temporal" || result.Audit.ExecutionPath != "queue_only" {
		t.Fatalf("audit = %+v, want queue_only layer_temporal audit", result.Audit)
	}
	if result.Audit.ExecutedAt.IsZero() {
		t.Fatalf("audit = %+v, want executed timestamp", result.Audit)
	}
}

func TestTaskGenerationLayerTemporalStandardServiceDelegatesStandardPhase(t *testing.T) {
	t.Parallel()

	source := readFunctionSourceMatching(t, "task_generation_service.go", "method executeLayerTemporalAction", func(decl *ast.FuncDecl) bool {
		if decl.Name == nil || decl.Name.Name != "executeLayerTemporalAction" || decl.Recv == nil || len(decl.Recv.List) != 1 {
			return false
		}
		star, ok := decl.Recv.List[0].Type.(*ast.StarExpr)
		if !ok {
			return false
		}
		ident, ok := star.X.(*ast.Ident)
		return ok && ident.Name == "taskGenerationService"
	})

	assertSourceContainsAll(t, source, []string{
		"buildTaskGenerationActionTemporalStandardPhase(s).run(ctx, taskID, req)",
	})
	assertSourceExcludesAll(t, source, []string{
		"client.StartStandardProduct(",
		`fmt.Errorf("standard product temporal workflow is not configured")`,
	})
}

func TestTaskGenerationLayerTemporalPlatformServiceDelegatesPlatformPhase(t *testing.T) {
	t.Parallel()

	source := readFunctionSourceMatching(t, "task_generation_service.go", "method executeLayerTemporalAction", func(decl *ast.FuncDecl) bool {
		if decl.Name == nil || decl.Name.Name != "executeLayerTemporalAction" || decl.Recv == nil || len(decl.Recv.List) != 1 {
			return false
		}
		star, ok := decl.Recv.List[0].Type.(*ast.StarExpr)
		if !ok {
			return false
		}
		ident, ok := star.X.(*ast.Ident)
		return ok && ident.Name == "taskGenerationService"
	})

	assertSourceContainsAll(t, source, []string{
		"buildTaskGenerationActionTemporalPlatformPhase(s).run(ctx, taskID, req)",
	})
	assertSourceExcludesAll(t, source, []string{
		"client.StartPlatformAdaptation(",
		`fmt.Errorf("platform adaptation temporal workflow is not configured")`,
		"resolveLayerTemporalPlatform(req)",
	})
}

func TestTaskGenerationLayerTemporalStandardPhaseRejectsUnconfiguredWorkflow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		service *taskGenerationService
	}{
		{
			name: "disabled workflow",
			service: &taskGenerationService{
				standardWorkflow: func() (StandardProductWorkflowClient, bool) {
					return &stubStandardProductWorkflowClient{}, false
				},
			},
		},
		{
			name: "nil client",
			service: &taskGenerationService{
				standardWorkflow: func() (StandardProductWorkflowClient, bool) {
					return nil, true
				},
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result, err := buildTaskGenerationActionTemporalStandardPhase(tc.service).run(
				context.Background(),
				" task-generation-action-standard-seam-error-1 ",
				&ExecuteGenerationActionRequest{ActionKey: assetGenerationActionRunStandardProductTemporal},
			)
			if err == nil {
				t.Fatal("taskGenerationActionTemporalStandardPhase.run() error = nil, want unconfigured error")
			}
			if err.Error() != "standard product temporal workflow is not configured" {
				t.Fatalf("taskGenerationActionTemporalStandardPhase.run() error = %v, want exact unconfigured error", err)
			}
			if result != nil {
				t.Fatalf("result = %+v, want nil result when workflow is unconfigured", result)
			}
		})
	}
}

func TestTaskGenerationLayerTemporalPlatformPhaseRejectsUnconfiguredWorkflow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		service *taskGenerationService
	}{
		{
			name: "disabled workflow",
			service: &taskGenerationService{
				platformAdaptWorkflow: func() (PlatformAdaptWorkflowClient, bool) {
					return &stubPlatformAdaptWorkflowClient{}, false
				},
			},
		},
		{
			name: "nil client",
			service: &taskGenerationService{
				platformAdaptWorkflow: func() (PlatformAdaptWorkflowClient, bool) {
					return nil, true
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := buildTaskGenerationActionTemporalPlatformPhase(tt.service).run(context.Background(), "task-generation-action-platform-phase-unconfigured-1", &ExecuteGenerationActionRequest{
				ActionKey: assetGenerationActionRunPlatformAdaptTemporal,
			})
			if err == nil || err.Error() != "platform adaptation temporal workflow is not configured" {
				t.Fatalf("run() error = %v, want unconfigured workflow error", err)
			}
			if result != nil {
				t.Fatalf("result = %+v, want nil when workflow is unavailable", result)
			}
		})
	}
}

func TestTaskGenerationLayerTemporalPlatformPhaseStartsWorkflowAndDelegatesResult(t *testing.T) {
	t.Parallel()

	client := &stubPlatformAdaptWorkflowClient{}
	service := &taskGenerationService{
		platformAdaptWorkflow: func() (PlatformAdaptWorkflowClient, bool) {
			return client, true
		},
	}

	result, err := buildTaskGenerationActionTemporalPlatformPhase(service).run(context.Background(), "  task-generation-action-platform-phase-1  ", &ExecuteGenerationActionRequest{
		ActionKey:    assetGenerationActionRunPlatformAdaptTemporal,
		ResponseMode: "patch_only",
		Target: &AssetGenerationActionTarget{
			NavigationTarget: &GenerationReviewNavigationTarget{
				SessionQuery: &GenerationQueueQuery{
					Platform: " AMAZON ",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if len(client.calls) != 1 {
		t.Fatalf("platform adapt temporal calls = %+v, want single call", client.calls)
	}
	if client.calls[0].TaskID != "task-generation-action-platform-phase-1" {
		t.Fatalf("workflow call = %+v, want trimmed task id", client.calls[0])
	}
	if client.calls[0].Platform != "amazon" {
		t.Fatalf("workflow call = %+v, want normalized platform", client.calls[0])
	}
	if client.calls[0].RequestedAt.IsZero() {
		t.Fatalf("workflow call = %+v, want requested timestamp", client.calls[0])
	}
	if result == nil {
		t.Fatal("result = nil, want temporal result seam response")
	}
	if result.ActionKey != assetGenerationActionRunPlatformAdaptTemporal || result.InteractionMode != "queue_only" {
		t.Fatalf("result = %+v, want queue_only platform temporal result", result)
	}
	if result.ResponseMode != "patch_only" {
		t.Fatalf("response mode = %q, want patch_only", result.ResponseMode)
	}
	if result.ResolvedTarget == nil || result.ResolvedTarget.QueueQuery == nil || result.ResolvedTarget.QueueQuery.Platform != "amazon" {
		t.Fatalf("resolved target = %+v, want normalized amazon queue query", result.ResolvedTarget)
	}
}

func TestTaskGenerationLayerTemporalPlatformPhaseDefaultsToSheinWithoutPlatformInput(t *testing.T) {
	t.Parallel()

	client := &stubPlatformAdaptWorkflowClient{}
	service := &taskGenerationService{
		platformAdaptWorkflow: func() (PlatformAdaptWorkflowClient, bool) {
			return client, true
		},
	}

	result, err := buildTaskGenerationActionTemporalPlatformPhase(service).run(context.Background(), "task-generation-action-platform-phase-default-1", &ExecuteGenerationActionRequest{
		ActionKey: assetGenerationActionRunPlatformAdaptTemporal,
	})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if len(client.calls) != 1 {
		t.Fatalf("platform adapt temporal calls = %+v, want single call", client.calls)
	}
	if client.calls[0].Platform != "shein" {
		t.Fatalf("workflow call = %+v, want default shein platform", client.calls[0])
	}
	if result == nil || result.ResolvedTarget == nil || result.ResolvedTarget.QueueQuery == nil || result.ResolvedTarget.QueueQuery.Platform != "shein" {
		t.Fatalf("result = %+v, want default shein queue query", result)
	}
}

func TestTaskGenerationLayerTemporalPlatformPhasePropagatesWorkflowStartError(t *testing.T) {
	t.Parallel()

	client := &stubPlatformAdaptWorkflowClient{err: errors.New("start platform adaptation failed")}
	service := &taskGenerationService{
		platformAdaptWorkflow: func() (PlatformAdaptWorkflowClient, bool) {
			return client, true
		},
	}

	result, err := buildTaskGenerationActionTemporalPlatformPhase(service).run(context.Background(), "task-generation-action-platform-phase-error-1", &ExecuteGenerationActionRequest{
		ActionKey: assetGenerationActionRunPlatformAdaptTemporal,
		Target: &AssetGenerationActionTarget{
			QueueQuery: &GenerationQueueQuery{Platform: "amazon"},
		},
	})
	if err == nil || err.Error() != "start platform adaptation failed" {
		t.Fatalf("run() error = %v, want propagated workflow start error", err)
	}
	if result != nil {
		t.Fatalf("result = %+v, want nil when workflow start fails", result)
	}
	if len(client.calls) != 1 || client.calls[0].Platform != "amazon" {
		t.Fatalf("platform adapt temporal calls = %+v, want single amazon call before error", client.calls)
	}
}

func TestTaskGenerationLayerTemporalStandardPhaseStartsWorkflowAndShapesResult(t *testing.T) {
	t.Parallel()

	client := &stubStandardProductWorkflowClient{}
	service := &taskGenerationService{
		standardWorkflow: func() (StandardProductWorkflowClient, bool) {
			return client, true
		},
	}

	result, err := buildTaskGenerationActionTemporalStandardPhase(service).run(
		context.Background(),
		"  task-generation-action-standard-seam-1  ",
		&ExecuteGenerationActionRequest{
			ActionKey:     assetGenerationActionRunStandardProductTemporal,
			ResponseMode:  "",
			Target:        &AssetGenerationActionTarget{ActionKey: assetGenerationActionRunStandardProductTemporal},
		},
	)
	if err != nil {
		t.Fatalf("taskGenerationActionTemporalStandardPhase.run() error = %v", err)
	}
	if len(client.calls) != 1 {
		t.Fatalf("standard product temporal calls = %+v, want single workflow start", client.calls)
	}
	if client.calls[0].TaskID != "task-generation-action-standard-seam-1" {
		t.Fatalf("start input task id = %q, want trimmed task id", client.calls[0].TaskID)
	}
	if client.calls[0].RequestedAt.IsZero() {
		t.Fatalf("start input = %+v, want non-zero requested timestamp", client.calls[0])
	}
	if result == nil {
		t.Fatal("result = nil, want shared temporal result seam output")
	}
	if result.ActionKey != assetGenerationActionRunStandardProductTemporal || result.InteractionMode != "queue_only" {
		t.Fatalf("result = %+v, want standard queue_only temporal action result", result)
	}
	if result.ResponseMode != "full" {
		t.Fatalf("response mode = %q, want normalized full mode", result.ResponseMode)
	}
	if result.ResolvedTarget == nil || result.ResolvedTarget.ActionKey != assetGenerationActionRunStandardProductTemporal || result.ResolvedTarget.InteractionMode != "queue_only" {
		t.Fatalf("resolved target = %+v, want queue_only standard target", result.ResolvedTarget)
	}
	if result.ResolvedTarget.QueueQuery != nil {
		t.Fatalf("resolved target = %+v, want standard target without queue query", result.ResolvedTarget)
	}
	if result.Audit == nil || result.Audit.RequestedActionKey != assetGenerationActionRunStandardProductTemporal || result.Audit.ResolvedActionKey != assetGenerationActionRunStandardProductTemporal || result.Audit.ResolutionSource != "layer_temporal" || result.Audit.ExecutionPath != "queue_only" {
		t.Fatalf("audit = %+v, want shared temporal result audit", result.Audit)
	}
	if result.Audit.ExecutedAt.IsZero() {
		t.Fatalf("audit = %+v, want executed timestamp", result.Audit)
	}
}

func TestTaskGenerationLayerTemporalStandardPhaseReturnsStartError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("start standard temporal failed")
	client := &stubStandardProductWorkflowClient{err: wantErr}
	service := &taskGenerationService{
		standardWorkflow: func() (StandardProductWorkflowClient, bool) {
			return client, true
		},
	}

	result, err := buildTaskGenerationActionTemporalStandardPhase(service).run(
		context.Background(),
		"task-generation-action-standard-seam-start-error-1",
		&ExecuteGenerationActionRequest{
			ActionKey:    assetGenerationActionRunStandardProductTemporal,
			ResponseMode: "full",
		},
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("taskGenerationActionTemporalStandardPhase.run() error = %v, want %v", err, wantErr)
	}
	if result != nil {
		t.Fatalf("result = %+v, want nil result when workflow start fails", result)
	}
	if len(client.calls) != 1 {
		t.Fatalf("standard product temporal calls = %+v, want single attempted workflow start", client.calls)
	}
}

func TestTaskGenerationActionTemporalResultPhaseShapesStandardTemporalResult(t *testing.T) {
	t.Parallel()

	result := buildTaskGenerationActionTemporalResultPhase().run(
		assetGenerationActionRunStandardProductTemporal,
		"",
		nil,
	)
	if result == nil {
		t.Fatal("result = nil, want queue_only temporal result")
	}
	if result.ActionKey != assetGenerationActionRunStandardProductTemporal || result.InteractionMode != "queue_only" {
		t.Fatalf("result = %+v, want standard queue_only action result", result)
	}
	if result.ResponseMode != "full" {
		t.Fatalf("response mode = %q, want normalized full mode", result.ResponseMode)
	}
	if result.ResolvedTarget == nil || result.ResolvedTarget.ActionKey != assetGenerationActionRunStandardProductTemporal || result.ResolvedTarget.InteractionMode != "queue_only" {
		t.Fatalf("resolved target = %+v, want queue_only standard target", result.ResolvedTarget)
	}
	if result.ResolvedTarget.QueueQuery != nil {
		t.Fatalf("resolved target = %+v, want standard target without queue query", result.ResolvedTarget)
	}
	if result.Audit == nil || result.Audit.RequestedActionKey != assetGenerationActionRunStandardProductTemporal || result.Audit.ResolvedActionKey != assetGenerationActionRunStandardProductTemporal || result.Audit.ResolutionSource != "layer_temporal" || result.Audit.ExecutionPath != "queue_only" {
		t.Fatalf("audit = %+v, want queue_only layer_temporal audit", result.Audit)
	}
	if result.Audit.ExecutedAt.IsZero() {
		t.Fatalf("audit = %+v, want executed timestamp", result.Audit)
	}
}

func TestTaskGenerationActionTemporalResultPhaseShapesPlatformTemporalResult(t *testing.T) {
	t.Parallel()

	result := buildTaskGenerationActionTemporalResultPhase().run(
		assetGenerationActionRunPlatformAdaptTemporal,
		"patch_only",
		&GenerationQueueQuery{Platform: "amazon"},
	)
	if result == nil {
		t.Fatal("result = nil, want queue_only temporal result")
	}
	if result.ActionKey != assetGenerationActionRunPlatformAdaptTemporal || result.InteractionMode != "queue_only" {
		t.Fatalf("result = %+v, want platform queue_only action result", result)
	}
	if result.ResponseMode != "patch_only" {
		t.Fatalf("response mode = %q, want patch_only", result.ResponseMode)
	}
	if result.ResolvedTarget == nil || result.ResolvedTarget.ActionKey != assetGenerationActionRunPlatformAdaptTemporal || result.ResolvedTarget.InteractionMode != "queue_only" {
		t.Fatalf("resolved target = %+v, want queue_only platform target", result.ResolvedTarget)
	}
	if result.ResolvedTarget.QueueQuery == nil || result.ResolvedTarget.QueueQuery.Platform != "amazon" {
		t.Fatalf("resolved target = %+v, want amazon queue query", result.ResolvedTarget)
	}
	if result.Audit == nil || result.Audit.RequestedActionKey != assetGenerationActionRunPlatformAdaptTemporal || result.Audit.ResolvedActionKey != assetGenerationActionRunPlatformAdaptTemporal || result.Audit.ResolutionSource != "layer_temporal" || result.Audit.ExecutionPath != "queue_only" {
		t.Fatalf("audit = %+v, want queue_only layer_temporal audit", result.Audit)
	}
	if result.Audit.ExecutedAt.IsZero() {
		t.Fatalf("audit = %+v, want executed timestamp", result.Audit)
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

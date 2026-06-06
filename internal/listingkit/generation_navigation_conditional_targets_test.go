package listingkit

import (
	"context"
	"testing"
	"time"

	assetrepo "task-processor/internal/asset/repository"
	"task-processor/internal/listingkit/reviewstore"
	common "task-processor/internal/publishing/common"
)

func TestReviewSessionResponseCarriesConditionalNavigationTargets(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	svc := &service{
		repo:       repo,
		assetRepo:  assetrepo.NewMemRepository(),
		reviewRepo: reviewstore.NewMemRepository(),
	}

	task := newConditionalContractTestTask("task-generation-navigation-target-conditional-1")
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	response, err := svc.GetTaskGenerationReviewSession(context.Background(), task.ID, &GenerationQueueQuery{
		Platform:          "shein",
		Slot:              "main",
		PreviewCapability: "detail_preview",
	})
	if err != nil {
		t.Fatalf("GetTaskGenerationReviewSession() error = %v", err)
	}
	if response == nil || response.Session == nil || response.DeltaToken == "" {
		t.Fatalf("response = %+v, want full review session with delta token", response)
	}

	defaultTarget := response.Session.DefaultTarget
	if defaultTarget == nil || defaultTarget.NavigationTarget == nil {
		t.Fatalf("defaultTarget = %+v, want navigation target", defaultTarget)
	}
	if defaultTarget.NavigationTarget.Conditional == nil || defaultTarget.NavigationTarget.Conditional.DeltaToken != response.DeltaToken {
		t.Fatalf("defaultTarget navigation conditional = %+v, want response delta token", defaultTarget.NavigationTarget.Conditional)
	}
	if defaultTarget.NavigationTarget.ResourceKind != "review_session" || defaultTarget.NavigationTarget.CacheKey == "" {
		t.Fatalf("defaultTarget navigation target = %+v, want session resource identity", defaultTarget.NavigationTarget)
	}
	if defaultTarget.NavigationTarget.CachePolicy != "stale_while_revalidate" || defaultTarget.NavigationTarget.RevalidateAfterAction {
		t.Fatalf("defaultTarget navigation target = %+v, want session cache policy", defaultTarget.NavigationTarget)
	}
	if defaultTarget.NavigationTarget.Descriptor == nil ||
		defaultTarget.NavigationTarget.Descriptor.ResourceKind != defaultTarget.NavigationTarget.ResourceKind ||
		defaultTarget.NavigationTarget.Descriptor.CacheKey != defaultTarget.NavigationTarget.CacheKey ||
		!defaultTarget.NavigationTarget.Descriptor.SupportsStaleWhileRevalidate ||
		defaultTarget.NavigationTarget.Descriptor.RefreshScope != "panel_read" ||
		len(defaultTarget.NavigationTarget.Descriptor.Invalidates) != 1 || defaultTarget.NavigationTarget.Descriptor.Invalidates[0] != "review_session" ||
		defaultTarget.NavigationTarget.Descriptor.DispatchPlan == nil ||
		defaultTarget.NavigationTarget.Descriptor.DispatchPlan.Strategy != "fanout_read" ||
		defaultTarget.NavigationTarget.Descriptor.DispatchPlan.MaxParallelism != 3 ||
		defaultTarget.NavigationTarget.Descriptor.DispatchPlan.DedupePolicy != "by_step_identity" ||
		defaultTarget.NavigationTarget.Descriptor.DispatchPlan.WinnerPolicy != "prefer_preview_then_session_then_queue" ||
		defaultTarget.NavigationTarget.Descriptor.DispatchPlan.FallbackStrategy != "prefer_preview_then_session_then_queue" ||
		defaultTarget.NavigationTarget.Descriptor.DispatchPlan.StopOnNotModified ||
		defaultTarget.NavigationTarget.Descriptor.DispatchPlan.StopOnFirstSuccess ||
		defaultTarget.NavigationTarget.Descriptor.DispatchPlan.RequiresRevalidate ||
		len(defaultTarget.NavigationTarget.Descriptor.DispatchPlan.Steps) < 2 ||
		len(defaultTarget.NavigationTarget.Descriptor.FollowUpReads) < 2 ||
		defaultTarget.NavigationTarget.Descriptor.Conditional == nil ||
		defaultTarget.NavigationTarget.Descriptor.Conditional.DeltaToken != response.DeltaToken {
		t.Fatalf("defaultTarget descriptor = %+v, want unified session descriptor", defaultTarget.NavigationTarget.Descriptor)
	}
	if !hasDispatchStepCachePreference(defaultTarget.NavigationTarget.Descriptor.DispatchPlan, "stale_while_revalidate") ||
		!hasDispatchStepCachePreference(defaultTarget.NavigationTarget.Descriptor.DispatchPlan, "revalidate") {
		t.Fatalf("defaultTarget dispatch plan = %+v, want mixed stale/revalidate steps", defaultTarget.NavigationTarget.Descriptor.DispatchPlan)
	}
	if defaultTarget.NavigationTarget.SessionQuery == nil || defaultTarget.NavigationTarget.SessionQuery.IfMatch != response.DeltaToken {
		t.Fatalf("defaultTarget session query = %+v, want prefilled if_match", defaultTarget.NavigationTarget.SessionQuery)
	}
	if defaultTarget.NavigationTarget.PreviewQuery == nil || defaultTarget.NavigationTarget.PreviewQuery.IfMatch != response.DeltaToken {
		t.Fatalf("defaultTarget preview query = %+v, want prefilled if_match", defaultTarget.NavigationTarget.PreviewQuery)
	}
	if defaultTarget.NavigationTarget.QueueQuery == nil || defaultTarget.NavigationTarget.QueueQuery.IfMatch != response.DeltaToken {
		t.Fatalf("defaultTarget queue query = %+v, want prefilled if_match", defaultTarget.NavigationTarget.QueueQuery)
	}

	if response.Session.FocusedToolbar == nil || response.Session.FocusedToolbar.PreviewViewer == nil || response.Session.FocusedToolbar.PreviewViewer.NavigationTarget == nil {
		t.Fatalf("focused toolbar = %+v, want preview viewer navigation target", response.Session.FocusedToolbar)
	}
	if response.Session.FocusedToolbar.PreviewViewer.NavigationTarget.Conditional == nil || response.Session.FocusedToolbar.PreviewViewer.NavigationTarget.Conditional.DeltaToken != response.DeltaToken {
		t.Fatalf("preview viewer navigation conditional = %+v, want response delta token", response.Session.FocusedToolbar.PreviewViewer.NavigationTarget.Conditional)
	}
	if response.Session.FocusedToolbar.PreviewViewer.NavigationTarget.ResourceKind != "review_preview" || response.Session.FocusedToolbar.PreviewViewer.NavigationTarget.CacheKey == "" {
		t.Fatalf("preview viewer navigation target = %+v, want preview resource identity", response.Session.FocusedToolbar.PreviewViewer.NavigationTarget)
	}
	if response.Session.FocusedToolbar.PreviewViewer.NavigationTarget.CachePolicy != "stale_while_revalidate" || response.Session.FocusedToolbar.PreviewViewer.NavigationTarget.RevalidateAfterAction {
		t.Fatalf("preview viewer navigation target = %+v, want preview cache policy", response.Session.FocusedToolbar.PreviewViewer.NavigationTarget)
	}
	if response.Session.FocusedToolbar.PreviewViewer.NavigationTarget.Descriptor == nil ||
		response.Session.FocusedToolbar.PreviewViewer.NavigationTarget.Descriptor.ResourceKind != "review_preview" ||
		response.Session.FocusedToolbar.PreviewViewer.NavigationTarget.Descriptor.RefreshScope != "focused_read" ||
		response.Session.FocusedToolbar.PreviewViewer.NavigationTarget.Descriptor.DispatchPlan == nil ||
		response.Session.FocusedToolbar.PreviewViewer.NavigationTarget.Descriptor.DispatchPlan.Strategy != "fanout_read" ||
		response.Session.FocusedToolbar.PreviewViewer.NavigationTarget.Descriptor.DispatchPlan.MaxParallelism != 3 ||
		response.Session.FocusedToolbar.PreviewViewer.NavigationTarget.Descriptor.DispatchPlan.DedupePolicy != "by_step_identity" ||
		response.Session.FocusedToolbar.PreviewViewer.NavigationTarget.Descriptor.DispatchPlan.WinnerPolicy != "prefer_preview_then_session_then_queue" ||
		response.Session.FocusedToolbar.PreviewViewer.NavigationTarget.Descriptor.DispatchPlan.FallbackStrategy != "prefer_preview_then_session_then_queue" ||
		response.Session.FocusedToolbar.PreviewViewer.NavigationTarget.Descriptor.DispatchPlan.StopOnNotModified ||
		response.Session.FocusedToolbar.PreviewViewer.NavigationTarget.Descriptor.DispatchPlan.StopOnFirstSuccess ||
		len(response.Session.FocusedToolbar.PreviewViewer.NavigationTarget.Descriptor.DispatchPlan.Steps) < 2 ||
		len(response.Session.FocusedToolbar.PreviewViewer.NavigationTarget.Descriptor.Invalidates) != 1 || response.Session.FocusedToolbar.PreviewViewer.NavigationTarget.Descriptor.Invalidates[0] != "review_preview" ||
		len(response.Session.FocusedToolbar.PreviewViewer.NavigationTarget.Descriptor.FollowUpReads) == 0 ||
		!response.Session.FocusedToolbar.PreviewViewer.NavigationTarget.Descriptor.SupportsStaleWhileRevalidate {
		t.Fatalf("preview viewer descriptor = %+v, want unified preview descriptor", response.Session.FocusedToolbar.PreviewViewer.NavigationTarget.Descriptor)
	}
	if !hasDispatchStepCachePreference(response.Session.FocusedToolbar.PreviewViewer.NavigationTarget.Descriptor.DispatchPlan, "stale_while_revalidate") ||
		!hasDispatchStepCachePreference(response.Session.FocusedToolbar.PreviewViewer.NavigationTarget.Descriptor.DispatchPlan, "revalidate") {
		t.Fatalf("preview viewer dispatch plan = %+v, want mixed stale/revalidate steps", response.Session.FocusedToolbar.PreviewViewer.NavigationTarget.Descriptor.DispatchPlan)
	}
	if response.Session.FocusedToolbar.PreviewViewer.NavigationTarget.PreviewQuery == nil || response.Session.FocusedToolbar.PreviewViewer.NavigationTarget.PreviewQuery.IfMatch != response.DeltaToken {
		t.Fatalf("preview viewer navigation preview query = %+v, want prefilled if_match", response.Session.FocusedToolbar.PreviewViewer.NavigationTarget.PreviewQuery)
	}
	if len(response.ResourceDescriptors) == 0 {
		t.Fatalf("session response resource descriptors = %+v, want session response descriptors", response.ResourceDescriptors)
	}
}

func TestReviewPreviewResponseCarriesConditionalNavigationTargets(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	svc := &service{
		repo:       repo,
		assetRepo:  assetrepo.NewMemRepository(),
		reviewRepo: reviewstore.NewMemRepository(),
	}

	task := newConditionalContractTestTask("task-generation-navigation-target-conditional-2")
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	response, err := svc.GetTaskGenerationReviewPreview(context.Background(), task.ID, &GenerationQueueQuery{
		Platform:          "shein",
		Slot:              "main",
		PreviewCapability: "detail_preview",
	})
	if err != nil {
		t.Fatalf("GetTaskGenerationReviewPreview() error = %v", err)
	}
	if response == nil || response.DeltaToken == "" || response.ReviewTarget == nil || response.ReviewTarget.NavigationTarget == nil {
		t.Fatalf("response = %+v, want review target navigation target", response)
	}
	if response.ReviewTarget.NavigationTarget.Conditional == nil || response.ReviewTarget.NavigationTarget.Conditional.DeltaToken != response.DeltaToken {
		t.Fatalf("review target navigation conditional = %+v, want response delta token", response.ReviewTarget.NavigationTarget.Conditional)
	}
	if response.ReviewTarget.NavigationTarget.ResourceKind != "review_session" || response.ReviewTarget.NavigationTarget.CacheKey == "" {
		t.Fatalf("review target navigation target = %+v, want session resource identity", response.ReviewTarget.NavigationTarget)
	}
	if response.ReviewTarget.NavigationTarget.CachePolicy != "stale_while_revalidate" || response.ReviewTarget.NavigationTarget.RevalidateAfterAction {
		t.Fatalf("review target navigation target = %+v, want session cache policy", response.ReviewTarget.NavigationTarget)
	}
	if response.ReviewTarget.NavigationTarget.Descriptor == nil ||
		response.ReviewTarget.NavigationTarget.Descriptor.ResourceKind != "review_session" ||
		response.ReviewTarget.NavigationTarget.Descriptor.RefreshScope != "panel_read" ||
		response.ReviewTarget.NavigationTarget.Descriptor.DispatchPlan == nil ||
		response.ReviewTarget.NavigationTarget.Descriptor.DispatchPlan.Strategy != "fanout_read" ||
		response.ReviewTarget.NavigationTarget.Descriptor.DispatchPlan.MaxParallelism != 3 ||
		response.ReviewTarget.NavigationTarget.Descriptor.DispatchPlan.DedupePolicy != "by_step_identity" ||
		response.ReviewTarget.NavigationTarget.Descriptor.DispatchPlan.WinnerPolicy != "prefer_preview_then_session_then_queue" ||
		response.ReviewTarget.NavigationTarget.Descriptor.DispatchPlan.FallbackStrategy != "prefer_preview_then_session_then_queue" ||
		response.ReviewTarget.NavigationTarget.Descriptor.DispatchPlan.StopOnNotModified ||
		response.ReviewTarget.NavigationTarget.Descriptor.DispatchPlan.StopOnFirstSuccess ||
		!response.ReviewTarget.NavigationTarget.Descriptor.SupportsStaleWhileRevalidate {
		t.Fatalf("review target descriptor = %+v, want unified session descriptor", response.ReviewTarget.NavigationTarget.Descriptor)
	}
	if !hasDispatchStepCachePreference(response.ReviewTarget.NavigationTarget.Descriptor.DispatchPlan, "stale_while_revalidate") ||
		!hasDispatchStepCachePreference(response.ReviewTarget.NavigationTarget.Descriptor.DispatchPlan, "revalidate") {
		t.Fatalf("review target dispatch plan = %+v, want mixed stale/revalidate steps", response.ReviewTarget.NavigationTarget.Descriptor.DispatchPlan)
	}
	if response.ReviewTarget.NavigationTarget.SessionQuery == nil || response.ReviewTarget.NavigationTarget.SessionQuery.IfMatch != response.DeltaToken {
		t.Fatalf("review target session query = %+v, want prefilled if_match", response.ReviewTarget.NavigationTarget.SessionQuery)
	}
	if response.Toolbar == nil || len(response.Toolbar.PreviewActions) == 0 || response.Toolbar.PreviewActions[0].NavigationTarget == nil {
		t.Fatalf("toolbar = %+v, want preview action navigation target", response.Toolbar)
	}
	if response.Toolbar.PreviewActions[0].NavigationTarget.Conditional == nil || response.Toolbar.PreviewActions[0].NavigationTarget.Conditional.DeltaToken != response.DeltaToken {
		t.Fatalf("preview action navigation conditional = %+v, want response delta token", response.Toolbar.PreviewActions[0].NavigationTarget.Conditional)
	}
	if response.Toolbar.PreviewActions[0].NavigationTarget.ResourceKind != "review_preview" || response.Toolbar.PreviewActions[0].NavigationTarget.CacheKey == "" {
		t.Fatalf("preview action navigation target = %+v, want preview resource identity", response.Toolbar.PreviewActions[0].NavigationTarget)
	}
	if response.Toolbar.PreviewActions[0].NavigationTarget.CachePolicy != "stale_while_revalidate" || response.Toolbar.PreviewActions[0].NavigationTarget.RevalidateAfterAction {
		t.Fatalf("preview action navigation target = %+v, want preview cache policy", response.Toolbar.PreviewActions[0].NavigationTarget)
	}
	if response.Toolbar.PreviewActions[0].NavigationTarget.Descriptor == nil ||
		response.Toolbar.PreviewActions[0].NavigationTarget.Descriptor.ResourceKind != "review_preview" ||
		response.Toolbar.PreviewActions[0].NavigationTarget.Descriptor.RefreshScope != "focused_read" ||
		response.Toolbar.PreviewActions[0].NavigationTarget.Descriptor.DispatchPlan == nil ||
		response.Toolbar.PreviewActions[0].NavigationTarget.Descriptor.DispatchPlan.Strategy != "fanout_read" ||
		response.Toolbar.PreviewActions[0].NavigationTarget.Descriptor.DispatchPlan.MaxParallelism != 3 ||
		response.Toolbar.PreviewActions[0].NavigationTarget.Descriptor.DispatchPlan.DedupePolicy != "by_step_identity" ||
		response.Toolbar.PreviewActions[0].NavigationTarget.Descriptor.DispatchPlan.WinnerPolicy != "prefer_preview_then_session_then_queue" ||
		response.Toolbar.PreviewActions[0].NavigationTarget.Descriptor.DispatchPlan.FallbackStrategy != "prefer_preview_then_session_then_queue" ||
		response.Toolbar.PreviewActions[0].NavigationTarget.Descriptor.DispatchPlan.StopOnNotModified ||
		response.Toolbar.PreviewActions[0].NavigationTarget.Descriptor.DispatchPlan.StopOnFirstSuccess ||
		!response.Toolbar.PreviewActions[0].NavigationTarget.Descriptor.SupportsStaleWhileRevalidate {
		t.Fatalf("preview action descriptor = %+v, want unified preview descriptor", response.Toolbar.PreviewActions[0].NavigationTarget.Descriptor)
	}
	if !hasDispatchStepCachePreference(response.Toolbar.PreviewActions[0].NavigationTarget.Descriptor.DispatchPlan, "stale_while_revalidate") ||
		!hasDispatchStepCachePreference(response.Toolbar.PreviewActions[0].NavigationTarget.Descriptor.DispatchPlan, "revalidate") {
		t.Fatalf("preview action dispatch plan = %+v, want mixed stale/revalidate steps", response.Toolbar.PreviewActions[0].NavigationTarget.Descriptor.DispatchPlan)
	}
	if response.Toolbar.PreviewActions[0].NavigationTarget.PreviewQuery == nil || response.Toolbar.PreviewActions[0].NavigationTarget.PreviewQuery.IfMatch != response.DeltaToken {
		t.Fatalf("preview action preview query = %+v, want prefilled if_match", response.Toolbar.PreviewActions[0].NavigationTarget.PreviewQuery)
	}
	if len(response.ResourceDescriptors) == 0 {
		t.Fatalf("preview response resource descriptors = %+v, want preview response descriptors", response.ResourceDescriptors)
	}
	if len(response.Toolbar.PreviewActions) > 1 {
		if response.Toolbar.PreviewActions[1].NavigationTarget == nil || response.Toolbar.PreviewActions[1].NavigationTarget.ResourceKind != "generation_action" || response.Toolbar.PreviewActions[1].NavigationTarget.CacheKey == "" {
			t.Fatalf("workflow action navigation target = %+v, want action resource identity", response.Toolbar.PreviewActions[1].NavigationTarget)
		}
		if response.Toolbar.PreviewActions[1].NavigationTarget.CachePolicy != "network_only" || !response.Toolbar.PreviewActions[1].NavigationTarget.RevalidateAfterAction {
			t.Fatalf("workflow action navigation target = %+v, want action cache policy", response.Toolbar.PreviewActions[1].NavigationTarget)
		}
		if response.Toolbar.PreviewActions[1].NavigationTarget.Descriptor == nil ||
			response.Toolbar.PreviewActions[1].NavigationTarget.Descriptor.ResourceKind != "generation_action" ||
			response.Toolbar.PreviewActions[1].NavigationTarget.Descriptor.RefreshScope != "mutation" ||
			response.Toolbar.PreviewActions[1].NavigationTarget.Descriptor.DispatchPlan == nil ||
			response.Toolbar.PreviewActions[1].NavigationTarget.Descriptor.DispatchPlan.Strategy != "mutation_then_refresh" ||
			response.Toolbar.PreviewActions[1].NavigationTarget.Descriptor.DispatchPlan.MaxParallelism != 2 ||
			response.Toolbar.PreviewActions[1].NavigationTarget.Descriptor.DispatchPlan.DedupePolicy != "by_step_identity" ||
			response.Toolbar.PreviewActions[1].NavigationTarget.Descriptor.DispatchPlan.WinnerPolicy != "prefer_preview_then_session_then_queue" ||
			response.Toolbar.PreviewActions[1].NavigationTarget.Descriptor.DispatchPlan.FallbackStrategy != "prefer_action_then_refresh_results" ||
			response.Toolbar.PreviewActions[1].NavigationTarget.Descriptor.DispatchPlan.StopOnNotModified ||
			response.Toolbar.PreviewActions[1].NavigationTarget.Descriptor.DispatchPlan.StopOnFirstSuccess ||
			!response.Toolbar.PreviewActions[1].NavigationTarget.Descriptor.DispatchPlan.RequiresRevalidate ||
			len(response.Toolbar.PreviewActions[1].NavigationTarget.Descriptor.DispatchPlan.Steps) == 0 ||
			response.Toolbar.PreviewActions[1].NavigationTarget.Descriptor.DispatchPlan.Steps[0].CachePreference != "revalidate" ||
			!response.Toolbar.PreviewActions[1].NavigationTarget.Descriptor.DispatchPlan.Steps[0].RequiresRevalidate ||
			len(response.Toolbar.PreviewActions[1].NavigationTarget.Descriptor.Invalidates) != 3 ||
			len(response.Toolbar.PreviewActions[1].NavigationTarget.Descriptor.FollowUpReads) == 0 ||
			response.Toolbar.PreviewActions[1].NavigationTarget.Descriptor.SupportsStaleWhileRevalidate ||
			!response.Toolbar.PreviewActions[1].NavigationTarget.Descriptor.RevalidateAfterAction {
			t.Fatalf("workflow action descriptor = %+v, want unified action descriptor", response.Toolbar.PreviewActions[1].NavigationTarget.Descriptor)
		}
	}
}

func hasDispatchStepCachePreference(plan *GenerationNavigationDispatchPlan, cachePreference string) bool {
	if plan == nil {
		return false
	}
	for _, step := range plan.Steps {
		if step.CachePreference == cachePreference {
			return true
		}
	}
	return false
}

func TestDispatchTaskGenerationNavigationUsesTargetConditionalBaseline(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	svc := &service{repo: repo}
	task := &Task{
		ID:        "task-generation-navigation-conditional-baseline-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"shein"}},
		Result: &ListingKitResult{
			TaskID: "task-generation-navigation-conditional-baseline-1",
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
		Platform:          "shein",
		Slot:              "main",
		PreviewCapability: "detail_preview",
	})
	if err != nil {
		t.Fatalf("GetTaskGenerationReviewPreview() error = %v", err)
	}
	if first == nil || first.DeltaToken == "" {
		t.Fatalf("first = %+v, want delta token", first)
	}

	response, err := svc.DispatchTaskGenerationNavigation(context.Background(), task.ID, &GenerationReviewNavigationDispatchRequest{
		ResponseMode: "patch_only",
		Target: &GenerationReviewNavigationTarget{
			DispatchKind: "preview",
			Conditional:  &GenerationConditionalState{DeltaToken: first.DeltaToken},
			PreviewQuery: &GenerationQueueQuery{
				Platform:          "shein",
				Slot:              "main",
				PreviewCapability: "detail_preview",
			},
		},
	})
	if err != nil {
		t.Fatalf("DispatchTaskGenerationNavigation() error = %v", err)
	}
	if response == nil || !response.NotModified || response.DeltaToken != first.DeltaToken {
		t.Fatalf("response = %+v, want not_modified dispatch from target baseline", response)
	}
}

func TestDispatchTaskGenerationNavigationKeepsExplicitQueryOverConditionalBaseline(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	svc := &service{repo: repo}
	task := &Task{
		ID:        "task-generation-navigation-conditional-baseline-2",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"shein"}},
		Result: &ListingKitResult{
			TaskID: "task-generation-navigation-conditional-baseline-2",
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

	response, err := svc.DispatchTaskGenerationNavigation(context.Background(), task.ID, &GenerationReviewNavigationDispatchRequest{
		ResponseMode: "patch_only",
		Target: &GenerationReviewNavigationTarget{
			DispatchKind: "preview",
			Conditional:  &GenerationConditionalState{DeltaToken: "delta-body"},
			PreviewQuery: &GenerationQueueQuery{
				Platform:          "shein",
				Slot:              "main",
				PreviewCapability: "detail_preview",
				IfMatch:           "delta-explicit",
			},
		},
	})
	if err != nil {
		t.Fatalf("DispatchTaskGenerationNavigation() error = %v", err)
	}
	if response == nil || response.NotModified {
		t.Fatalf("response = %+v, want explicit query to win over body baseline", response)
	}
}

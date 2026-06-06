package listingkit

import (
	"context"
	"testing"
	"time"

	assetrepo "task-processor/internal/asset/repository"
	"task-processor/internal/listingkit/reviewstore"
	common "task-processor/internal/publishing/common"
)

func TestGenerationActionDeltaTokenMatchesSubsequentReviewReads(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	svc := &service{
		repo:       repo,
		assetRepo:  assetrepo.NewMemRepository(),
		reviewRepo: reviewstore.NewMemRepository(),
	}

	task := newConditionalContractTestTask("task-generation-conditional-contract-1")
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	action, err := svc.ExecuteTaskGenerationAction(context.Background(), task.ID, &ExecuteGenerationActionRequest{
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
	if action == nil || action.DeltaToken == "" {
		t.Fatalf("action = %+v, want delta token", action)
	}
	if action.Conditional == nil || action.Conditional.DeltaToken != action.DeltaToken || action.Conditional.ETag == "" || action.Conditional.NotModified || action.Conditional.NoChanges {
		t.Fatalf("action conditional = %+v, want populated mutable conditional metadata", action.Conditional)
	}

	query := &GenerationQueueQuery{
		Platform:          "shein",
		Slot:              "main",
		PreviewCapability: "detail_preview",
	}
	session, err := svc.GetTaskGenerationReviewSession(context.Background(), task.ID, query)
	if err != nil {
		t.Fatalf("GetTaskGenerationReviewSession() error = %v", err)
	}
	if session == nil || session.DeltaToken != action.DeltaToken {
		t.Fatalf("session = %+v, want delta token %q", session, action.DeltaToken)
	}
	if session.Conditional == nil || session.Conditional.DeltaToken != session.DeltaToken || session.Conditional.ETag == "" || session.Conditional.NotModified || session.Conditional.NoChanges {
		t.Fatalf("session conditional = %+v, want populated read conditional metadata", session.Conditional)
	}

	preview, err := svc.GetTaskGenerationReviewPreview(context.Background(), task.ID, query)
	if err != nil {
		t.Fatalf("GetTaskGenerationReviewPreview() error = %v", err)
	}
	if preview == nil || preview.DeltaToken != action.DeltaToken {
		t.Fatalf("preview = %+v, want delta token %q", preview, action.DeltaToken)
	}
	if preview.Conditional == nil || preview.Conditional.DeltaToken != preview.DeltaToken || preview.Conditional.ETag == "" || preview.Conditional.NotModified || preview.Conditional.NoChanges {
		t.Fatalf("preview conditional = %+v, want populated read conditional metadata", preview.Conditional)
	}

	sessionNotModified, err := svc.GetTaskGenerationReviewSession(context.Background(), task.ID, &GenerationQueueQuery{
		Platform:          "shein",
		Slot:              "main",
		PreviewCapability: "detail_preview",
		IfMatch:           action.DeltaToken,
	})
	if err != nil {
		t.Fatalf("GetTaskGenerationReviewSession(not modified) error = %v", err)
	}
	if sessionNotModified == nil || !sessionNotModified.NotModified || sessionNotModified.DeltaToken != action.DeltaToken {
		t.Fatalf("sessionNotModified = %+v, want not_modified with action delta token", sessionNotModified)
	}
	if sessionNotModified.Conditional == nil || !sessionNotModified.Conditional.NotModified || sessionNotModified.Conditional.DeltaToken != action.DeltaToken {
		t.Fatalf("sessionNotModified conditional = %+v, want not_modified conditional metadata", sessionNotModified.Conditional)
	}

	previewNotModified, err := svc.GetTaskGenerationReviewPreview(context.Background(), task.ID, &GenerationQueueQuery{
		Platform:          "shein",
		Slot:              "main",
		PreviewCapability: "detail_preview",
		IfMatch:           action.DeltaToken,
	})
	if err != nil {
		t.Fatalf("GetTaskGenerationReviewPreview(not modified) error = %v", err)
	}
	if previewNotModified == nil || !previewNotModified.NotModified || previewNotModified.DeltaToken != action.DeltaToken {
		t.Fatalf("previewNotModified = %+v, want not_modified with action delta token", previewNotModified)
	}
	if previewNotModified.Conditional == nil || !previewNotModified.Conditional.NotModified || previewNotModified.Conditional.DeltaToken != action.DeltaToken {
		t.Fatalf("previewNotModified conditional = %+v, want not_modified conditional metadata", previewNotModified.Conditional)
	}
}

func TestDispatchTaskGenerationNavigationReusesActionDeltaTokenForConditionalReads(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	svc := &service{
		repo:       repo,
		assetRepo:  assetrepo.NewMemRepository(),
		reviewRepo: reviewstore.NewMemRepository(),
	}

	task := newConditionalContractTestTask("task-generation-conditional-contract-2")
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	action, err := svc.ExecuteTaskGenerationAction(context.Background(), task.ID, &ExecuteGenerationActionRequest{
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
	if action == nil || action.DeltaToken == "" {
		t.Fatalf("action = %+v, want delta token", action)
	}

	sessionDispatch, err := svc.DispatchTaskGenerationNavigation(context.Background(), task.ID, &GenerationReviewNavigationDispatchRequest{
		ResponseMode: "patch_only",
		Target: &GenerationReviewNavigationTarget{
			DispatchKind: "session",
			SessionQuery: &GenerationQueueQuery{
				Platform:          "shein",
				Slot:              "main",
				PreviewCapability: "detail_preview",
				IfMatch:           action.DeltaToken,
				ResponseMode:      "patch_only",
			},
		},
	})
	if err != nil {
		t.Fatalf("DispatchTaskGenerationNavigation(session) error = %v", err)
	}
	if sessionDispatch == nil || !sessionDispatch.NotModified || sessionDispatch.DeltaToken != action.DeltaToken {
		t.Fatalf("sessionDispatch = %+v, want session not_modified with action delta token", sessionDispatch)
	}
	if sessionDispatch.Conditional == nil || !sessionDispatch.Conditional.NotModified || sessionDispatch.Conditional.DeltaToken != action.DeltaToken {
		t.Fatalf("sessionDispatch conditional = %+v, want not_modified dispatch conditional metadata", sessionDispatch.Conditional)
	}
	if sessionDispatch.PanelUpdate == nil || !sessionDispatch.PanelUpdate.NoChanges {
		t.Fatalf("sessionDispatch = %+v, want session no_changes panel update", sessionDispatch)
	}
	if sessionDispatch.PanelUpdate.Conditional == nil || !sessionDispatch.PanelUpdate.Conditional.NoChanges || sessionDispatch.PanelUpdate.Conditional.DeltaToken != action.DeltaToken {
		t.Fatalf("sessionDispatch panel conditional = %+v, want no_changes panel conditional metadata", sessionDispatch.PanelUpdate.Conditional)
	}

	previewDispatch, err := svc.DispatchTaskGenerationNavigation(context.Background(), task.ID, &GenerationReviewNavigationDispatchRequest{
		ResponseMode: "patch_only",
		Target: &GenerationReviewNavigationTarget{
			DispatchKind: "preview",
			PreviewQuery: &GenerationQueueQuery{
				Platform:          "shein",
				Slot:              "main",
				PreviewCapability: "detail_preview",
				IfMatch:           action.DeltaToken,
			},
		},
	})
	if err != nil {
		t.Fatalf("DispatchTaskGenerationNavigation(preview) error = %v", err)
	}
	if previewDispatch == nil || !previewDispatch.NotModified || previewDispatch.DeltaToken != action.DeltaToken {
		t.Fatalf("previewDispatch = %+v, want preview not_modified with action delta token", previewDispatch)
	}
	if previewDispatch.Conditional == nil || !previewDispatch.Conditional.NotModified || previewDispatch.Conditional.DeltaToken != action.DeltaToken {
		t.Fatalf("previewDispatch conditional = %+v, want not_modified dispatch conditional metadata", previewDispatch.Conditional)
	}
	if previewDispatch.PanelUpdate == nil || !previewDispatch.PanelUpdate.NoChanges {
		t.Fatalf("previewDispatch = %+v, want preview no_changes panel update", previewDispatch)
	}
	if previewDispatch.PanelUpdate.Conditional == nil || !previewDispatch.PanelUpdate.Conditional.NoChanges || previewDispatch.PanelUpdate.Conditional.DeltaToken != action.DeltaToken {
		t.Fatalf("previewDispatch panel conditional = %+v, want no_changes panel conditional metadata", previewDispatch.PanelUpdate.Conditional)
	}
}

func TestGenerationQueueExposesUnifiedConditionalMetadata(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	svc := &service{
		repo:       repo,
		assetRepo:  assetrepo.NewMemRepository(),
		reviewRepo: reviewstore.NewMemRepository(),
	}

	task := newConditionalContractTestTask("task-generation-conditional-contract-3")
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	page, err := svc.GetTaskGenerationQueue(context.Background(), task.ID, &GenerationQueueQuery{
		Platform: "shein",
	})
	if err != nil {
		t.Fatalf("GetTaskGenerationQueue() error = %v", err)
	}
	if page == nil || page.DeltaToken == "" {
		t.Fatalf("page = %+v, want delta token", page)
	}
	if page.Conditional == nil || page.Conditional.DeltaToken != page.DeltaToken || page.Conditional.ETag == "" || page.Conditional.NotModified || page.Conditional.NoChanges {
		t.Fatalf("page conditional = %+v, want unified queue conditional metadata", page.Conditional)
	}
	if len(page.ResourceDescriptors) == 0 || page.ResourceDescriptors[0].Descriptor == nil || page.ResourceDescriptors[0].Descriptor.ResourceKind != "generation_queue" {
		t.Fatalf("page resource descriptors = %+v, want queue resource descriptors", page.ResourceDescriptors)
	}
	if page.ResourceDescriptors[0].Descriptor.RefreshScope != "collection_read" ||
		page.ResourceDescriptors[0].Descriptor.DispatchPlan == nil ||
		page.ResourceDescriptors[0].Descriptor.DispatchPlan.Strategy != "single_read" ||
		page.ResourceDescriptors[0].Descriptor.DispatchPlan.MaxParallelism != 1 ||
		page.ResourceDescriptors[0].Descriptor.DispatchPlan.DedupePolicy != "by_step_identity" ||
		page.ResourceDescriptors[0].Descriptor.DispatchPlan.WinnerPolicy != "prefer_preview_then_session_then_queue" ||
		page.ResourceDescriptors[0].Descriptor.DispatchPlan.FallbackStrategy != "prefer_queue_then_session" ||
		!page.ResourceDescriptors[0].Descriptor.DispatchPlan.StopOnNotModified ||
		!page.ResourceDescriptors[0].Descriptor.DispatchPlan.StopOnFirstSuccess ||
		!page.ResourceDescriptors[0].Descriptor.DispatchPlan.StopOnError ||
		page.ResourceDescriptors[0].Descriptor.DispatchPlan.RequiresRevalidate ||
		len(page.ResourceDescriptors[0].Descriptor.DispatchPlan.Steps) != 1 ||
		page.ResourceDescriptors[0].Descriptor.DispatchPlan.Steps[0].CachePreference != "revalidate" ||
		!page.ResourceDescriptors[0].Descriptor.DispatchPlan.Steps[0].RequiresRevalidate ||
		len(page.ResourceDescriptors[0].Descriptor.Invalidates) != 1 ||
		page.ResourceDescriptors[0].Descriptor.Invalidates[0] != "generation_queue" ||
		len(page.ResourceDescriptors[0].Descriptor.FollowUpReads) == 0 {
		t.Fatalf("page resource descriptor = %+v, want queue refresh contract", page.ResourceDescriptors[0].Descriptor)
	}

	notModified, err := svc.GetTaskGenerationQueue(context.Background(), task.ID, &GenerationQueueQuery{
		Platform: "shein",
		IfMatch:  page.DeltaToken,
	})
	if err != nil {
		t.Fatalf("GetTaskGenerationQueue(not modified) error = %v", err)
	}
	if notModified == nil || !notModified.NotModified || notModified.DeltaToken != page.DeltaToken {
		t.Fatalf("notModified = %+v, want not_modified queue response", notModified)
	}
	if notModified.Conditional == nil || !notModified.Conditional.NotModified || notModified.Conditional.DeltaToken != page.DeltaToken {
		t.Fatalf("notModified conditional = %+v, want unified not_modified queue metadata", notModified.Conditional)
	}
	if len(notModified.ResourceDescriptors) != 0 {
		t.Fatalf("notModified resource descriptors = %+v, want no descriptors for not_modified queue response", notModified.ResourceDescriptors)
	}
}

func newConditionalContractTestTask(taskID string) *Task {
	now := time.Now()
	return &Task{
		ID:        taskID,
		Status:    TaskStatusCompleted,
		CreatedAt: now,
		UpdatedAt: now,
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
}

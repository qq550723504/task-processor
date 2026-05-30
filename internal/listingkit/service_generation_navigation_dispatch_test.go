package listingkit

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	common "task-processor/internal/publishing/common"
)

func TestTaskGenerationNavigationPrimaryRunRoutesDispatchKinds(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	svc := &service{repo: repo}
	task := &Task{
		ID:        "task-generation-navigation-primary-route-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"shein"}},
		Result: &ListingKitResult{
			TaskID: "task-generation-navigation-primary-route-1",
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

	phase := buildTaskGenerationNavigationDispatchPrimaryPhase(svc.taskGenerationOrDefault())

	t.Run("action", func(t *testing.T) {
		t.Parallel()

		response, err := phase.run(context.Background(), task.ID, &GenerationReviewNavigationTarget{
			DispatchKind: "action",
			ActionTarget: &AssetGenerationActionTarget{
				ActionKey:       "approve_section_review",
				InteractionMode: "review_only",
				QueueQuery: &GenerationQueueQuery{
					Platform:          "shein",
					Slot:              "main",
					PreviewCapability: "detail_preview",
				},
			},
		}, "full")
		if err != nil {
			t.Fatalf("run() error = %v", err)
		}
		if response == nil || response.DispatchKind != "action" || response.Action == nil {
			t.Fatalf("run() response = %+v, want action dispatch response", response)
		}
		if response.Action.ActionKey != "approve_section_review" || response.Action.ResponseMode != "full" {
			t.Fatalf("run() action response = %+v, want routed action dispatch", response.Action)
		}
	})

	t.Run("preview", func(t *testing.T) {
		t.Parallel()

		response, err := phase.run(context.Background(), task.ID, &GenerationReviewNavigationTarget{
			DispatchKind: "preview",
			PreviewQuery: &GenerationQueueQuery{
				Platform: "shein",
				Slot:     "main",
			},
		}, "full")
		if err != nil {
			t.Fatalf("run() error = %v", err)
		}
		if response == nil || response.DispatchKind != "preview" || response.ReviewPreview == nil {
			t.Fatalf("run() response = %+v, want preview dispatch response", response)
		}
		if response.ReviewPreview.Viewer == nil || response.ReviewPreview.Viewer.Platform != "shein" {
			t.Fatalf("run() preview response = %+v, want routed preview query", response.ReviewPreview)
		}
	})

	t.Run("queue", func(t *testing.T) {
		t.Parallel()

		response, err := phase.run(context.Background(), task.ID, &GenerationReviewNavigationTarget{
			DispatchKind: "queue",
			QueueQuery: &GenerationQueueQuery{
				Platform: "shein",
				Slot:     "main",
			},
		}, "full")
		if err != nil {
			t.Fatalf("run() error = %v", err)
		}
		if response == nil || response.DispatchKind != "queue" || response.Queue == nil {
			t.Fatalf("run() response = %+v, want queue dispatch response", response)
		}
		if response.Queue.TaskID != task.ID {
			t.Fatalf("run() queue response = %+v, want task-specific queue response", response.Queue)
		}
	})

	t.Run("session_precedence", func(t *testing.T) {
		t.Parallel()

		response, err := phase.run(context.Background(), task.ID, &GenerationReviewNavigationTarget{
			SessionQuery: &GenerationQueueQuery{
				Platform:          "session-platform",
				Slot:              "session-slot",
				PreviewCapability: "session-cap",
			},
			QueueQuery: &GenerationQueueQuery{
				Platform:          "queue-platform",
				Slot:              "queue-slot",
				PreviewCapability: "queue-cap",
			},
			PreviewQuery: &GenerationQueueQuery{
				Platform:          "preview-platform",
				Slot:              "preview-slot",
				PreviewCapability: "preview-cap",
			},
		}, "full")
		if err != nil {
			t.Fatalf("run() error = %v", err)
		}
		if response == nil || response.DispatchKind != "session" || response.ReviewSession == nil || response.ReviewSession.Session == nil {
			t.Fatalf("run() response = %+v, want session dispatch response", response)
		}
		if response.ReviewSession.ResponseMode != "full" {
			t.Fatalf("run() session response mode = %q, want full", response.ReviewSession.ResponseMode)
		}
		if response.ReviewSession.Session.SelectedPlatform != "session-platform" ||
			response.ReviewSession.Session.FocusCapability != "session-cap" {
			t.Fatalf("run() session payload = %+v, want SessionQuery precedence over QueueQuery and PreviewQuery", response.ReviewSession.Session)
		}
	})
}

func TestTaskGenerationNavigationPrimaryServiceDelegatesToPhase(t *testing.T) {
	t.Parallel()

	source := readNavigationPrimarySource(t, "task_generation_service.go")
	assertSourceContainsAll(t, source, []string{
		"buildTaskGenerationNavigationDispatchPrimaryPhase(s).run(ctx, taskID, target, responseMode)",
	})
	assertSourceExcludesAll(t, source, []string{
		"switch normalizeGenerationReviewDispatchKind(target)",
		"ExecuteTaskGenerationAction(ctx, taskID, actionReq)",
		"GetTaskGenerationReviewPreview(ctx, taskID, cloneGenerationQueueQuery(target.PreviewQuery))",
		"GetTaskGenerationQueue(ctx, taskID, cloneGenerationQueueQuery(target.QueueQuery))",
		"GetTaskGenerationReviewSession(ctx, taskID, sessionQuery)",
	})
}

func TestTaskGenerationNavigationPrimaryPhaseOwnsPrimaryRouting(t *testing.T) {
	t.Parallel()

	source := readNavigationPrimarySource(t, "task_generation_navigation_dispatch_primary.go")
	assertSourceContainsAll(t, source, []string{
		"type taskGenerationNavigationDispatchPrimaryPhase struct",
		"buildTaskGenerationNavigationDispatchPrimaryPhase",
		"switch normalizeGenerationReviewDispatchKind(target)",
		"ExecuteTaskGenerationAction(ctx, taskID, actionReq)",
		"GetTaskGenerationReviewPreview(ctx, taskID, cloneGenerationQueueQuery(target.PreviewQuery))",
		"GetTaskGenerationQueue(ctx, taskID, cloneGenerationQueueQuery(target.QueueQuery))",
		"GetTaskGenerationReviewSession(ctx, taskID, sessionQuery)",
		"SessionQuery",
		"QueueQuery",
		"PreviewQuery",
	})
	assertSourceExcludesAll(t, source, []string{
		"executeGenerationNavigationDispatchPlan",
		"finalizeGenerationReviewNavigationDispatchResponse",
		"applyExecutedPlanToDispatchResponse",
	})
}

func readNavigationPrimarySource(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	return string(content)
}

func TestTaskGenerationNavigationDispatchEntryRunNormalizesTargetAndPlanMode(t *testing.T) {
	t.Parallel()

	entry := buildTaskGenerationNavigationDispatchEntry()
	target := &GenerationReviewNavigationTarget{
		DispatchKind: "session",
		Conditional: &GenerationConditionalState{
			DeltaToken: "nav-delta-123",
		},
		SessionQuery: &GenerationQueueQuery{
			Platform: "shein",
			Slot:     "main",
		},
	}

	input, err := entry.run(&GenerationReviewNavigationDispatchRequest{
		ResponseMode: "patch_only",
		PlanMode:     " execute_plan ",
		Target:       target,
	})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if input == nil {
		t.Fatalf("run() input = nil, want normalized dispatch input")
	}
	if input.target == nil {
		t.Fatalf("run() target = nil, want cloned target")
	}
	if input.target == target {
		t.Fatalf("run() target = original pointer, want clone")
	}
	if input.responseMode != "patch_only" {
		t.Fatalf("run() responseMode = %q, want patch_only", input.responseMode)
	}
	if input.planMode != "execute_plan" {
		t.Fatalf("run() planMode = %q, want execute_plan", input.planMode)
	}
	defaultInput, err := entry.run(&GenerationReviewNavigationDispatchRequest{
		Target: target,
	})
	if err != nil {
		t.Fatalf("run() default plan mode error = %v", err)
	}
	if defaultInput.planMode != "primary_only" {
		t.Fatalf("run() default planMode = %q, want primary_only", defaultInput.planMode)
	}
	if target.SessionQuery.IfMatch != "" || target.SessionQuery.DeltaToken != "" {
		t.Fatalf("original target = %+v, want unchanged source target", target.SessionQuery)
	}
	if input.target.SessionQuery.IfMatch != "nav-delta-123" {
		t.Fatalf("cloned target session query = %+v, want conditional baseline applied", input.target.SessionQuery)
	}
	if input.target.Conditional == target.Conditional {
		t.Fatalf("cloned target conditional = %+v, want cloned conditional state", input.target.Conditional)
	}
}

func TestTaskGenerationNavigationDispatchEntryRunRejectsMissingTarget(t *testing.T) {
	t.Parallel()

	entry := buildTaskGenerationNavigationDispatchEntry()

	tests := []struct {
		name string
		req  *GenerationReviewNavigationDispatchRequest
	}{
		{
			name: "nil_request",
			req:  nil,
		},
		{
			name: "nil_target",
			req: &GenerationReviewNavigationDispatchRequest{
				Target: nil,
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			input, err := entry.run(tc.req)
			if !errors.Is(err, ErrGenerationActionNotFound) {
				t.Fatalf("run() error = %v, want ErrGenerationActionNotFound", err)
			}
			if input != nil {
				t.Fatalf("run() input = %+v, want nil", input)
			}
		})
	}
}

func TestDispatchTaskGenerationNavigationDefaultsPlanModeToPrimaryOnly(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	svc := &service{repo: repo}
	task := &Task{
		ID:        "task-generation-navigation-default-plan-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"shein"}},
		Result: &ListingKitResult{
			TaskID: "task-generation-navigation-default-plan-1",
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
		Target: &GenerationReviewNavigationTarget{
			DispatchKind: "session",
			SessionQuery: &GenerationQueueQuery{
				Platform: "shein",
				Slot:     "main",
			},
		},
	})
	if err != nil {
		t.Fatalf("DispatchTaskGenerationNavigation() error = %v", err)
	}
	if response == nil {
		t.Fatalf("response = nil, want dispatch response")
	}
	if response.PlanMode != "primary_only" {
		t.Fatalf("response.PlanMode = %q, want primary_only", response.PlanMode)
	}
	if response.ExecutedPlan != nil {
		t.Fatalf("response.ExecutedPlan = %+v, want nil for default primary-only dispatch", response.ExecutedPlan)
	}
}

func TestDispatchTaskGenerationNavigationRoutesSessionTarget(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	svc := &service{repo: repo}
	task := &Task{
		ID:        "task-generation-navigation-session-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"shein"}},
		Result: &ListingKitResult{
			TaskID: "task-generation-navigation-session-1",
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
		Target: &GenerationReviewNavigationTarget{
			DispatchKind: "session",
			SessionQuery: &GenerationQueueQuery{
				Platform: "shein",
				Slot:     "main",
			},
		},
	})
	if err != nil {
		t.Fatalf("DispatchTaskGenerationNavigation() error = %v", err)
	}
	if response == nil || response.DispatchKind != "session" || response.ReviewSession == nil {
		t.Fatalf("response = %+v, want session dispatch response", response)
	}
	if len(response.ResourceDescriptors) == 0 {
		t.Fatalf("dispatch response resource descriptors = %+v, want response descriptors", response.ResourceDescriptors)
	}
	if response.PanelUpdate == nil || response.PanelUpdate.DispatchKind != "session" || response.PanelUpdate.ReviewSession == nil || response.PanelUpdate.FocusedTarget == nil {
		t.Fatalf("response = %+v, want normalized panel update", response)
	}
	if len(response.PanelUpdate.FocusedDescriptors) == 0 || response.PanelUpdate.FocusedDescriptors[0].Descriptor == nil || response.PanelUpdate.FocusedDescriptors[0].Descriptor.ResourceKind != "review_session" {
		t.Fatalf("panel update focused descriptors = %+v, want focused session descriptor", response.PanelUpdate.FocusedDescriptors)
	}
}

func TestDispatchTaskGenerationNavigationReturnsPatchOnlyPanelUpdateForSession(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	svc := &service{repo: repo}
	task := &Task{
		ID:        "task-generation-navigation-session-patch-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"shein"}},
		Result: &ListingKitResult{
			TaskID: "task-generation-navigation-session-patch-1",
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
			DispatchKind: "session",
			SessionQuery: &GenerationQueueQuery{
				Platform:     "shein",
				Slot:         "main",
				ResponseMode: "patch_only",
			},
		},
	})
	if err != nil {
		t.Fatalf("DispatchTaskGenerationNavigation() error = %v", err)
	}
	if response == nil || response.PanelUpdate == nil {
		t.Fatalf("response = %+v, want panel update", response)
	}
	if response.PanelUpdate.ReviewSession != nil {
		t.Fatalf("response = %+v, want patch-only panel update", response)
	}
	if response.PanelUpdate.Overview != nil || response.PanelUpdate.QueueSummary != nil || response.PanelUpdate.ReviewSummary != nil {
		t.Fatalf("response = %+v, want patch-only panel update without duplicated summaries", response)
	}
	if response.PanelUpdate.FocusedTarget != nil || response.PanelUpdate.FocusedRenderPreview != nil || response.PanelUpdate.FocusedToolbar != nil {
		t.Fatalf("response = %+v, want patch-only panel update without duplicated focus payload", response)
	}
	if !response.NotModified || !response.PanelUpdate.NoChanges || response.PanelUpdate.ReviewPatch != nil {
		t.Fatalf("response = %+v, want no_changes session panel update", response)
	}
}

func TestDispatchTaskGenerationNavigationRoutesActionTarget(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	svc := &service{repo: repo}
	task := &Task{
		ID:        "task-generation-navigation-action-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"shein"}},
		Result: &ListingKitResult{
			TaskID: "task-generation-navigation-action-1",
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	response, err := svc.DispatchTaskGenerationNavigation(context.Background(), task.ID, &GenerationReviewNavigationDispatchRequest{
		ResponseMode: "patch_only",
		Target: &GenerationReviewNavigationTarget{
			DispatchKind: "action",
			ActionTarget: &AssetGenerationActionTarget{
				ActionKey:       "review_ready_assets",
				InteractionMode: "queue_only",
				QueueQuery: &GenerationQueueQuery{
					Platform: "shein",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("DispatchTaskGenerationNavigation() error = %v", err)
	}
	if response == nil || response.DispatchKind != "action" || response.Action == nil {
		t.Fatalf("response = %+v, want action dispatch response", response)
	}
	if len(response.ResourceDescriptors) == 0 {
		t.Fatalf("dispatch response resource descriptors = %+v, want action response descriptors", response.ResourceDescriptors)
	}
	if response.Action.ResponseMode != "patch_only" {
		t.Fatalf("response = %+v, want patch_only action response mode", response)
	}
	if response.PanelUpdate == nil || response.PanelUpdate.DispatchKind != "action" {
		t.Fatalf("response = %+v, want normalized action panel update", response)
	}
	if response.NotModified || response.PanelUpdate.NoChanges {
		t.Fatalf("response = %+v, want action dispatch to remain modified because workflow feedback exists", response)
	}
	if response.PanelUpdate.ReviewPatch == nil {
		t.Fatalf("response = %+v, want action patch payload", response)
	}
	if response.PanelUpdate.FocusedTarget != nil || response.PanelUpdate.FocusedRenderPreview != nil || response.PanelUpdate.FocusedToolbar != nil {
		t.Fatalf("response = %+v, want patch-only action panel update without duplicated focus payload", response)
	}
}

func TestDispatchTaskGenerationNavigationBuildsChangedDescriptorsForReviewWorkflowAction(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	svc := &service{repo: repo}
	task := &Task{
		ID:        "task-generation-navigation-action-descriptor-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"shein"}},
		Result: &ListingKitResult{
			TaskID: "task-generation-navigation-action-descriptor-1",
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
			DispatchKind: "action",
			ActionTarget: &AssetGenerationActionTarget{
				ActionKey:       "approve_section_review",
				InteractionMode: "review_only",
				QueueQuery: &GenerationQueueQuery{
					Platform:          "shein",
					Slot:              "main",
					PreviewCapability: "detail_preview",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("DispatchTaskGenerationNavigation() error = %v", err)
	}
	if response == nil || response.PanelUpdate == nil || response.PanelUpdate.ReviewPatch == nil {
		t.Fatalf("response = %+v, want review workflow patch", response)
	}
	if len(response.PanelUpdate.ChangedDescriptors) == 0 {
		t.Fatalf("panel update changed descriptors = %+v, want changed descriptors for review workflow action", response.PanelUpdate.ChangedDescriptors)
	}
	if response.PanelUpdate.ChangedDescriptors[0].Descriptor == nil || response.PanelUpdate.ChangedDescriptors[0].Descriptor.ResourceKind != "review_session" {
		t.Fatalf("panel update changed descriptors = %+v, want session descriptor", response.PanelUpdate.ChangedDescriptors)
	}
}

func TestDispatchTaskGenerationNavigationReturnsPatchOnlyPanelUpdateForAction(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	svc := &service{repo: repo}
	task := &Task{
		ID:        "task-generation-navigation-action-patch-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"shein"}},
		Result: &ListingKitResult{
			TaskID: "task-generation-navigation-action-patch-1",
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	response, err := svc.DispatchTaskGenerationNavigation(context.Background(), task.ID, &GenerationReviewNavigationDispatchRequest{
		ResponseMode: "patch_only",
		Target: &GenerationReviewNavigationTarget{
			DispatchKind: "action",
			ActionTarget: &AssetGenerationActionTarget{
				ActionKey:       "review_ready_assets",
				InteractionMode: "queue_only",
				QueueQuery: &GenerationQueueQuery{
					Platform: "shein",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("DispatchTaskGenerationNavigation() error = %v", err)
	}
	if response == nil || response.PanelUpdate == nil {
		t.Fatalf("response = %+v, want panel update", response)
	}
	if response.PanelUpdate.Action != nil {
		t.Fatalf("response = %+v, want patch-only action panel update without full action payload", response)
	}
	if response.PanelUpdate.Overview != nil || response.PanelUpdate.QueueSummary != nil || response.PanelUpdate.ReviewSummary != nil {
		t.Fatalf("response = %+v, want patch-only action panel update without duplicated summaries", response)
	}
	if response.PanelUpdate.FocusedTarget != nil || response.PanelUpdate.FocusedRenderPreview != nil || response.PanelUpdate.FocusedToolbar != nil {
		t.Fatalf("response = %+v, want patch-only action panel update without duplicated focus payload", response)
	}
	if response.NotModified || response.PanelUpdate.NoChanges || response.PanelUpdate.ReviewPatch == nil {
		t.Fatalf("response = %+v, want action panel update with workflow patch", response)
	}
}

func TestDispatchTaskGenerationNavigationReturnsNoChangesForPreviewNotModified(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	svc := &service{repo: repo}
	task := &Task{
		ID:        "task-generation-navigation-preview-not-modified-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"shein"}},
		Result: &ListingKitResult{
			TaskID: "task-generation-navigation-preview-not-modified-1",
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
		t.Fatalf("GetTaskGenerationReviewPreview() error = %v", err)
	}

	response, err := svc.DispatchTaskGenerationNavigation(context.Background(), task.ID, &GenerationReviewNavigationDispatchRequest{
		ResponseMode: "patch_only",
		Target: &GenerationReviewNavigationTarget{
			DispatchKind: "preview",
			PreviewQuery: &GenerationQueueQuery{
				Platform:   "shein",
				Slot:       "main",
				DeltaToken: first.DeltaToken,
			},
		},
	})
	if err != nil {
		t.Fatalf("DispatchTaskGenerationNavigation() error = %v", err)
	}
	if response == nil || !response.NotModified || response.PanelUpdate == nil || !response.PanelUpdate.NoChanges {
		t.Fatalf("response = %+v, want preview not_modified no_changes response", response)
	}
	if len(response.PanelUpdate.FocusedDescriptors) != 0 || len(response.PanelUpdate.ChangedDescriptors) != 0 {
		t.Fatalf("panel update descriptors = %+v/%+v, want no descriptors for no_changes preview", response.PanelUpdate.FocusedDescriptors, response.PanelUpdate.ChangedDescriptors)
	}
	if response.PanelUpdate.ReviewPreview != nil || response.PanelUpdate.FocusedRenderPreview != nil {
		t.Fatalf("response = %+v, want preview payload omitted on no_changes", response)
	}
}

func TestDispatchTaskGenerationNavigationReturnsNoChangesForQueueNotModified(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	svc := &service{repo: repo}
	task := &Task{
		ID:        "task-generation-navigation-queue-not-modified-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"shein"}},
		Result: &ListingKitResult{
			TaskID: "task-generation-navigation-queue-not-modified-1",
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
		t.Fatalf("GetTaskGenerationQueue() error = %v", err)
	}

	response, err := svc.DispatchTaskGenerationNavigation(context.Background(), task.ID, &GenerationReviewNavigationDispatchRequest{
		ResponseMode: "patch_only",
		Target: &GenerationReviewNavigationTarget{
			DispatchKind: "queue",
			QueueQuery: &GenerationQueueQuery{
				Platform:   "shein",
				DeltaToken: first.DeltaToken,
			},
		},
	})
	if err != nil {
		t.Fatalf("DispatchTaskGenerationNavigation() error = %v", err)
	}
	if response == nil || !response.NotModified || response.PanelUpdate == nil || !response.PanelUpdate.NoChanges {
		t.Fatalf("response = %+v, want queue not_modified no_changes response", response)
	}
	if response.Queue == nil || !response.Queue.NotModified || response.Queue.DeltaToken != first.DeltaToken {
		t.Fatalf("response = %+v, want queue not_modified payload", response)
	}
	if response.PanelUpdate.QueueSummary != nil {
		t.Fatalf("response = %+v, want queue summary omitted on no_changes", response)
	}
}

func TestDispatchTaskGenerationNavigationExecutesDispatchPlanForSessionTarget(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	svc := &service{repo: repo}
	task := &Task{
		ID:        "task-generation-navigation-session-plan-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"shein"}},
		Result: &ListingKitResult{
			TaskID: "task-generation-navigation-session-plan-1",
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
		PlanMode: "execute_plan",
		Target: &GenerationReviewNavigationTarget{
			DispatchKind: "session",
			SessionQuery: &GenerationQueueQuery{
				Platform:          "shein",
				Slot:              "main",
				PreviewCapability: "detail_preview",
			},
		},
	})
	if err != nil {
		t.Fatalf("DispatchTaskGenerationNavigation() error = %v", err)
	}
	if response == nil || response.ExecutedPlan == nil {
		t.Fatalf("response = %+v, want executed plan", response)
	}
	if response.PlanMode != "execute_plan" || response.ExecutedPlan.Strategy != "fanout_read" || response.ExecutedPlan.StopReason != "" {
		t.Fatalf("response = %+v, want fanout executed plan", response)
	}
	if len(response.ExecutedPlan.Steps) < 3 {
		t.Fatalf("executed plan = %+v, want queue/session/preview steps", response.ExecutedPlan)
	}
	if response.ExecutedPlan.Partial || response.ExecutedPlan.CompletedSteps < 3 || response.ExecutedPlan.FailedSteps != 0 {
		t.Fatalf("executed plan = %+v, want completed fanout execution stats", response.ExecutedPlan)
	}
	if response.ExecutedPlan.Strategy != "fanout_read" || response.ExecutedPlan.CompletedSteps < 3 {
		t.Fatalf("executed plan = %+v, want stable fanout execution", response.ExecutedPlan)
	}
	if response.ReviewSession == nil || response.Queue == nil || response.ReviewPreview == nil {
		t.Fatalf("response = %+v, want plan to auto-populate session/queue/preview", response)
	}
	if response.PanelUpdate == nil || response.PanelUpdate.FocusedToolbar == nil || response.PanelUpdate.QueueSummary == nil {
		t.Fatalf("response = %+v, want panel update merged from executed plan", response)
	}
	if response.FocusedSourceKind != "preview" || response.FocusedSourceStep < 0 || response.FocusedViaFallback {
		t.Fatalf("response = %+v, want preview-focused winner metadata", response)
	}
	if response.FocusedResolution == nil || response.FocusedResolution.SourceKind != "preview" || response.FocusedResolution.SourceStep != response.FocusedSourceStep {
		t.Fatalf("response focused resolution = %+v, want focused resolution metadata", response.FocusedResolution)
	}
	if response.PanelUpdate.FocusedSourceKind != response.FocusedSourceKind || response.PanelUpdate.FocusedSourceStep != response.FocusedSourceStep {
		t.Fatalf("panel update = %+v, want focused source metadata propagated", response.PanelUpdate)
	}
	if response.PanelUpdate.FocusedResolution == nil || response.PanelUpdate.FocusedResolution.SourceKind != response.FocusedSourceKind {
		t.Fatalf("panel update focused resolution = %+v, want propagated focused resolution", response.PanelUpdate.FocusedResolution)
	}
	if len(response.PanelUpdate.FocusedDescriptors) == 0 || response.PanelUpdate.FocusedDescriptors[0].SourceKind != "preview" || response.PanelUpdate.FocusedDescriptors[0].SourceStep != response.FocusedSourceStep {
		t.Fatalf("focused descriptors = %+v, want focused source metadata", response.PanelUpdate.FocusedDescriptors)
	}
	if response.PrimaryRecoveryDescriptor != nil || len(response.RecommendedRecoveryDescriptors) != 0 {
		t.Fatalf("response recovery descriptors = %+v/%+v, want no recovery recommendation on healthy winner path", response.PrimaryRecoveryDescriptor, response.RecommendedRecoveryDescriptors)
	}
	if response.PanelUpdate.PrimaryRecoveryDescriptor != nil || len(response.PanelUpdate.RecommendedRecoveryDescriptors) != 0 {
		t.Fatalf("panel update recovery descriptors = %+v/%+v, want no panel recovery recommendation on healthy winner path", response.PanelUpdate.PrimaryRecoveryDescriptor, response.PanelUpdate.RecommendedRecoveryDescriptors)
	}
}

func TestDispatchTaskGenerationNavigationExecutesDispatchPlanForActionTarget(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	svc := &service{repo: repo}
	task := &Task{
		ID:        "task-generation-navigation-action-plan-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"shein"}},
		Result: &ListingKitResult{
			TaskID: "task-generation-navigation-action-plan-1",
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
		PlanMode:     "execute_plan",
		Target: &GenerationReviewNavigationTarget{
			DispatchKind: "action",
			ActionTarget: &AssetGenerationActionTarget{
				ActionKey:       "approve_section_review",
				InteractionMode: "review_only",
				QueueQuery: &GenerationQueueQuery{
					Platform:          "shein",
					Slot:              "main",
					PreviewCapability: "detail_preview",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("DispatchTaskGenerationNavigation() error = %v", err)
	}
	if response == nil || response.Action == nil || response.ExecutedPlan == nil {
		t.Fatalf("response = %+v, want action plus executed plan", response)
	}
	if response.PlanMode != "execute_plan" || response.ExecutedPlan.Strategy != "mutation_then_refresh" || response.ExecutedPlan.StopReason != "" {
		t.Fatalf("response = %+v, want mutation_then_refresh executed plan", response)
	}
	if len(response.ExecutedPlan.Steps) == 0 || response.ExecutedPlan.Steps[0].CachePreference != "revalidate" || !response.ExecutedPlan.Steps[0].RequiresRevalidate {
		t.Fatalf("executed plan = %+v, want revalidate follow-up steps", response.ExecutedPlan)
	}
	if response.ExecutedPlan.Partial || response.ExecutedPlan.FailedSteps != 0 {
		t.Fatalf("executed plan = %+v, want successful mutation follow-up execution", response.ExecutedPlan)
	}
	if response.Queue == nil || response.ReviewSession == nil || response.ReviewPreview == nil {
		t.Fatalf("response = %+v, want action plan to auto-populate follow-up reads", response)
	}
	if response.FocusedSourceKind != "preview" || response.FocusedViaFallback {
		t.Fatalf("response = %+v, want preview winner for action follow-up", response)
	}
	if len(response.ResourceDescriptors) == 0 {
		t.Fatalf("response descriptors = %+v, want resource descriptors", response.ResourceDescriptors)
	}
	foundQueueRecovery := false
	for _, item := range response.ResourceDescriptors {
		if item.Role == "queue_item" {
			if item.RecoveryHint != "" {
				if item.RecoveryTarget == nil || item.RecoveryDispatchPlan == nil {
					t.Fatalf("queue descriptor = %+v, want recovery contract when recovery_hint is present", item)
				}
			}
			foundQueueRecovery = true
			break
		}
	}
	if !foundQueueRecovery {
		t.Fatalf("response descriptors = %+v, want queue descriptor recovery metadata path", response.ResourceDescriptors)
	}
	if response.PrimaryRecoveryDescriptor != nil || len(response.RecommendedRecoveryDescriptors) != 0 {
		t.Fatalf("response recovery descriptors = %+v/%+v, want no top-level recovery recommendation when no recovery hints are present", response.PrimaryRecoveryDescriptor, response.RecommendedRecoveryDescriptors)
	}
}

func TestExecuteGenerationNavigationDispatchPlanDeduplicatesDuplicateSteps(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	svc := &service{repo: repo}
	task := &Task{
		ID:        "task-generation-navigation-plan-dedupe-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"shein"}},
		Result: &ListingKitResult{
			TaskID: "task-generation-navigation-plan-dedupe-1",
			Shein: &SheinPackage{ImageBundle: &common.PublishImageBundle{
				Platform: "shein",
				Main: &common.BundleSlot{
					Key:        "main",
					AssetID:    "asset-preview-1",
					StateLabel: "ready",
				},
			}},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	target := applyIdentityToNavigationTarget(&GenerationReviewNavigationTarget{
		DispatchKind: "queue",
		QueueQuery: &GenerationQueueQuery{
			Platform: "shein",
			Slot:     "main",
		},
	})
	target.Descriptor.DispatchPlan = &GenerationNavigationDispatchPlan{
		Strategy: "fanout_read",
		Steps: []GenerationNavigationDispatchStep{
			{Kind: "queue", ResponseMode: "full", CachePreference: "revalidate", Query: cloneGenerationQueueQuery(target.QueueQuery)},
			{Kind: "queue", ResponseMode: "full", CachePreference: "revalidate", Query: cloneGenerationQueueQuery(target.QueueQuery)},
		},
	}

	execution, err := svc.executeGenerationNavigationDispatchPlan(context.Background(), task.ID, target, "full")
	if err != nil {
		t.Fatalf("executeGenerationNavigationDispatchPlan() error = %v", err)
	}
	if execution == nil || len(execution.Steps) != 2 {
		t.Fatalf("execution = %+v, want two execution steps", execution)
	}
	if execution.DedupedSteps != 1 || execution.CompletedSteps != 1 || execution.FailedSteps != 0 {
		t.Fatalf("execution = %+v, want one deduped and one completed step", execution)
	}
	if execution.Steps[0].Status == execution.Steps[1].Status {
		t.Fatalf("execution steps = %+v, want one completed and one deduplicated step", execution.Steps)
	}
	for _, step := range execution.Steps {
		if step.Status == "deduplicated" && (step.Retryable || step.RetryHint != "no_retry") {
			t.Fatalf("execution step = %+v, want deduplicated step to be non-retryable", step)
		}
	}
}

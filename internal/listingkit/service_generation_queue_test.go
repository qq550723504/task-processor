package listingkit

import (
	"context"
	"errors"
	"testing"
	"time"

	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
	assetrepo "task-processor/internal/asset/repository"
	"task-processor/internal/listingkit/reviewstore"
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

func TestGetTaskGenerationQueueBuildsEmptyQueueFinalResponseShape(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	assetRepository := assetrepo.NewMemRepository()
	svc := &service{
		repo:      repo,
		assetRepo: assetRepository,
	}

	updatedAt := time.Date(2026, 5, 30, 11, 0, 0, 0, time.UTC)
	task := &Task{
		ID:        "task-generation-queue-empty-final-1",
		Status:    TaskStatusCompleted,
		CreatedAt: updatedAt,
		UpdatedAt: updatedAt,
		Request:   &GenerateRequest{Platforms: []string{"shein"}},
		Result:    &ListingKitResult{TaskID: "task-generation-queue-empty-final-1"},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	page, err := svc.GetTaskGenerationQueue(context.Background(), task.ID, &GenerationQueueQuery{
		Platform: "shein",
		Page:     2,
		PageSize: 5,
	})
	if err != nil {
		t.Fatalf("GetTaskGenerationQueue() error = %v", err)
	}
	if page == nil {
		t.Fatal("page = nil, want empty queue response")
	}
	if page.TaskID != task.ID || page.Page != 2 || page.PageSize != 5 || page.Total != 0 || !page.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("page = %+v, want empty queue response metadata preserved", page)
	}
	if page.DeltaToken == "" {
		t.Fatalf("page = %+v, want final delta token on empty queue response", page)
	}
	if page.NotModified {
		t.Fatalf("page = %+v, want full empty queue response instead of not_modified", page)
	}
	if len(page.Items) != 0 {
		t.Fatalf("page.Items = %+v, want no queue items", page.Items)
	}
	if page.Summary == nil || page.Summary.TotalItems != 0 || page.Summary.ReadyItems != 0 || page.Summary.ReviewPendingSections != 0 {
		t.Fatalf("page.Summary = %+v, want zeroed empty summary on final response", page.Summary)
	}
	if page.Conditional == nil || page.Conditional.DeltaToken != page.DeltaToken || page.Conditional.NotModified {
		t.Fatalf("page.Conditional = %+v, want final conditional decoration", page.Conditional)
	}
}

func TestGetTaskGenerationQueueFinalResponseRetainsReviewSummaryAndDeltaSensitivity(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	assetRepository := assetrepo.NewMemRepository()
	svc := &service{
		repo:       repo,
		assetRepo:  assetRepository,
		reviewRepo: reviewstore.NewMemRepository(),
	}

	updatedAt := time.Date(2026, 5, 30, 11, 5, 0, 0, time.UTC)
	task := &Task{
		ID:        "task-generation-queue-review-final-1",
		Status:    TaskStatusCompleted,
		CreatedAt: updatedAt,
		UpdatedAt: updatedAt,
		Request:   &GenerateRequest{Platforms: []string{"shein"}},
		Result: &ListingKitResult{
			TaskID: "task-generation-queue-review-final-1",
			AssetRenderPreviews: []AssetRenderPreview{{
				AssetID:         "asset-reviewed-final-1",
				AssetRevision:   "asset-rev-final-1",
				PreviewRevision: "preview-rev-final-1",
				TaskRevision:    "task-rev-final-1",
				PreviewFormat:   "svg",
				PreviewSVG:      "<svg/>",
				VisualMode:      "selling_point",
				LayerTypes:      []string{"detail", "text"},
			}},
			Shein: &SheinPackage{ImageBundle: &common.PublishImageBundle{
				Platform: "shein",
				Main: &common.BundleSlot{
					Key:             "main",
					Purpose:         "main",
					RecipeID:        "shein-main-model",
					TemplateLabel:   "SHEIN Main",
					StateLabel:      "ready",
					SatisfiedBy:     "exact_asset",
					ExecutionStatus: "ready",
					AssetID:         "asset-reviewed-final-1",
				},
			}},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if _, err := svc.ExecuteTaskGenerationAction(context.Background(), task.ID, &ExecuteGenerationActionRequest{
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
	}); err != nil {
		t.Fatalf("ExecuteTaskGenerationAction() error = %v", err)
	}

	query := &GenerationQueueQuery{Platform: "shein"}
	page, err := svc.GetTaskGenerationQueue(context.Background(), task.ID, query)
	if err != nil {
		t.Fatalf("GetTaskGenerationQueue() error = %v", err)
	}
	if page == nil {
		t.Fatal("page = nil, want final queue response with review summary")
	}
	if page.Summary == nil || page.Summary.ApprovedSections != 1 || page.Summary.DeferredSections != 0 || page.Summary.ReviewPendingSections < 1 {
		t.Fatalf("page.Summary = %+v, want final response to retain review summary fields", page.Summary)
	}
	if page.DeltaToken == "" {
		t.Fatalf("page = %+v, want delta token on final response", page)
	}
	if page.Conditional == nil || page.Conditional.DeltaToken != page.DeltaToken {
		t.Fatalf("page.Conditional = %+v, want final conditional decoration using delta token", page.Conditional)
	}

	withoutReviewSummary := *page
	withoutReviewSummary.Summary = &GenerationWorkQueueSummary{
		TotalItems:       page.Summary.TotalItems,
		ReadyItems:       page.Summary.ReadyItems,
		PreviewableItems: page.Summary.PreviewableItems,
	}
	if page.DeltaToken == buildGenerationQueueDeltaToken(&withoutReviewSummary, query) {
		t.Fatalf("page.DeltaToken = %q, want final delta token sensitive to review summary", page.DeltaToken)
	}
}

func TestGetTaskGenerationQueueFinalResponseIncludesQueueResourceDescriptors(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	assetRepository := assetrepo.NewMemRepository()
	svc := &service{
		repo:      repo,
		assetRepo: assetRepository,
	}

	updatedAt := time.Date(2026, 5, 30, 11, 10, 0, 0, time.UTC)
	task := &Task{
		ID:        "task-generation-queue-descriptors-final-1",
		Status:    TaskStatusCompleted,
		CreatedAt: updatedAt,
		UpdatedAt: updatedAt,
		Request:   &GenerateRequest{Platforms: []string{"shein"}},
		Result: &ListingKitResult{
			TaskID: "task-generation-queue-descriptors-final-1",
			AssetRenderPreviews: []AssetRenderPreview{{
				AssetID:         "asset-descriptor-final-1",
				AssetRevision:   "asset-rev-descriptor-final-1",
				PreviewRevision: "preview-rev-descriptor-final-1",
				TaskRevision:    "task-rev-descriptor-final-1",
				PreviewFormat:   "svg",
				PreviewSVG:      "<svg/>",
				VisualMode:      "selling_point",
				LayerTypes:      []string{"detail", "text"},
			}},
			Shein: &SheinPackage{ImageBundle: &common.PublishImageBundle{
				Platform: "shein",
				Main: &common.BundleSlot{
					Key:             "main",
					Purpose:         "main",
					RecipeID:        "shein-main-model",
					TemplateLabel:   "SHEIN Main",
					StateLabel:      "ready",
					SatisfiedBy:     "exact_asset",
					ExecutionStatus: "ready",
					AssetID:         "asset-descriptor-final-1",
				},
			}},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	page, err := svc.GetTaskGenerationQueue(context.Background(), task.ID, &GenerationQueueQuery{Platform: "shein"})
	if err != nil {
		t.Fatalf("GetTaskGenerationQueue() error = %v", err)
	}
	if page == nil || len(page.Items) != 1 {
		t.Fatalf("page = %+v, want single queue item response", page)
	}
	if len(page.ResourceDescriptors) == 0 {
		t.Fatalf("page.ResourceDescriptors = %+v, want queue response descriptors from final conditional decoration", page.ResourceDescriptors)
	}
	descriptor := page.ResourceDescriptors[0]
	if descriptor.Role != "queue_item" ||
		descriptor.Platform != page.Items[0].Platform ||
		descriptor.Slot != page.Items[0].Slot ||
		descriptor.Capability != page.Items[0].PreviewCapabilities[0] {
		t.Fatalf("descriptor = %+v, item = %+v, want descriptor to match final queue item", descriptor, page.Items[0])
	}
	if descriptor.Descriptor == nil || descriptor.Descriptor.ResourceKind != "generation_queue" || descriptor.Descriptor.RefreshScope != "collection_read" {
		t.Fatalf("descriptor metadata = %+v, want queue response descriptor contract", descriptor.Descriptor)
	}
}

func TestTaskGenerationQueueReadPageDeferredOnlyReviewSummaryChangeAffectsDeltaToken(t *testing.T) {
	t.Parallel()

	query := &GenerationQueueQuery{
		Platform:  "shein",
		Page:      1,
		PageSize:  1,
		SortBy:    "platform",
		SortOrder: "asc",
	}
	base := &GenerationQueuePage{
		TaskID:    "task-generation-queue-deferred-delta-1",
		Page:      1,
		PageSize:  1,
		Total:     2,
		UpdatedAt: time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC),
		Summary: &GenerationWorkQueueSummary{
			TotalItems:            2,
			ReadyItems:            2,
			PreviewableItems:      2,
			ApprovedSections:      0,
			DeferredSections:      0,
			ReviewPendingSections: 2,
		},
		Items: []GenerationWorkQueueItem{{
			TaskID:                 "task-generation-queue-deferred-delta-1",
			Platform:               "amazon",
			Slot:                   "main",
			State:                  "ready",
			RenderPreviewAvailable: true,
		}},
	}
	withDeferred := *base
	withDeferred.Summary = &GenerationWorkQueueSummary{
		TotalItems:            base.Summary.TotalItems,
		ReadyItems:            base.Summary.ReadyItems,
		PreviewableItems:      base.Summary.PreviewableItems,
		ApprovedSections:      base.Summary.ApprovedSections,
		DeferredSections:      1,
		ReviewPendingSections: base.Summary.ReviewPendingSections,
	}

	baseToken := buildGenerationQueueDeltaToken(base, query)
	deferredToken := buildGenerationQueueDeltaToken(&withDeferred, query)
	if baseToken == "" || deferredToken == "" {
		t.Fatalf("tokens = %q / %q, want non-empty queue delta tokens", baseToken, deferredToken)
	}
	if baseToken == deferredToken {
		t.Fatalf("tokens = %q / %q, want deferred-only summary change to alter queue delta token", baseToken, deferredToken)
	}
}

func TestGetTaskGenerationQueueDeferredOnlyReviewSummaryChangeInvalidatesOldToken(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	assetRepository := assetrepo.NewMemRepository()
	svc := &service{
		repo:       repo,
		assetRepo:  assetRepository,
		reviewRepo: reviewstore.NewMemRepository(),
	}

	now := time.Date(2026, 5, 30, 12, 5, 0, 0, time.UTC)
	task := &Task{
		ID:        "task-generation-queue-deferred-not-modified-1",
		Status:    TaskStatusCompleted,
		CreatedAt: now,
		UpdatedAt: now,
		Request:   &GenerateRequest{Platforms: []string{"amazon", "shein"}},
		Result: &ListingKitResult{
			TaskID: "task-generation-queue-deferred-not-modified-1",
			AssetRenderPreviews: []AssetRenderPreview{
				{
					AssetID:         "asset-amazon-main-1",
					AssetRevision:   "asset-rev-amazon-1",
					PreviewRevision: "preview-rev-amazon-1",
					TaskRevision:    "task-rev-shared-1",
					PreviewFormat:   "svg",
					PreviewSVG:      "<svg/>",
					VisualMode:      "selling_point",
					LayerTypes:      []string{"detail", "text"},
				},
				{
					AssetID:         "asset-shein-main-1",
					AssetRevision:   "asset-rev-shein-1",
					PreviewRevision: "preview-rev-shein-1",
					TaskRevision:    "task-rev-shared-1",
					PreviewFormat:   "svg",
					PreviewSVG:      "<svg/>",
					VisualMode:      "selling_point",
					LayerTypes:      []string{"detail", "text"},
				},
			},
			Amazon: &AmazonPackage{
				ImageBundle: &common.PublishImageBundle{
					Platform: "amazon",
					Main: &common.BundleSlot{
						Key:             "main",
						Purpose:         "main",
						RecipeID:        "amazon-main-white-bg",
						TemplateLabel:   "Amazon Main",
						StateLabel:      "ready",
						SatisfiedBy:     "exact_asset",
						ExecutionStatus: "ready",
						AssetID:         "asset-amazon-main-1",
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
						TemplateLabel:   "SHEIN Main",
						StateLabel:      "ready",
						SatisfiedBy:     "exact_asset",
						ExecutionStatus: "ready",
						AssetID:         "asset-shein-main-1",
					},
				},
			},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	query := &GenerationQueueQuery{
		Page:      1,
		PageSize:  1,
		SortBy:    "platform",
		SortOrder: "asc",
	}
	first, err := svc.GetTaskGenerationQueue(context.Background(), task.ID, query)
	if err != nil {
		t.Fatalf("GetTaskGenerationQueue() first call error = %v", err)
	}
	if first == nil || first.DeltaToken == "" || len(first.Items) != 1 {
		t.Fatalf("first = %+v, want first paged queue response with delta token", first)
	}
	if first.Items[0].Platform != "amazon" {
		t.Fatalf("first.Items = %+v, want amazon item on first page before review change", first.Items)
	}

	if _, err := svc.ExecuteTaskGenerationAction(context.Background(), task.ID, &ExecuteGenerationActionRequest{
		ActionKey: "defer_section_review",
		Target: &AssetGenerationActionTarget{
			ActionKey:       "defer_section_review",
			InteractionMode: "review_only",
			QueueQuery: &GenerationQueueQuery{
				Platform:          "shein",
				Slot:              "main",
				PreviewCapability: "detail_preview",
			},
		},
	}); err != nil {
		t.Fatalf("ExecuteTaskGenerationAction() error = %v", err)
	}

	second, err := svc.GetTaskGenerationQueue(context.Background(), task.ID, &GenerationQueueQuery{
		Page:       1,
		PageSize:   1,
		SortBy:     "platform",
		SortOrder:  "asc",
		DeltaToken: first.DeltaToken,
	})
	if err != nil {
		t.Fatalf("GetTaskGenerationQueue() second call error = %v", err)
	}
	if second == nil {
		t.Fatal("second = nil, want updated queue response after deferred-only summary change")
	}
	if second.NotModified {
		t.Fatalf("second = %+v, want old token invalidated by deferred-only summary change", second)
	}
	if second.DeltaToken == "" || second.DeltaToken == first.DeltaToken {
		t.Fatalf("second.DeltaToken = %q, first = %q, want changed final response token", second.DeltaToken, first.DeltaToken)
	}
	if second.Summary == nil || second.Summary.DeferredSections != 1 {
		t.Fatalf("second.Summary = %+v, want deferred review summary reflected in final response", second.Summary)
	}
	if len(second.Items) != 1 || second.Items[0].Platform != "amazon" {
		t.Fatalf("second.Items = %+v, want same paged item while deferred-only summary changed elsewhere", second.Items)
	}
}

func TestTaskGenerationQueueReadSnapshotRunUsesSingleCurrentSnapshot(t *testing.T) {
	t.Parallel()

	const taskID = "task-generation-queue-snapshot-1"
	repo := &sequencedTaskSnapshotsRepo{
		snapshots: []*Task{
			{
				ID: taskID,
				Result: &ListingKitResult{
					TaskID: taskID,
					Shein: &SheinPackage{ImageBundle: &common.PublishImageBundle{
						Platform: "shein",
						Main: &common.BundleSlot{
							Key:           "main",
							AssetID:       "asset-first",
							StateLabel:    "ready",
							TemplateLabel: "First Snapshot",
						},
					}},
				},
			},
			{
				ID: taskID,
				Result: &ListingKitResult{
					TaskID: taskID,
					Shein: &SheinPackage{ImageBundle: &common.PublishImageBundle{
						Platform: "shein",
						Main: &common.BundleSlot{
							Key:           "main",
							AssetID:       "asset-second",
							StateLabel:    "ready",
							TemplateLabel: "Second Snapshot",
						},
					}},
				},
			},
		},
	}
	svc := &taskGenerationService{
		repo: repo,
		listAssetGenerationTasks: func(context.Context, string) ([]assetgeneration.Task, error) {
			return nil, nil
		},
		listGenerationReviews: func(context.Context, string) ([]GenerationReviewRecord, error) {
			return nil, nil
		},
	}

	snapshot, err := buildTaskGenerationQueueReadSnapshotPhase(svc).run(context.Background(), taskID)
	if err != nil {
		t.Fatalf("taskGenerationQueueReadSnapshotPhase.run() error = %v", err)
	}
	if snapshot == nil || snapshot.task == nil || snapshot.result == nil || snapshot.queue == nil {
		t.Fatalf("snapshot = %+v, want hydrated task/result/queue snapshot", snapshot)
	}
	if repo.getCalls != 1 {
		t.Fatalf("repo.getCalls = %d, want single current snapshot read", repo.getCalls)
	}
	if snapshot.task.ID != taskID {
		t.Fatalf("snapshot.task = %+v, want task %q", snapshot.task, taskID)
	}
	if len(snapshot.queue.Items) != 1 || snapshot.queue.Items[0].AssetID != "asset-first" {
		t.Fatalf("queue = %+v, want queue from first snapshot", snapshot.queue)
	}
	if snapshot.result.AssetGenerationQueue == nil || len(snapshot.result.AssetGenerationQueue.Items) != 1 || snapshot.result.AssetGenerationQueue.Items[0].AssetID != "asset-first" {
		t.Fatalf("result queue = %+v, want result from same first snapshot", snapshot.result.AssetGenerationQueue)
	}
}

func TestTaskGenerationQueueReadSnapshotRunPropagatesLoadErrors(t *testing.T) {
	t.Parallel()

	taskListErr := errors.New("snapshot load failed")
	reviewListErr := errors.New("review snapshot load failed")
	tests := []struct {
		name    string
		service *taskGenerationService
		taskID  string
		wantErr error
	}{
		{
			name:    "repo get task",
			service: &taskGenerationService{repo: &stubGenerationRepo{}},
			taskID:  "task-generation-queue-snapshot-missing-1",
			wantErr: ErrTaskNotFound,
		},
		{
			name: "list asset generation tasks",
			service: &taskGenerationService{
				repo: &stubGenerationRepo{task: &Task{
					ID:     "task-generation-queue-snapshot-error-1",
					Result: &ListingKitResult{TaskID: "task-generation-queue-snapshot-error-1"},
				}},
				listAssetGenerationTasks: func(context.Context, string) ([]assetgeneration.Task, error) {
					return nil, taskListErr
				},
				listGenerationReviews: func(context.Context, string) ([]GenerationReviewRecord, error) {
					return nil, nil
				},
			},
			taskID:  "task-generation-queue-snapshot-error-1",
			wantErr: taskListErr,
		},
		{
			name: "list generation reviews",
			service: &taskGenerationService{
				repo: &stubGenerationRepo{task: &Task{
					ID:     "task-generation-queue-snapshot-error-2",
					Result: &ListingKitResult{TaskID: "task-generation-queue-snapshot-error-2"},
				}},
				listAssetGenerationTasks: func(context.Context, string) ([]assetgeneration.Task, error) {
					return nil, nil
				},
				listGenerationReviews: func(context.Context, string) ([]GenerationReviewRecord, error) {
					return nil, reviewListErr
				},
			},
			taskID:  "task-generation-queue-snapshot-error-2",
			wantErr: reviewListErr,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildTaskGenerationQueueReadSnapshotPhase(tc.service).run(context.Background(), tc.taskID)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("taskGenerationQueueReadSnapshotPhase.run() error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestTaskGenerationQueueReadSnapshotRunUsesSingleReviewedResultHandoff(t *testing.T) {
	t.Parallel()

	const taskID = "task-generation-queue-snapshot-handoff-1"
	taskCalls := 0
	reviewCalls := 0
	svc := &taskGenerationService{
		repo: &stubGenerationRepo{task: &Task{
			ID: taskID,
			Result: &ListingKitResult{
				TaskID: taskID,
				Shein: &SheinPackage{ImageBundle: &common.PublishImageBundle{
					Platform: "shein",
					Main: &common.BundleSlot{
						Key:             "main",
						Purpose:         "main",
						AssetID:         "asset-reviewed",
						RecipeID:        "shein-main-model",
						TemplateLabel:   "SHEIN Main",
						StateLabel:      "ready",
						SatisfiedBy:     "exact_asset",
						ExecutionStatus: "ready",
					},
				}},
			},
		}},
		listAssetGenerationTasks: func(context.Context, string) ([]assetgeneration.Task, error) {
			taskCalls++
			if taskCalls == 1 {
				return []assetgeneration.Task{{
					ID:              "shein:shein-main-model",
					Platform:        "shein",
					RecipeID:        "shein-main-model",
					Slot:            "main",
					Purpose:         "main",
					AssetKind:       asset.KindModelImage,
					TemplateLabel:   "SHEIN Main",
					RenderProfile:   "shein_model_editorial",
					ExecutionStatus: "completed",
					CanExecute:      true,
				}}, nil
			}
			return []assetgeneration.Task{{
				ID:              "shein:shein-gallery-model",
				Platform:        "shein",
				RecipeID:        "shein-gallery-model",
				Slot:            "gallery",
				Purpose:         "detail",
				AssetKind:       asset.KindModelImage,
				TemplateLabel:   "SHEIN Gallery",
				RenderProfile:   "shein_model_detail",
				ExecutionStatus: "completed",
				CanExecute:      true,
			}}, nil
		},
		listGenerationReviews: func(context.Context, string) ([]GenerationReviewRecord, error) {
			reviewCalls++
			if reviewCalls == 1 {
				return []GenerationReviewRecord{{
					Platform:   "shein",
					Slot:       "main",
					Capability: "detail_preview",
					Decision:   GenerationReviewDecisionApprove,
					AssetID:    "asset-reviewed",
				}}, nil
			}
			return []GenerationReviewRecord{{
				Platform:   "shein",
				Slot:       "gallery",
				Capability: "detail_preview",
				Decision:   GenerationReviewDecisionDefer,
				AssetID:    "asset-reviewed-2",
			}}, nil
		},
	}

	snapshot, err := buildTaskGenerationQueueReadSnapshotPhase(svc).run(context.Background(), taskID)
	if err != nil {
		t.Fatalf("taskGenerationQueueReadSnapshotPhase.run() error = %v", err)
	}
	if snapshot == nil || snapshot.result == nil || snapshot.queue == nil {
		t.Fatalf("snapshot = %+v, want reviewed result + queue handoff", snapshot)
	}
	if taskCalls != 1 {
		t.Fatalf("listAssetGenerationTasks calls = %d, want 1", taskCalls)
	}
	if reviewCalls != 1 {
		t.Fatalf("listGenerationReviews calls = %d, want 1", reviewCalls)
	}
	if snapshot.queue != snapshot.result.AssetGenerationQueue {
		t.Fatalf("queue pointer = %p, result queue pointer = %p, want same handoff", snapshot.queue, snapshot.result.AssetGenerationQueue)
	}
	if len(snapshot.queue.Items) != 1 || snapshot.queue.Items[0].Slot != "main" {
		t.Fatalf("queue = %+v, want first handoff queue only", snapshot.queue)
	}
}

func TestTaskGenerationQueueReadPageRunBuildsEmptyQueueResponseShape(t *testing.T) {
	t.Parallel()

	updatedAt := time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC)
	page := buildTaskGenerationQueueReadPagePhase().run(&taskGenerationQueueReadSnapshot{
		task: &Task{
			ID:        "task-generation-queue-read-page-empty-1",
			UpdatedAt: updatedAt,
		},
	}, &GenerationQueueQuery{
		Page:     3,
		PageSize: 7,
	})
	if page == nil {
		t.Fatal("page = nil, want empty queue page")
	}
	if page.TaskID != "task-generation-queue-read-page-empty-1" || page.Page != 3 || page.PageSize != 7 || page.Total != 0 || !page.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("page = %+v, want empty queue page metadata preserved", page)
	}
	if len(page.Items) != 0 {
		t.Fatalf("page.Items = %+v, want no queue items", page.Items)
	}
	if page.Summary == nil {
		t.Fatal("page.Summary = nil, want empty summary shape")
	}
	if page.Summary.TotalItems != 0 || page.Summary.ReadyItems != 0 || page.Summary.RetryableItems != 0 {
		t.Fatalf("page.Summary = %+v, want zeroed empty summary", page.Summary)
	}
}

func TestTaskGenerationQueueReadPageRunAppliesFilteringSortingAndPaging(t *testing.T) {
	t.Parallel()

	page := buildTaskGenerationQueueReadPagePhase().run(&taskGenerationQueueReadSnapshot{
		task: &Task{
			ID:        "task-generation-queue-read-page-list-1",
			UpdatedAt: time.Date(2026, 5, 30, 10, 5, 0, 0, time.UTC),
		},
		queue: &GenerationWorkQueue{
			Items: []GenerationWorkQueueItem{
				{TaskID: "task-generation-queue-read-page-list-1", Platform: "shein", Slot: "main", State: "stubbed", TemplateLabel: "B Template"},
				{TaskID: "task-generation-queue-read-page-list-1", Platform: "amazon", Slot: "main", State: "ready", TemplateLabel: "A Template"},
				{TaskID: "task-generation-queue-read-page-list-1", Platform: "amazon", Slot: "gallery", State: "stubbed", TemplateLabel: "C Template"},
			},
		},
	}, &GenerationQueueQuery{
		State:     "stubbed",
		SortBy:    "template_label",
		SortOrder: "asc",
		Page:      2,
		PageSize:  1,
	})
	if page == nil {
		t.Fatal("page = nil, want filtered queue page")
	}
	if page.Total != 2 || page.Page != 2 || page.PageSize != 1 {
		t.Fatalf("page = %+v, want filtered total with paging metadata", page)
	}
	if len(page.Items) != 1 {
		t.Fatalf("page.Items = %+v, want single paged item", page.Items)
	}
	if page.Items[0].Platform != "amazon" || page.Items[0].Slot != "gallery" || page.Items[0].TemplateLabel != "C Template" {
		t.Fatalf("page.Items[0] = %+v, want second filtered item after template sort", page.Items[0])
	}
	if page.Summary == nil || page.Summary.TotalItems != 2 || page.Summary.StubbedItems != 2 {
		t.Fatalf("page.Summary = %+v, want filtered summary before paging", page.Summary)
	}
}

func TestTaskGenerationQueueReadPageRunAttachesReviewSummaryBeforeDeltaTokenBuild(t *testing.T) {
	t.Parallel()

	query := &GenerationQueueQuery{Platform: "shein"}
	snapshot := &taskGenerationQueueReadSnapshot{
		task: &Task{
			ID:        "task-generation-queue-read-page-review-1",
			UpdatedAt: time.Date(2026, 5, 30, 10, 10, 0, 0, time.UTC),
		},
		result: &ListingKitResult{
			TaskID: "task-generation-queue-read-page-review-1",
			ReviewSummary: &GenerationReviewSummary{
				ApprovedSections:      2,
				DeferredSections:      1,
				ReviewPendingSections: 3,
			},
		},
		queue: &GenerationWorkQueue{
			Items: []GenerationWorkQueueItem{{
				TaskID:                 "task-generation-queue-read-page-review-1",
				Platform:               "shein",
				Slot:                   "main",
				State:                  "ready",
				RenderPreviewAvailable: true,
			}},
		},
	}

	page := buildTaskGenerationQueueReadPagePhase().run(snapshot, query)
	if page == nil {
		t.Fatal("page = nil, want queue page with attached review summary")
	}
	if page.Summary == nil || page.Summary.ApprovedSections != 2 || page.Summary.DeferredSections != 1 || page.Summary.ReviewPendingSections != 3 {
		t.Fatalf("page.Summary = %+v, want attached review summary counts", page.Summary)
	}

	expectedDeltaToken := buildGenerationQueueDeltaToken(page, query)
	if expectedDeltaToken == "" {
		t.Fatal("expectedDeltaToken = empty, want review-aware queue delta token")
	}
	withoutReviewSummary := *page
	withoutReviewSummary.Summary = &GenerationWorkQueueSummary{
		TotalItems:       page.Summary.TotalItems,
		ReadyItems:       page.Summary.ReadyItems,
		PreviewableItems: page.Summary.PreviewableItems,
	}
	if expectedDeltaToken == buildGenerationQueueDeltaToken(&withoutReviewSummary, query) {
		t.Fatalf("delta token = %q, want review summary attached before delta token build", expectedDeltaToken)
	}
}

func TestTaskGenerationQueueReadResponsePhaseFinalizesDeltaTokenAndConditionalState(t *testing.T) {
	t.Parallel()

	query := &GenerationQueueQuery{Platform: "shein"}
	page := &GenerationQueuePage{
		TaskID:    "task-generation-queue-read-response-final-1",
		Page:      1,
		PageSize:  10,
		Total:     1,
		UpdatedAt: time.Date(2026, 5, 30, 10, 20, 0, 0, time.UTC),
		Summary: &GenerationWorkQueueSummary{
			TotalItems:            1,
			ReadyItems:            1,
			ApprovedSections:      2,
			DeferredSections:      1,
			ReviewPendingSections: 3,
		},
		Items: []GenerationWorkQueueItem{{
			TaskID:                 "task-generation-queue-read-response-final-1",
			Platform:               "shein",
			Slot:                   "main",
			State:                  "ready",
			RenderPreviewAvailable: true,
		}},
	}

	response := buildTaskGenerationQueueReadResponsePhase().run(page.TaskID, page, query)
	if response == nil {
		t.Fatal("response = nil, want finalized queue response")
	}
	if response.DeltaToken == "" {
		t.Fatalf("response = %+v, want delta token", response)
	}
	if response.DeltaToken != buildGenerationQueueDeltaToken(page, query) {
		t.Fatalf("response.DeltaToken = %q, want queue delta token derived from final page", response.DeltaToken)
	}
	if response.NotModified {
		t.Fatalf("response = %+v, want full response when token does not match", response)
	}
	if response.Conditional == nil || response.Conditional.DeltaToken != response.DeltaToken || response.Conditional.NotModified {
		t.Fatalf("response.Conditional = %+v, want final conditional state applied", response.Conditional)
	}
}

func TestTaskGenerationQueueReadResponsePhaseShortCircuitsNotModifiedBeforeFinalPayload(t *testing.T) {
	t.Parallel()

	page := &GenerationQueuePage{
		TaskID:    "task-generation-queue-read-response-not-modified-1",
		Page:      2,
		PageSize:  5,
		Total:     4,
		UpdatedAt: time.Date(2026, 5, 30, 10, 25, 0, 0, time.UTC),
		Summary: &GenerationWorkQueueSummary{
			TotalItems:       4,
			ReadyItems:       4,
			PreviewableItems: 1,
		},
		Items: []GenerationWorkQueueItem{{
			TaskID:   "task-generation-queue-read-response-not-modified-1",
			Platform: "shein",
			Slot:     "main",
			State:    "ready",
		}},
	}
	query := &GenerationQueueQuery{
		Platform: "shein",
		Page:     page.Page,
		PageSize: page.PageSize,
	}
	query.DeltaToken = buildGenerationQueueDeltaToken(page, query)

	response := buildTaskGenerationQueueReadResponsePhase().run(page.TaskID, page, query)
	if response == nil {
		t.Fatal("response = nil, want not_modified queue response")
	}
	if !response.NotModified {
		t.Fatalf("response = %+v, want not_modified response", response)
	}
	if response.DeltaToken != query.DeltaToken {
		t.Fatalf("response.DeltaToken = %q, want %q", response.DeltaToken, query.DeltaToken)
	}
	if response.Summary != nil || len(response.Items) != 0 {
		t.Fatalf("response = %+v, want final payload fields omitted after not_modified short-circuit", response)
	}
	if response.Conditional == nil || !response.Conditional.NotModified {
		t.Fatalf("response.Conditional = %+v, want not_modified conditional state", response.Conditional)
	}
}

func TestTaskGenerationQueueReadServiceBoundary(t *testing.T) {
	t.Parallel()

	source := readExactMethodSource(t, "task_generation_service.go", "func (s *taskGenerationService) GetTaskGenerationQueue(")

	assertSourceOccurrenceCount(t, source, "buildTaskGenerationQueueReadSnapshotPhase(s).run(", 1)
	assertSourceOccurrenceCount(t, source, "buildTaskGenerationQueueReadPagePhase().run(", 1)
	assertSourceOccurrenceCount(t, source, "buildTaskGenerationQueueReadResponsePhase().run(", 1)
	assertSourceExcludesAll(t, source, []string{
		"buildGenerationQueueDeltaToken(",
		"isGenerationReviewReadNotModified(",
		"applyGenerationConditionalStateToQueuePage(",
		"buildGenerationQueuePage(",
		"filterGenerationQueueItems(",
		"sortGenerationQueueItems(",
		"paginateGenerationQueueItems(",
		"attachReviewSummaryToGenerationQueuePage(",
	})
}

func TestTaskGenerationReviewReadSnapshotPhaseRunUsesSingleCurrentSnapshot(t *testing.T) {
	t.Parallel()

	const taskID = "task-generation-review-snapshot-1"
	repo := &sequencedTaskSnapshotsRepo{
		snapshots: []*Task{
			{
				ID: taskID,
				Result: &ListingKitResult{
					TaskID: taskID,
					Shein: &SheinPackage{ImageBundle: &common.PublishImageBundle{
						Platform: "shein",
						Main: &common.BundleSlot{
							Key:           "main",
							AssetID:       "asset-first",
							StateLabel:    "ready",
							TemplateLabel: "First Snapshot",
						},
					}},
				},
			},
			{
				ID: taskID,
				Result: &ListingKitResult{
					TaskID: taskID,
					Shein: &SheinPackage{ImageBundle: &common.PublishImageBundle{
						Platform: "shein",
						Main: &common.BundleSlot{
							Key:           "main",
							AssetID:       "asset-second",
							StateLabel:    "ready",
							TemplateLabel: "Second Snapshot",
						},
					}},
				},
			},
		},
	}
	svc := &taskGenerationService{
		repo: repo,
		listAssetGenerationTasks: func(context.Context, string) ([]assetgeneration.Task, error) {
			return nil, nil
		},
		listGenerationReviews: func(context.Context, string) ([]GenerationReviewRecord, error) {
			return nil, nil
		},
	}

	snapshot, err := buildTaskGenerationReviewReadSnapshotPhase(svc).run(context.Background(), taskID)
	if err != nil {
		t.Fatalf("taskGenerationReviewReadSnapshotPhase.run() error = %v", err)
	}
	if snapshot == nil || snapshot.result == nil || snapshot.queue == nil {
		t.Fatalf("snapshot = %+v, want hydrated result + queue snapshot", snapshot)
	}
	if repo.getCalls != 1 {
		t.Fatalf("repo.getCalls = %d, want single current snapshot read", repo.getCalls)
	}
	if len(snapshot.queue.Items) != 1 || snapshot.queue.Items[0].AssetID != "asset-first" {
		t.Fatalf("queue = %+v, want queue from first snapshot", snapshot.queue)
	}
	if snapshot.result.AssetGenerationQueue == nil || len(snapshot.result.AssetGenerationQueue.Items) != 1 || snapshot.result.AssetGenerationQueue.Items[0].AssetID != "asset-first" {
		t.Fatalf("result queue = %+v, want result from same first snapshot", snapshot.result.AssetGenerationQueue)
	}
}

func TestTaskGenerationReviewSessionMissingSnapshotUsesCurrentEmptyResponseShape(t *testing.T) {
	t.Parallel()

	snapshot, err := buildTaskGenerationReviewReadSnapshotPhase(nil).run(context.Background(), "task-generation-review-session-empty-1")
	if err != nil {
		t.Fatalf("taskGenerationReviewReadSnapshotPhase.run() error = %v", err)
	}
	if snapshot == nil {
		t.Fatal("snapshot = nil, want missing snapshot handoff")
	}
	session := buildGenerationReviewSession(snapshot.result, snapshot.queue, &GenerationQueueQuery{
		Platform: "shein",
		Slot:     "main",
	})
	if session != nil {
		t.Fatalf("session = %+v, want nil when snapshot is missing", session)
	}
	response := applyGenerationConditionalStateToReviewSessionResponse(&GenerationReviewSessionResponse{TaskID: snapshot.taskID})
	if response == nil {
		t.Fatal("response = nil, want empty review session response")
	}
	if response.TaskID != snapshot.taskID || response.Session != nil || response.Patch != nil || response.DeltaToken != "" || response.NotModified {
		t.Fatalf("response = %+v, want current empty review session response shape", response)
	}
}

func TestTaskGenerationReviewReadsPropagateSnapshotLoadErrors(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("snapshot load failed")
	repo := &stubGenerationRepo{}
	svc := &service{
		repo: repo,
		taskGeneration: &taskGenerationService{
			repo: repo,
			listAssetGenerationTasks: func(context.Context, string) ([]assetgeneration.Task, error) {
				return nil, wantErr
			},
			listGenerationReviews: func(context.Context, string) ([]GenerationReviewRecord, error) {
				return nil, nil
			},
		},
	}

	task := &Task{
		ID:        "task-generation-review-error-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"shein"}},
		Result:    &ListingKitResult{TaskID: "task-generation-review-error-1"},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	tests := []struct {
		name string
		read func(context.Context, string, *GenerationQueueQuery) error
	}{
		{
			name: "session",
			read: func(ctx context.Context, taskID string, query *GenerationQueueQuery) error {
				_, err := svc.GetTaskGenerationReviewSession(ctx, taskID, query)
				return err
			},
		},
		{
			name: "preview",
			read: func(ctx context.Context, taskID string, query *GenerationQueueQuery) error {
				_, err := svc.GetTaskGenerationReviewPreview(ctx, taskID, query)
				return err
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.read(context.Background(), task.ID, &GenerationQueueQuery{Platform: "shein", Slot: "main"})
			if !errors.Is(err, wantErr) {
				t.Fatalf("%s err = %v, want %v", tc.name, err, wantErr)
			}
		})
	}
}

func TestTaskGenerationReviewSessionReadPhaseRunReturnsPatchOnlyResponse(t *testing.T) {
	t.Parallel()

	snapshot := &taskGenerationReviewReadSnapshot{
		taskID: "task-generation-review-session-read-patch-1",
		result: newTaskGenerationActionProjectionResult(
			"task-generation-review-session-read-patch-1",
			"asset-rev-patch",
			"preview-rev-patch",
			"task-rev-patch",
		),
		queue: newTaskGenerationActionProjectionQueue(
			"task-generation-review-session-read-patch-1",
			&GenerationWorkQueueSummary{
				TotalItems:       1,
				ReadyItems:       1,
				PreviewableItems: 1,
			},
			"ready",
		),
	}

	response := buildTaskGenerationReviewSessionReadPhase().run(
		snapshot.taskID,
		snapshot,
		&GenerationQueueQuery{
			Platform:          "shein",
			Slot:              "main",
			PreviewCapability: "detail_preview",
			ResponseMode:      "patch_only",
		},
	)
	if response == nil {
		t.Fatal("response = nil, want patch_only review session response")
	}
	if response.ResponseMode != "patch_only" {
		t.Fatalf("response.ResponseMode = %q, want patch_only", response.ResponseMode)
	}
	if response.Session != nil {
		t.Fatalf("response.Session = %+v, want patch_only response without full session payload", response.Session)
	}
	if response.Patch == nil {
		t.Fatalf("response.Patch = nil, want patch payload for patch_only response")
	}
	if response.DeltaToken == "" || response.Patch.DeltaToken != response.DeltaToken {
		t.Fatalf("response = %+v, want patch delta token to match response delta token", response)
	}
}

func TestTaskGenerationReviewSessionReadPhaseRunNormalizesResponseModeAndShortCircuitsNotModified(t *testing.T) {
	t.Parallel()

	snapshot := &taskGenerationReviewReadSnapshot{
		taskID: "task-generation-review-session-read-not-modified-1",
		result: newTaskGenerationActionProjectionResult(
			"task-generation-review-session-read-not-modified-1",
			"asset-rev-not-modified",
			"preview-rev-not-modified",
			"task-rev-not-modified",
		),
		queue: newTaskGenerationActionProjectionQueue(
			"task-generation-review-session-read-not-modified-1",
			&GenerationWorkQueueSummary{
				TotalItems:       1,
				ReadyItems:       1,
				PreviewableItems: 1,
			},
			"ready",
		),
	}
	query := &GenerationQueueQuery{
		Platform:          "shein",
		Slot:              "main",
		PreviewCapability: "detail_preview",
		ResponseMode:      "unexpected_mode",
	}
	current := buildGenerationReviewSession(snapshot.result, snapshot.queue, query)
	if current == nil {
		t.Fatal("current session = nil, want baseline review session for not-modified test")
	}
	query.DeltaToken = buildGenerationReviewDeltaToken(current)

	response := buildTaskGenerationReviewSessionReadPhase().run(snapshot.taskID, snapshot, query)
	if response == nil {
		t.Fatal("response = nil, want not_modified review session response")
	}
	if !response.NotModified {
		t.Fatalf("response = %+v, want not_modified short-circuit", response)
	}
	if response.ResponseMode != "full" {
		t.Fatalf("response.ResponseMode = %q, want normalized full response mode", response.ResponseMode)
	}
	if response.Session != nil || response.Patch != nil {
		t.Fatalf("response = %+v, want not_modified short-circuit before payload shaping", response)
	}
	if response.DeltaToken != query.DeltaToken {
		t.Fatalf("response.DeltaToken = %q, want %q", response.DeltaToken, query.DeltaToken)
	}
}

func TestTaskGenerationReviewSessionReadPhaseRunBuildsFullResponseDeltaToken(t *testing.T) {
	t.Parallel()

	snapshot := &taskGenerationReviewReadSnapshot{
		taskID: "task-generation-review-session-read-full-1",
		result: newTaskGenerationActionProjectionResult(
			"task-generation-review-session-read-full-1",
			"asset-rev-full",
			"preview-rev-full",
			"task-rev-full",
		),
		queue: newTaskGenerationActionProjectionQueue(
			"task-generation-review-session-read-full-1",
			&GenerationWorkQueueSummary{
				TotalItems:       1,
				ReadyItems:       1,
				PreviewableItems: 1,
			},
			"ready",
		),
	}

	response := buildTaskGenerationReviewSessionReadPhase().run(
		snapshot.taskID,
		snapshot,
		&GenerationQueueQuery{
			Platform:          "shein",
			Slot:              "main",
			PreviewCapability: "detail_preview",
			ResponseMode:      "unknown",
		},
	)
	if response == nil {
		t.Fatal("response = nil, want full review session response")
	}
	if response.ResponseMode != "full" {
		t.Fatalf("response.ResponseMode = %q, want normalized full response mode", response.ResponseMode)
	}
	if response.Session == nil {
		t.Fatalf("response = %+v, want full session payload", response)
	}
	if response.Patch != nil {
		t.Fatalf("response.Patch = %+v, want full response without patch payload", response.Patch)
	}
	if response.DeltaToken == "" || response.DeltaToken != buildGenerationReviewDeltaToken(response.Session) {
		t.Fatalf("response = %+v, want full response delta token derived from session", response)
	}
}

func TestTaskGenerationReviewPreviewReadPhaseRunBuildsPreviewFromSessionBaseline(t *testing.T) {
	t.Parallel()

	result := newTaskGenerationActionProjectionResult(
		"task-generation-review-preview-read-baseline-1",
		"asset-rev-preview",
		"preview-rev-preview",
		"task-rev-preview",
	)
	result.AssetBundle = &asset.Bundle{
		Assets: []asset.Asset{{
			ID:   "asset-preview-1",
			Kind: asset.KindSceneImage,
			Metadata: map[string]string{
				"prompt_key":            "productimage.scene.bags",
				"scene_defaults_source": "platform_category",
				"scene_category":        "bags",
			},
		}},
	}
	snapshot := &taskGenerationReviewReadSnapshot{
		taskID: result.TaskID,
		result: result,
		queue: newTaskGenerationActionProjectionQueue(
			result.TaskID,
			&GenerationWorkQueueSummary{
				TotalItems:       1,
				ReadyItems:       1,
				PreviewableItems: 1,
			},
			"ready",
		),
	}
	query := &GenerationQueueQuery{
		Platform:          "shein",
		Slot:              "main",
		PreviewCapability: "detail_preview",
		AssetID:           "asset-preview-1",
		AssetRevision:     "asset-rev-preview",
		PreviewRevision:   "preview-rev-other",
		TaskRevision:      "task-rev-preview",
	}
	session := buildGenerationReviewSession(snapshot.result, snapshot.queue, query)
	if session == nil {
		t.Fatal("session = nil, want baseline review session for preview read")
	}
	wantViewer, wantPreview, wantTarget, wantToolbar := resolveGenerationReviewPreviewResponse(session, query)
	wantRevisionStatus, wantRevisionReason := resolveGenerationReviewPreviewRevisionStatus(wantViewer, query)

	response := buildTaskGenerationReviewPreviewReadPhase().run(snapshot.taskID, snapshot, query)
	if response == nil {
		t.Fatal("response = nil, want preview read response")
	}
	if response.DeltaToken != buildGenerationReviewDeltaToken(session) {
		t.Fatalf("response.DeltaToken = %q, want %q", response.DeltaToken, buildGenerationReviewDeltaToken(session))
	}
	if response.Viewer == nil || wantViewer == nil || response.Viewer.AssetID != wantViewer.AssetID || response.Viewer.AssetRevision != wantViewer.AssetRevision || response.Viewer.PreviewRevision != wantViewer.PreviewRevision || response.Viewer.TaskRevision != wantViewer.TaskRevision {
		t.Fatalf("response.Viewer = %+v, want baseline viewer %+v", response.Viewer, wantViewer)
	}
	if response.Preview == nil || wantPreview == nil || response.Preview.AssetID != wantPreview.AssetID || response.Preview.Slot != wantPreview.Slot {
		t.Fatalf("response.Preview = %+v, want baseline preview %+v", response.Preview, wantPreview)
	}
	if response.ReviewTarget == nil || wantTarget == nil || response.ReviewTarget.Platform != wantTarget.Platform || response.ReviewTarget.Slot != wantTarget.Slot || response.ReviewTarget.Capability != wantTarget.Capability {
		t.Fatalf("response.ReviewTarget = %+v, want baseline target %+v", response.ReviewTarget, wantTarget)
	}
	if response.Toolbar == nil || wantToolbar == nil || response.Toolbar.Platform != wantToolbar.Platform || response.Toolbar.Slot != wantToolbar.Slot || response.Toolbar.Capability != wantToolbar.Capability {
		t.Fatalf("response.Toolbar = %+v, want baseline toolbar %+v", response.Toolbar, wantToolbar)
	}
	if response.RevisionStatus != wantRevisionStatus || response.RevisionMismatchReason != wantRevisionReason {
		t.Fatalf("revision = (%q, %q), want (%q, %q)", response.RevisionStatus, response.RevisionMismatchReason, wantRevisionStatus, wantRevisionReason)
	}
	if response.ScenePreset == nil || response.ScenePreset.PromptKey != "productimage.scene.bags" || response.ScenePreset.SceneCategory != "bags" {
		t.Fatalf("response.ScenePreset = %+v, want scene preset summary from focused preview asset", response.ScenePreset)
	}
	if response.Conditional == nil || response.Conditional.DeltaToken != response.DeltaToken {
		t.Fatalf("response.Conditional = %+v, want final conditional decoration", response.Conditional)
	}
	if len(response.ResourceDescriptors) == 0 {
		t.Fatalf("response.ResourceDescriptors = %+v, want decorated preview resource descriptors", response.ResourceDescriptors)
	}
}

func TestTaskGenerationReviewPreviewReadPhaseRunShortCircuitsNotModifiedBeforeProjection(t *testing.T) {
	t.Parallel()

	snapshot := &taskGenerationReviewReadSnapshot{
		taskID: "task-generation-review-preview-read-not-modified-1",
		result: newTaskGenerationActionProjectionResult(
			"task-generation-review-preview-read-not-modified-1",
			"asset-rev-preview-not-modified",
			"preview-rev-preview-not-modified",
			"task-rev-preview-not-modified",
		),
		queue: newTaskGenerationActionProjectionQueue(
			"task-generation-review-preview-read-not-modified-1",
			&GenerationWorkQueueSummary{
				TotalItems:       1,
				ReadyItems:       1,
				PreviewableItems: 1,
			},
			"ready",
		),
	}
	query := &GenerationQueueQuery{
		Platform:          "shein",
		Slot:              "main",
		PreviewCapability: "detail_preview",
	}
	session := buildGenerationReviewSession(snapshot.result, snapshot.queue, query)
	if session == nil {
		t.Fatal("session = nil, want preview session baseline for not_modified test")
	}
	query.DeltaToken = buildGenerationReviewDeltaToken(session)

	response := buildTaskGenerationReviewPreviewReadPhase().run(snapshot.taskID, snapshot, query)
	if response == nil {
		t.Fatal("response = nil, want not_modified preview response")
	}
	if !response.NotModified {
		t.Fatalf("response = %+v, want not_modified short-circuit", response)
	}
	if response.Viewer != nil || response.Preview != nil || response.ReviewTarget != nil || response.Toolbar != nil || response.ScenePreset != nil {
		t.Fatalf("response = %+v, want not_modified short-circuit before preview projection", response)
	}
	if response.Conditional == nil || !response.Conditional.NotModified || response.Conditional.DeltaToken != response.DeltaToken {
		t.Fatalf("response.Conditional = %+v, want not_modified conditional decoration", response.Conditional)
	}
}

func TestTaskGenerationReviewPreviewReadPhaseRunReturnsEmptyResponseWhenSessionMissing(t *testing.T) {
	t.Parallel()

	response := buildTaskGenerationReviewPreviewReadPhase().run(
		"task-generation-review-preview-read-empty-1",
		&taskGenerationReviewReadSnapshot{taskID: "task-generation-review-preview-read-empty-1"},
		&GenerationQueueQuery{Platform: "shein", Slot: "main"},
	)
	if response == nil {
		t.Fatal("response = nil, want empty preview response")
	}
	if response.TaskID != "task-generation-review-preview-read-empty-1" || response.Viewer != nil || response.Preview != nil || response.ReviewTarget != nil || response.Toolbar != nil || response.ScenePreset != nil || response.DeltaToken != "" || response.NotModified {
		t.Fatalf("response = %+v, want current empty preview response shape", response)
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

package listingkit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	studiodomain "task-processor/internal/listing/studio"
	sheinpub "task-processor/internal/publishing/shein"
	sdstemplate "task-processor/internal/sds/template"

	"gorm.io/gorm"
)

func TestServiceGetStudioBatchDetailProjectsItemizedGraph(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	if err := repo.CreateStudioBatchGraph(ctx, newStudioBatchRecordForTest("batch-1", now), []StudioBatchItemRecord{
		{
			ID:               "item-1",
			BatchID:          "batch-1",
			TargetGroupKey:   "size:1200x1200",
			TargetGroupLabel: "1200 x 1200",
			Status:           StudioBatchItemStatusReviewReady,
			SelectionCount:   3,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		{
			ID:               "item-2",
			BatchID:          "batch-1",
			TargetGroupKey:   "size:2000x2000",
			TargetGroupLabel: "2000 x 2000",
			Status:           StudioBatchItemStatusGenerating,
			SelectionCount:   2,
			CreatedAt:        now.Add(time.Second),
			UpdatedAt:        now.Add(time.Second),
		},
	}, []StudioGenerationAttemptRecord{
		{
			ID:        "attempt-1",
			ItemID:    "item-1",
			AttemptNo: 1,
			Status:    StudioGenerationAttemptStatusMaterialized,
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:        "attempt-2",
			ItemID:    "item-2",
			AttemptNo: 1,
			Status:    StudioGenerationAttemptStatusRunning,
			CreatedAt: now.Add(time.Second),
			UpdatedAt: now.Add(time.Second),
		},
	}, []StudioMaterializedDesignRecord{
		{
			ID:               "design-1",
			BatchID:          "batch-1",
			ItemID:           "item-1",
			SourceAttemptID:  "attempt-1",
			TargetGroupKey:   "size:1200x1200",
			TargetGroupLabel: "1200 x 1200",
			ImageURL:         "https://cdn.example.com/design-1.png",
			ReviewStatus:     StudioMaterializedDesignReviewStatusApproved,
			SortOrder:        0,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		{
			ID:               "design-2",
			BatchID:          "batch-1",
			ItemID:           "item-1",
			SourceAttemptID:  "attempt-1",
			TargetGroupKey:   "size:1200x1200",
			TargetGroupLabel: "1200 x 1200",
			ImageURL:         "https://cdn.example.com/design-2.png",
			ReviewStatus:     StudioMaterializedDesignReviewStatusUnreviewed,
			SortOrder:        1,
			CreatedAt:        now.Add(time.Second),
			UpdatedAt:        now.Add(time.Second),
		},
		{
			ID:               "design-3",
			BatchID:          "batch-1",
			ItemID:           "item-2",
			SourceAttemptID:  "attempt-2",
			TargetGroupKey:   "size:2000x2000",
			TargetGroupLabel: "2000 x 2000",
			ImageURL:         "https://cdn.example.com/design-3.png",
			ReviewStatus:     StudioMaterializedDesignReviewStatusRejected,
			SortOrder:        0,
			CreatedAt:        now.Add(2 * time.Second),
			UpdatedAt:        now.Add(2 * time.Second),
		},
	}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	svc := &service{studioDeps: studioDependencies{batchRepo: repo}}
	detail, err := svc.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}

	if detail.Batch == nil || detail.Batch.ID != "batch-1" {
		t.Fatalf("detail.Batch = %+v, want batch-1", detail.Batch)
	}
	if len(detail.Items) != 2 {
		t.Fatalf("len(detail.Items) = %d, want 2", len(detail.Items))
	}
	if detail.Items[0].Item.ID != "item-1" || len(detail.Items[0].Attempts) != 1 || len(detail.Items[0].Designs) != 2 {
		t.Fatalf("detail.Items[0] = %+v, want item-1 with 1 attempt and 2 designs", detail.Items[0])
	}
	if detail.Items[1].Item.ID != "item-2" || len(detail.Items[1].Attempts) != 1 || len(detail.Items[1].Designs) != 1 {
		t.Fatalf("detail.Items[1] = %+v, want item-2 with 1 attempt and 1 design", detail.Items[1])
	}
	if detail.Items[0].Designs[0].ID != "design-1" || detail.Items[0].Designs[1].ID != "design-2" {
		t.Fatalf("detail.Items[0].Designs = %+v, want sorted item-1 designs", detail.Items[0].Designs)
	}
	assertStudioBatchStatusGroup(t, detail.StatusGroups, "submittable", 1, "item-1")
	assertStudioBatchStatusGroup(t, detail.StatusGroups, "processing", 1, "item-2")
}

func TestTaskStudioBatchServiceUsesListingStudioDetailRunner(t *testing.T) {
	svc := newTaskStudioBatchService(taskStudioBatchServiceConfig{
		detailRunner: studiodomain.NewBatchDetailService(studiodomain.BatchDetailServiceConfig[
			StudioBatchDetailGraph,
			StudioBatchDetail,
		]{
			LoadGraph: func(context.Context, string) (*StudioBatchDetailGraph, error) {
				return &StudioBatchDetailGraph{
					Batch: &StudioBatchRecord{ID: "batch-1"},
				}, nil
			},
			IsGraphMissing: func(error) bool { return false },
			ResolveWithoutGraph: func(context.Context, string) (*StudioBatchDetail, bool, error) {
				return nil, false, nil
			},
			EnsureGraph: func(context.Context, string) error { return nil },
			ProjectDetail: func(context.Context, string, *StudioBatchDetailGraph) (*StudioBatchDetail, error) {
				return &StudioBatchDetail{Batch: &StudioBatchRecord{ID: "batch-1"}}, nil
			},
		}),
	})

	detail, err := svc.GetStudioBatchDetail(context.Background(), "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}
	if detail.Batch == nil || detail.Batch.ID != "batch-1" {
		t.Fatalf("detail.Batch = %+v, want batch-1", detail.Batch)
	}
}

func TestTaskStudioBatchServiceUsesListingStudioReviewRunner(t *testing.T) {
	svc := newTaskStudioBatchService(taskStudioBatchServiceConfig{
		reviewRunner: studiodomain.NewBatchDesignReviewService(studiodomain.BatchDesignReviewServiceConfig[StudioBatchDetail]{
			EnsureBatchExists: func(context.Context, string) error { return nil },
			ReplaceReviews:    func(context.Context, string, []string, time.Time) error { return nil },
			LoadDetail: func(context.Context, string) (*StudioBatchDetail, error) {
				return &StudioBatchDetail{Batch: &StudioBatchRecord{ID: "batch-1"}}, nil
			},
			CurrentTime: time.Now,
		}),
	})

	detail, err := svc.ApproveStudioBatchDesigns(context.Background(), "batch-1", &ApproveStudioBatchDesignsRequest{
		DesignIDs: []string{"design-1"},
	})
	if err != nil {
		t.Fatalf("ApproveStudioBatchDesigns() error = %v", err)
	}
	if detail.Batch == nil || detail.Batch.ID != "batch-1" {
		t.Fatalf("detail.Batch = %+v, want batch-1", detail.Batch)
	}
}

func TestServiceGetStudioBatchDetailIncludesDraftUpdatedAtFromSavedBatchDraft(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Date(2026, 6, 2, 2, 0, 0, 0, time.UTC)
	draftUpdatedAt := now.Add(-3 * time.Minute)

	if err := repo.CreateStudioBatchGraph(ctx, newStudioBatchRecordForTest("batch-1", now), []StudioBatchItemRecord{
		{
			ID:               "item-1",
			BatchID:          "batch-1",
			TargetGroupKey:   "size:1200x1200",
			TargetGroupLabel: "1200 x 1200",
			Status:           StudioBatchItemStatusPending,
			SelectionCount:   1,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
	}, nil, nil); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	svc := &service{studioDeps: studioDependencies{batchRepo: repo, sessionRepo: &studioBatchGenerationSessionRepoStub{
		session: &SheinStudioSession{
			ID:           "batch-1",
			SavedAsBatch: true,
			UpdatedAt:    draftUpdatedAt,
		},
	}},
	}

	detail, err := svc.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}
	if detail.Batch == nil || detail.Batch.DraftUpdatedAt == nil {
		t.Fatalf("detail.Batch.DraftUpdatedAt = %+v, want non-nil", detail.Batch)
	}
	if got := detail.Batch.DraftUpdatedAt.UTC(); !got.Equal(draftUpdatedAt) {
		t.Fatalf("detail.Batch.DraftUpdatedAt = %s, want %s", got, draftUpdatedAt)
	}
}

func TestServiceGetStudioBatchDetailDerivesActiveBatchStatusFromItems(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()
	batch := newStudioBatchRecordForTest("batch-1", now)
	batch.Status = StudioBatchStatusPartiallyFailed

	if err := repo.CreateStudioBatchGraph(ctx, batch, []StudioBatchItemRecord{
		{
			ID:               "item-1",
			BatchID:          "batch-1",
			TargetGroupKey:   "size:1200x1200",
			TargetGroupLabel: "1200 x 1200",
			Status:           StudioBatchItemStatusReviewReady,
			SelectionCount:   1,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		{
			ID:               "item-2",
			BatchID:          "batch-1",
			TargetGroupKey:   "size:2000x2000",
			TargetGroupLabel: "2000 x 2000",
			Status:           StudioBatchItemStatusGenerating,
			SelectionCount:   1,
			CreatedAt:        now.Add(time.Second),
			UpdatedAt:        now.Add(time.Second),
		},
	}, nil, nil); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	svc := &service{studioDeps: studioDependencies{batchRepo: repo}}
	detail, err := svc.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}

	if detail.Batch == nil {
		t.Fatal("detail.Batch = nil, want batch detail")
	}
	if got := detail.Batch.Status; got != StudioBatchStatusPartiallyMaterialized {
		t.Fatalf("detail.Batch.Status = %q, want %q", got, StudioBatchStatusPartiallyMaterialized)
	}
}

func TestServiceGetStudioBatchDetailPreservesTasksCreatedStatus(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()
	batch := newStudioBatchRecordForTest("batch-1", now)
	batch.Status = StudioBatchStatusTasksCreated

	if err := repo.CreateStudioBatchGraph(ctx, batch, []StudioBatchItemRecord{
		{
			ID:               "item-1",
			BatchID:          "batch-1",
			TargetGroupKey:   "size:1200x1200",
			TargetGroupLabel: "1200 x 1200",
			Status:           StudioBatchItemStatusReviewReady,
			SelectionCount:   1,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
	}, nil, nil); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	svc := &service{studioDeps: studioDependencies{batchRepo: repo}}
	detail, err := svc.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}

	if detail.Batch == nil {
		t.Fatal("detail.Batch = nil, want batch detail")
	}
	if got := detail.Batch.Status; got != StudioBatchStatusTasksCreated {
		t.Fatalf("detail.Batch.Status = %q, want %q", got, StudioBatchStatusTasksCreated)
	}
}

func TestServiceTaskStudioBatchOrDefaultCachesOnService(t *testing.T) {
	t.Parallel()

	svc := &service{studioDeps: studioDependencies{batchRepo: NewMemStudioBatchRepository()}}
	collaborator := svc.taskStudioBatchOrDefault()
	if collaborator == nil {
		t.Fatal("taskStudioBatchOrDefault() = nil, want collaborator")
	}
	if svc.studio.batchGroup.batch != collaborator {
		t.Fatal("expected collaborator to be cached on studio group")
	}
}

func TestTaskStudioBatchServiceContinueGenerationRecoversBeforeRunningPendingItems(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()
	if err := repo.CreateStudioBatchGraph(ctx, newStudioBatchRecordForTest("batch-1", now), []StudioBatchItemRecord{
		{
			ID:               "item-1",
			BatchID:          "batch-1",
			TargetGroupKey:   "size:1200x1200",
			TargetGroupLabel: "1200 x 1200",
			Status:           StudioBatchItemStatusGenerating,
			SelectionCount:   1,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
	}, nil, nil); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	var calls []string
	svc := newTaskStudioBatchService(taskStudioBatchServiceConfig{
		repo: repo,
		generator: studioBatchGeneratorStub{
			recover: func(context.Context, string) error {
				calls = append(calls, "recover")
				return nil
			},
			runPending: func(context.Context, string) error {
				calls = append(calls, "run_pending")
				return nil
			},
		},
	})

	if _, err := svc.continueStudioBatchGeneration(ctx, "batch-1"); err != nil {
		t.Fatalf("continueStudioBatchGeneration() error = %v", err)
	}

	if got := strings.Join(calls, ","); got != "recover,run_pending,recover" {
		t.Fatalf("generator calls = %q, want recover before and after run_pending", got)
	}
}

func TestBuildStudioBatchItemDesignRequestUsesHotReferenceImageDirectly(t *testing.T) {
	t.Parallel()

	batch := &StudioBatchRecord{
		ID:                      "batch-1",
		Prompt:                  "punk eagle collage",
		HotStyleReferenceBrief:  "retro badge with cream and red palette",
		HotStyleReferencePrompt: "Create an original retro badge.",
		StyleCount:              "1",
		VariationIntensity:      "medium",
		ArtworkModel:            "gpt-image-2",
		HotStyleReferenceImageURLs: SheinStudioStringList{
			"https://example.com/hot-ref.png",
			"https://example.com/hot-ref-2.png",
		},
		Selection: SheinStudioSelectionSnapshot(func() SheinStudioSelection {
			selection := testStudioBatchSelection(101, "Canvas Tote", "Red", 1200, 1200)
			selection.MockupImageURL = "https://example.com/mockup.png"
			selection.MockupImageURLs = []string{
				"https://example.com/mockup-1.png",
				"https://example.com/mockup-2.png",
				"https://example.com/mockup-3.png",
				"https://example.com/mockup-4.png",
			}
			selection.SizeReferenceImageURLs = []string{"https://example.com/size.png"}
			return selection
		}()),
		GroupedSelections: SheinStudioGroupedSelectionList{
			{
				SelectionID: "7001:9001:101:layer-1:101",
				Selection: func() SheinStudioSelection {
					selection := testStudioBatchSelection(101, "Canvas Tote", "Red", 1200, 1200)
					selection.MockupImageURL = "https://example.com/mockup.png"
					selection.MockupImageURLs = []string{
						"https://example.com/mockup-1.png",
						"https://example.com/mockup-2.png",
						"https://example.com/mockup-3.png",
						"https://example.com/mockup-4.png",
					}
					selection.SizeReferenceImageURLs = []string{"https://example.com/size.png"}
					return selection
				}(),
				Eligible: true,
			},
		},
		SelectedSDSImages: SheinStudioSelectedSDSImageList{
			{ImageURL: "https://example.com/sds.png"},
		},
	}
	item := StudioBatchItemRecord{
		ID:           "item-1",
		BatchID:      "batch-1",
		SelectionIDs: []string{selectionIDForStudioSelection(batch.GroupedSelections[0].Selection)},
	}

	req := buildStudioBatchItemDesignRequest(batch, item)
	if req == nil {
		t.Fatal("buildStudioBatchItemDesignRequest() = nil")
	}
	if got := req.Prompt; got != "punk eagle collage" {
		t.Fatalf("Prompt = %q, want user prompt only", got)
	}
	if got, want := req.ArtworkGenerationMode, "hot_reference"; got != want {
		t.Fatalf("ArtworkGenerationMode = %q, want %q", got, want)
	}
	if got, want := req.ProductReferenceImageURLs, []string{"https://example.com/hot-ref.png"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("ProductReferenceImageURLs = %v, want %v", got, want)
	}
}

func TestBuildStudioBatchItemDesignRequestUsesHotReferenceImageWithoutAnalysis(t *testing.T) {
	t.Parallel()

	batch := &StudioBatchRecord{
		ID:                      "batch-1",
		Prompt:                  "punk eagle collage",
		HotStyleReferencePrompt: "Create an original retro badge.",
		HotStyleReferenceImageURLs: SheinStudioStringList{
			"https://example.com/hot-ref.png",
		},
		Selection: SheinStudioSelectionSnapshot(func() SheinStudioSelection {
			selection := testStudioBatchSelection(101, "Canvas Tote", "Red", 1200, 1200)
			selection.MockupImageURL = "https://example.com/mockup.png"
			return selection
		}()),
		GroupedSelections: SheinStudioGroupedSelectionList{
			{
				SelectionID: "7001:9001:101:layer-1:101",
				Selection:   testStudioBatchSelection(101, "Canvas Tote", "Red", 1200, 1200),
				Eligible:    true,
			},
		},
	}
	item := StudioBatchItemRecord{
		ID:           "item-1",
		BatchID:      "batch-1",
		SelectionIDs: []string{selectionIDForStudioSelection(batch.GroupedSelections[0].Selection)},
	}

	req := buildStudioBatchItemDesignRequest(batch, item)
	if req == nil {
		t.Fatal("buildStudioBatchItemDesignRequest() = nil")
	}
	if got := req.Prompt; got != "punk eagle collage" {
		t.Fatalf("Prompt = %q, want user prompt only", got)
	}
	if got, want := req.ArtworkGenerationMode, "hot_reference"; got != want {
		t.Fatalf("ArtworkGenerationMode = %q, want %q", got, want)
	}
	if got, want := req.ProductReferenceImageURLs, []string{"https://example.com/hot-ref.png"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("ProductReferenceImageURLs = %v, want %v", got, want)
	}
}

func TestTaskStudioBatchServiceContinueGenerationAutoRetriesStaleGeneratingItemWithinAttemptLimit(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Date(2026, 6, 1, 10, 30, 0, 0, time.UTC)

	if err := repo.CreateStudioBatchGraph(ctx, newStudioBatchRecordForTest("batch-1", now), []StudioBatchItemRecord{
		{
			ID:               "item-1",
			BatchID:          "batch-1",
			TargetGroupKey:   "size:1200x1200",
			TargetGroupLabel: "1200 x 1200",
			Status:           StudioBatchItemStatusGenerating,
			SelectionCount:   1,
			CreatedAt:        now.Add(-20 * time.Minute),
			UpdatedAt:        now.Add(-20 * time.Minute),
		},
	}, []StudioGenerationAttemptRecord{
		{
			ID:        "attempt-1",
			ItemID:    "item-1",
			AttemptNo: 1,
			Status:    StudioGenerationAttemptStatusRunning,
			StartedAt: timePtr(now.Add(-20 * time.Minute)),
			CreatedAt: now.Add(-20 * time.Minute),
			UpdatedAt: now.Add(-20 * time.Minute),
		},
	}, nil); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	svc := newTaskStudioBatchService(taskStudioBatchServiceConfig{
		repo: repo,
		generator: newStudioBatchGenerationService(studioBatchGenerationServiceConfig{
			repo: repo,
			execute: func(_ context.Context, input StudioBatchGenerateExecutionInput) (*StudioBatchGenerateExecutionOutput, error) {
				return &StudioBatchGenerateExecutionOutput{
					Response: testStudioDesignResponse("design-"+input.ItemID, "https://cdn.example.com/"+input.ItemID+".png"),
					ItemID:   input.ItemID,
					BatchID:  input.BatchID,
				}, nil
			},
			currentTime: func() time.Time { return now },
		}),
	})

	detail, err := svc.continueStudioBatchGeneration(ctx, "batch-1")
	if err != nil {
		t.Fatalf("continueStudioBatchGeneration() error = %v", err)
	}

	if len(detail.Items) != 1 {
		t.Fatalf("len(detail.Items) = %d, want 1", len(detail.Items))
	}
	if got := detail.Items[0].Item.Status; got != StudioBatchItemStatusReviewReady {
		t.Fatalf("item status = %q, want review_ready after stale auto retry", got)
	}
	if got := len(detail.Items[0].Attempts); got != 2 {
		t.Fatalf("attempt count = %d, want 2", got)
	}
	if got := detail.Items[0].Attempts[0].Status; got != StudioGenerationAttemptStatusFailed {
		t.Fatalf("attempt-1 status = %q, want failed stale timeout", got)
	}
	if got := detail.Items[0].Attempts[1].Status; got != StudioGenerationAttemptStatusMaterialized {
		t.Fatalf("attempt-2 status = %q, want materialized after retry", got)
	}
	if got := len(detail.Items[0].Designs); got != 1 {
		t.Fatalf("design count = %d, want 1", got)
	}
}

func TestTaskStudioBatchServiceContinueGenerationDoesNotAutoRetryFailedItem(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Date(2026, 6, 1, 10, 40, 0, 0, time.UTC)

	if err := repo.CreateStudioBatchGraph(ctx, newStudioBatchRecordForTest("batch-1", now), []StudioBatchItemRecord{
		{
			ID:               "item-1",
			BatchID:          "batch-1",
			TargetGroupKey:   "size:1200x1200",
			TargetGroupLabel: "1200 x 1200",
			Status:           StudioBatchItemStatusFailed,
			LastError:        "submit image generation request returned status 400: generate image failed",
			SelectionCount:   1,
			CreatedAt:        now.Add(-20 * time.Minute),
			UpdatedAt:        now.Add(-20 * time.Minute),
		},
	}, []StudioGenerationAttemptRecord{
		{
			ID:           "attempt-1",
			ItemID:       "item-1",
			AttemptNo:    1,
			Status:       StudioGenerationAttemptStatusFailed,
			ErrorMessage: "submit image generation request returned status 400: generate image failed",
			CreatedAt:    now.Add(-20 * time.Minute),
			UpdatedAt:    now.Add(-20 * time.Minute),
		},
	}, nil); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	svc := newTaskStudioBatchService(taskStudioBatchServiceConfig{
		repo: repo,
		generator: studioBatchGeneratorStub{
			recover:    func(context.Context, string) error { return nil },
			runPending: func(context.Context, string) error { return nil },
		},
	})

	detail, err := svc.continueStudioBatchGeneration(ctx, "batch-1")
	if err != nil {
		t.Fatalf("continueStudioBatchGeneration() error = %v", err)
	}

	if len(detail.Items) != 1 {
		t.Fatalf("len(detail.Items) = %d, want 1", len(detail.Items))
	}
	if got := detail.Items[0].Item.Status; got != StudioBatchItemStatusFailed {
		t.Fatalf("item status = %q, want failed without explicit retry", got)
	}
	if got := detail.Items[0].Item.LastError; got == "" {
		t.Fatal("item last error was cleared without explicit retry")
	}
	if got := len(detail.Items[0].Attempts); got != 1 {
		t.Fatalf("attempt count = %d, want original failed attempt only", got)
	}
}

func TestTaskStudioBatchServiceRetryItemsRetriesPreviouslyFailedRetryableItem(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Date(2026, 6, 1, 10, 45, 0, 0, time.UTC)

	if err := repo.CreateStudioBatchGraph(ctx, newStudioBatchRecordForTest("batch-1", now), []StudioBatchItemRecord{
		{
			ID:               "item-1",
			BatchID:          "batch-1",
			TargetGroupKey:   "size:1200x1200",
			TargetGroupLabel: "1200 x 1200",
			Status:           StudioBatchItemStatusFailed,
			LastError:        "generation attempt timed out before result persisted",
			SelectionCount:   1,
			CreatedAt:        now.Add(-20 * time.Minute),
			UpdatedAt:        now.Add(-20 * time.Minute),
		},
	}, []StudioGenerationAttemptRecord{
		{
			ID:           "attempt-1",
			ItemID:       "item-1",
			AttemptNo:    1,
			Status:       StudioGenerationAttemptStatusFailed,
			ErrorMessage: `generate studio design 1: 调用 OpenAI image API 失败，已重试3次: image api returned status 400: {"error":{"message":"excessive system load"}}`,
			CreatedAt:    now.Add(-40 * time.Minute),
			UpdatedAt:    now.Add(-40 * time.Minute),
		},
		{
			ID:           "attempt-2",
			ItemID:       "item-1",
			AttemptNo:    2,
			Status:       StudioGenerationAttemptStatusFailed,
			ErrorMessage: "generation attempt timed out before result persisted",
			CreatedAt:    now.Add(-30 * time.Minute),
			UpdatedAt:    now.Add(-30 * time.Minute),
		},
		{
			ID:           "attempt-3",
			ItemID:       "item-1",
			AttemptNo:    defaultStudioBatchTransientRetryLimit,
			Status:       StudioGenerationAttemptStatusFailed,
			ErrorMessage: "generation attempt timed out before result persisted",
			CreatedAt:    now.Add(-20 * time.Minute),
			UpdatedAt:    now.Add(-20 * time.Minute),
		},
	}, nil); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	svc := newTaskStudioBatchService(taskStudioBatchServiceConfig{
		repo: repo,
		generator: newStudioBatchGenerationService(studioBatchGenerationServiceConfig{
			repo: repo,
			execute: func(_ context.Context, input StudioBatchGenerateExecutionInput) (*StudioBatchGenerateExecutionOutput, error) {
				return &StudioBatchGenerateExecutionOutput{
					Response: testStudioDesignResponse("design-"+input.ItemID, "https://cdn.example.com/"+input.ItemID+".png"),
					ItemID:   input.ItemID,
					BatchID:  input.BatchID,
				}, nil
			},
			currentTime: func() time.Time { return now },
		}),
	})

	detail, err := svc.RetryStudioBatchItems(ctx, "batch-1", &RetryStudioBatchItemsRequest{
		ItemIDs: []string{"item-1"},
	})
	if err != nil {
		t.Fatalf("RetryStudioBatchItems() error = %v", err)
	}

	if len(detail.Items) != 1 {
		t.Fatalf("len(detail.Items) = %d, want 1", len(detail.Items))
	}
	if got := detail.Items[0].Item.Status; got != StudioBatchItemStatusReviewReady {
		t.Fatalf("item status = %q, want review_ready after retrying previous failed item", got)
	}
	if got := len(detail.Items[0].Attempts); got != 4 {
		t.Fatalf("attempt count = %d, want 4", got)
	}
	if got := detail.Items[0].Attempts[2].Status; got != StudioGenerationAttemptStatusFailed {
		t.Fatalf("attempt-3 status = %q, want preserved failed original attempt", got)
	}
	if got := detail.Items[0].Attempts[3].Status; got != StudioGenerationAttemptStatusMaterialized {
		t.Fatalf("attempt-4 status = %q, want materialized retry attempt", got)
	}
	if got := len(detail.Items[0].Designs); got != 1 {
		t.Fatalf("design count = %d, want 1", got)
	}
}

func TestTaskStudioBatchServiceRetryItemsUsesExistingHotReferenceImageWithPromptlessDraft(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Date(2026, 7, 4, 13, 30, 0, 0, time.UTC)
	batch := newStudioBatchRecordForTest("batch-1", now)
	batch.Prompt = ""
	batch.HotStyleReferenceImageURLs = SheinStudioStringList{"https://cdn.example.com/hot-ref.png"}
	batch.HotStyleReferenceBrief = "dense streetwear skull reference"
	batch.HotStyleReferencePrompt = "Create an original skull streetwear print."
	batch.ArtworkModel = "gpt-image-2"

	if err := repo.CreateStudioBatchGraph(ctx, batch, []StudioBatchItemRecord{
		{
			ID:               "item-1",
			BatchID:          "batch-1",
			TargetGroupKey:   "size:2880x3000",
			TargetGroupLabel: "2880 x 3000",
			Status:           StudioBatchItemStatusFailed,
			LastError:        "timeout",
			SelectionCount:   1,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
	}, []StudioGenerationAttemptRecord{
		{
			ID:           "attempt-1",
			BatchID:      "batch-1",
			ItemID:       "item-1",
			AttemptNo:    1,
			Status:       StudioGenerationAttemptStatusFailed,
			ErrorMessage: "timeout",
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}, nil); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	sessionRepo := &studioBatchGenerationSessionRepoStub{
		session: &SheinStudioSession{
			ID:               "batch-1",
			SavedAsBatch:     true,
			Status:           SheinStudioSessionStatusSelecting,
			Prompt:           "",
			StyleCount:       "1",
			ArtworkModel:     "",
			GroupedImageMode: "shared_by_size",
			Selection:        SheinStudioSelectionSnapshot(testStudioBatchSelection(101, "Dress", "White", 2880, 3000)),
		},
	}
	var capturedReq *StudioDesignRequest
	svc := newTaskStudioBatchService(taskStudioBatchServiceConfig{
		repo:              repo,
		studioSessionRepo: sessionRepo,
		generator: newStudioBatchGenerationService(studioBatchGenerationServiceConfig{
			repo: repo,
			execute: func(ctx context.Context, input StudioBatchGenerateExecutionInput) (*StudioBatchGenerateExecutionOutput, error) {
				capturedReq = input.Request
				return &StudioBatchGenerateExecutionOutput{
					Response: testStudioDesignResponse("design-1", "https://cdn.example.com/design.png"),
					ItemID:   input.ItemID,
					BatchID:  input.BatchID,
				}, nil
			},
			currentTime: func() time.Time { return now.Add(time.Minute) },
		}),
	})

	detail, err := svc.RetryStudioBatchItems(ctx, "batch-1", &RetryStudioBatchItemsRequest{
		ItemIDs: []string{"item-1"},
	})
	if err != nil {
		t.Fatalf("RetryStudioBatchItems() error = %v", err)
	}
	if capturedReq == nil {
		t.Fatal("capturedReq = nil, want retry generation request")
	}
	if got := capturedReq.Prompt; got != "" {
		t.Fatalf("captured prompt = %q, want empty user prompt", got)
	}
	if got, want := capturedReq.ArtworkGenerationMode, studioArtworkGenerationModeHotReference; got != want {
		t.Fatalf("artwork generation mode = %q, want %q", got, want)
	}
	if got, want := capturedReq.ProductReferenceImageURLs, []string{"https://cdn.example.com/hot-ref.png"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("ProductReferenceImageURLs = %v, want %v", got, want)
	}
	if detail.Batch == nil {
		t.Fatal("detail.Batch = nil")
	}
	if got := detail.Batch.HotStyleReferencePrompt; got != "Create an original skull streetwear print." {
		t.Fatalf("hot style reference prompt = %q, want existing graph prompt preserved", got)
	}
}

func TestTaskStudioBatchServiceRetryItemsRetriesFailedBlockedItemAfterFix(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Date(2026, 6, 1, 11, 0, 0, 0, time.UTC)

	if err := repo.CreateStudioBatchGraph(ctx, newStudioBatchRecordForTest("batch-1", now), []StudioBatchItemRecord{
		{
			ID:               "item-1",
			BatchID:          "batch-1",
			TargetGroupKey:   "size:1200x1200",
			TargetGroupLabel: "1200 x 1200",
			Status:           StudioBatchItemStatusFailed,
			LastError:        "standard product temporal workflow is not configured",
			SelectionCount:   1,
			CreatedAt:        now.Add(-20 * time.Minute),
			UpdatedAt:        now.Add(-20 * time.Minute),
		},
	}, []StudioGenerationAttemptRecord{
		{
			ID:           "attempt-1",
			ItemID:       "item-1",
			AttemptNo:    1,
			Status:       StudioGenerationAttemptStatusFailed,
			ErrorMessage: "standard product temporal workflow is not configured",
			CreatedAt:    now.Add(-20 * time.Minute),
			UpdatedAt:    now.Add(-20 * time.Minute),
		},
	}, nil); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	svc := newTaskStudioBatchService(taskStudioBatchServiceConfig{
		repo: repo,
		generator: newStudioBatchGenerationService(studioBatchGenerationServiceConfig{
			repo: repo,
			execute: func(_ context.Context, input StudioBatchGenerateExecutionInput) (*StudioBatchGenerateExecutionOutput, error) {
				return &StudioBatchGenerateExecutionOutput{
					Response: testStudioDesignResponse("design-"+input.ItemID, "https://cdn.example.com/"+input.ItemID+".png"),
					ItemID:   input.ItemID,
					BatchID:  input.BatchID,
				}, nil
			},
			currentTime: func() time.Time { return now },
		}),
	})

	detail, err := svc.RetryStudioBatchItems(ctx, "batch-1", &RetryStudioBatchItemsRequest{
		ItemIDs: []string{"item-1"},
	})
	if err != nil {
		t.Fatalf("RetryStudioBatchItems() error = %v", err)
	}

	if len(detail.Items) != 1 {
		t.Fatalf("len(detail.Items) = %d, want 1", len(detail.Items))
	}
	if got := detail.Items[0].Item.Status; got != StudioBatchItemStatusReviewReady {
		t.Fatalf("item status = %q, want review_ready after blocked item retry", got)
	}
	if got := len(detail.Items[0].Attempts); got != 2 {
		t.Fatalf("attempt count = %d, want 2", got)
	}
	if got := detail.Items[0].Attempts[1].Status; got != StudioGenerationAttemptStatusMaterialized {
		t.Fatalf("attempt-2 status = %q, want materialized retry attempt", got)
	}
}

func TestServiceGetStudioBatchDetailReturnsDraftOnlyStateWhenSelectingBatchGraphIsMissing(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	sessionRepo := &studioBatchGenerationSessionRepoStub{
		session: &SheinStudioSession{
			ID:               "batch-1",
			SavedAsBatch:     true,
			Status:           SheinStudioSessionStatusSelecting,
			Prompt:           "retro summer fruit",
			StyleCount:       "1",
			ArtworkModel:     "gpt-image-1",
			GroupedImageMode: "shared_by_size",
			Selection:        SheinStudioSelectionSnapshot(testStudioBatchSelection(101, "Canvas Tote", "Red", 1200, 1200)),
			GroupedSelections: SheinStudioGroupedSelectionList{
				{
					SelectionID: "7001:9001:102:layer-1:102",
					Selection:   testStudioBatchSelection(102, "Canvas Tote", "Blue", 1200, 1200),
					Eligible:    true,
				},
			},
		},
	}

	svc := &service{studioDeps: studioDependencies{batchRepo: repo, sessionRepo: sessionRepo}}

	detail, err := svc.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}

	if detail.Batch == nil || detail.Batch.ID != "batch-1" {
		t.Fatalf("detail.Batch = %+v, want batch-1", detail.Batch)
	}
	if detail.Batch.Prompt != "retro summer fruit" {
		t.Fatalf("detail.Batch.Prompt = %q, want session prompt", detail.Batch.Prompt)
	}
	if got := detail.Batch.Status; got != StudioBatchStatusDraft {
		t.Fatalf("detail.Batch.Status = %q, want %q", got, StudioBatchStatusDraft)
	}
	if len(detail.Items) != 0 {
		t.Fatalf("len(detail.Items) = %d, want 0 for draft-only detail", len(detail.Items))
	}
	if _, err := repo.GetStudioBatch(ctx, "batch-1"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("GetStudioBatch() error = %v, want record not found because GET detail should stay read-only", err)
	}
}

func TestServiceGetStudioBatchDetailMaterializesBatchGraphFromGeneratingSessionWhenMissing(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	sessionRepo := &studioBatchGenerationSessionRepoStub{
		session: &SheinStudioSession{
			ID:               "batch-1",
			SavedAsBatch:     true,
			Status:           SheinStudioSessionStatusGenerating,
			Prompt:           "retro summer fruit",
			StyleCount:       "1",
			ArtworkModel:     "gpt-image-1",
			GroupedImageMode: "shared_by_size",
			Selection:        SheinStudioSelectionSnapshot(testStudioBatchSelection(101, "Canvas Tote", "Red", 1200, 1200)),
			GroupedSelections: SheinStudioGroupedSelectionList{
				{
					SelectionID: "7001:9001:102:layer-1:102",
					Selection:   testStudioBatchSelection(102, "Canvas Tote", "Blue", 1200, 1200),
					Eligible:    true,
				},
			},
		},
	}

	svc := &service{studioDeps: studioDependencies{batchRepo: repo, sessionRepo: sessionRepo}}

	detail, err := svc.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}

	if detail.Batch == nil || detail.Batch.ID != "batch-1" {
		t.Fatalf("detail.Batch = %+v, want batch-1", detail.Batch)
	}
	if len(detail.Items) != 1 {
		t.Fatalf("len(detail.Items) = %d, want 1 shared-size item", len(detail.Items))
	}
	wantGroupKey := buildStudioBatchSharedCompatibilityGroupKey(testStudioBatchSelection(101, "Canvas Tote", "Red", 1200, 1200))
	if detail.Items[0].Item.TargetGroupKey != wantGroupKey {
		t.Fatalf("detail.Items[0].Item.TargetGroupKey = %q, want %q", detail.Items[0].Item.TargetGroupKey, wantGroupKey)
	}
	if detail.Items[0].Item.SelectionCount != 2 {
		t.Fatalf("detail.Items[0].Item.SelectionCount = %d, want 2", detail.Items[0].Item.SelectionCount)
	}
}

func TestApproveStudioBatchDesignsRequestUsesDesignIDsJSONContract(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(ApproveStudioBatchDesignsRequest{
		DesignIDs: []string{"design-1", "design-2"},
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if got := string(payload); got != `{"design_ids":["design-1","design-2"]}` {
		t.Fatalf("Marshal() = %s, want design_ids contract", got)
	}
}

func TestRetryStudioBatchItemsRequestUsesItemIDsJSONContract(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(RetryStudioBatchItemsRequest{
		ItemIDs: []string{"item-1", "item-2"},
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if got := string(payload); got != `{"item_ids":["item-1","item-2"]}` {
		t.Fatalf("Marshal() = %s, want item_ids contract", got)
	}
}

func TestCreateStudioBatchTasksRequestUsesDesignIDsJSONContract(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(CreateStudioBatchTasksRequest{
		DesignIDs: []string{"design-1", "design-2"},
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if got := string(payload); got != `{"design_ids":["design-1","design-2"]}` {
		t.Fatalf("Marshal() = %s, want design_ids contract", got)
	}
}

func TestServiceApproveStudioBatchDesignsUpdatesReviewStatusFromDesignIDs(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	batch := newStudioBatchRecordForTest("batch-1", now)
	batch.GroupedSelections = SheinStudioGroupedSelectionList{
		studioBatchFanOutSelection("selection-1", 3003, "Red", "9001", "https://cdn.example.com/template.png", "https://cdn.example.com/mask.png"),
	}
	items := newStudioBatchItemsForTest("batch-1", now)
	items[0].SelectionIDs = SheinStudioStringList{"selection-1"}
	items[0].SelectionCount = 1
	if err := repo.CreateStudioBatchGraph(ctx, batch, items, newStudioBatchAttemptsForTest("item-1", now), []StudioMaterializedDesignRecord{
		{
			ID:              "design-1",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			ImageURL:        "https://cdn.example.com/design-1.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusApproved,
			SortOrder:       0,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		{
			ID:              "design-2",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			ImageURL:        "https://cdn.example.com/design-2.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusRejected,
			SortOrder:       1,
			CreatedAt:       now.Add(time.Second),
			UpdatedAt:       now.Add(time.Second),
		},
	}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	svc := &service{studioDeps: studioDependencies{batchRepo: repo}}
	detail, err := svc.ApproveStudioBatchDesigns(ctx, "batch-1", &ApproveStudioBatchDesignsRequest{
		DesignIDs: []string{"design-2"},
	})
	if err != nil {
		t.Fatalf("ApproveStudioBatchDesigns() error = %v", err)
	}

	if got := detail.Items[0].Designs[0].ReviewStatus; got != StudioMaterializedDesignReviewStatusUnreviewed {
		t.Fatalf("design-1 review status = %q, want unreviewed", got)
	}
	if got := detail.Items[0].Designs[1].ReviewStatus; got != StudioMaterializedDesignReviewStatusApproved {
		t.Fatalf("design-2 review status = %q, want approved", got)
	}
}

func TestServiceApproveStudioBatchDesignsRejectsUnknownDesignIDs(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	batch := newStudioBatchRecordForTest("batch-1", now)
	batch.GroupedSelections = SheinStudioGroupedSelectionList{
		studioBatchFanOutSelection("selection-1", 3003, "Red", "9001", "https://cdn.example.com/template.png", "https://cdn.example.com/mask.png"),
	}
	items := newStudioBatchItemsForTest("batch-1", now)
	items[0].SelectionIDs = SheinStudioStringList{"selection-1"}
	items[0].SelectionCount = 1
	if err := repo.CreateStudioBatchGraph(ctx, batch, items, newStudioBatchAttemptsForTest("item-1", now), []StudioMaterializedDesignRecord{
		{
			ID:              "design-1",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			TargetGroupKey:  "size:1200x1200",
			ImageURL:        "https://cdn.example.com/design-1.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusApproved,
			SortOrder:       0,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		{
			ID:              "design-2",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			TargetGroupKey:  "size:1200x1200",
			ImageURL:        "https://cdn.example.com/design-2.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusRejected,
			SortOrder:       1,
			CreatedAt:       now.Add(time.Second),
			UpdatedAt:       now.Add(time.Second),
		},
	}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	svc := &service{studioDeps: studioDependencies{batchRepo: repo}}
	_, err := svc.ApproveStudioBatchDesigns(ctx, "batch-1", &ApproveStudioBatchDesignsRequest{
		DesignIDs: []string{"design-2", "design-missing"},
	})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("ApproveStudioBatchDesigns() error = %v, want record not found", err)
	}

	detail, err := repo.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}
	if got := detail.DesignsByItem["item-1"][0].ReviewStatus; got != StudioMaterializedDesignReviewStatusApproved {
		t.Fatalf("design-1 stored review status = %q, want approved after rejected mutation", got)
	}
	if got := detail.DesignsByItem["item-1"][1].ReviewStatus; got != StudioMaterializedDesignReviewStatusRejected {
		t.Fatalf("design-2 stored review status = %q, want rejected after atomic failure", got)
	}
}

func TestServiceRetryStudioBatchItemsRegeneratesOnlySelectedItems(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	if err := repo.CreateStudioBatchGraph(ctx, newStudioBatchRecordForTest("batch-1", now), []StudioBatchItemRecord{
		{
			ID:               "item-1",
			BatchID:          "batch-1",
			TargetGroupKey:   "size:1200x1200",
			TargetGroupLabel: "1200 x 1200",
			Status:           StudioBatchItemStatusReviewReady,
			SelectionCount:   1,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		{
			ID:               "item-2",
			BatchID:          "batch-1",
			TargetGroupKey:   "size:2000x2000",
			TargetGroupLabel: "2000 x 2000",
			Status:           StudioBatchItemStatusReviewReady,
			SelectionCount:   1,
			CreatedAt:        now.Add(time.Second),
			UpdatedAt:        now.Add(time.Second),
		},
	}, []StudioGenerationAttemptRecord{
		{
			ID:        "attempt-1",
			ItemID:    "item-1",
			AttemptNo: 1,
			Status:    StudioGenerationAttemptStatusMaterialized,
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:        "attempt-2",
			ItemID:    "item-2",
			AttemptNo: 1,
			Status:    StudioGenerationAttemptStatusMaterialized,
			CreatedAt: now.Add(time.Second),
			UpdatedAt: now.Add(time.Second),
		},
	}, []StudioMaterializedDesignRecord{
		{
			ID:               "design-1",
			BatchID:          "batch-1",
			ItemID:           "item-1",
			SourceAttemptID:  "attempt-1",
			TargetGroupKey:   "size:1200x1200",
			TargetGroupLabel: "1200 x 1200",
			ImageURL:         "https://cdn.example.com/design-1.png",
			SortOrder:        0,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		{
			ID:               "design-2",
			BatchID:          "batch-1",
			ItemID:           "item-2",
			SourceAttemptID:  "attempt-2",
			TargetGroupKey:   "size:2000x2000",
			TargetGroupLabel: "2000 x 2000",
			ImageURL:         "https://cdn.example.com/design-2.png",
			SortOrder:        0,
			CreatedAt:        now.Add(time.Second),
			UpdatedAt:        now.Add(time.Second),
		},
	}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	svc := newTaskStudioBatchService(taskStudioBatchServiceConfig{
		repo: repo,
		generator: newStudioBatchGenerationService(studioBatchGenerationServiceConfig{
			repo: repo,
			execute: func(_ context.Context, input StudioBatchGenerateExecutionInput) (*StudioBatchGenerateExecutionOutput, error) {
				if input.ItemID != "item-2" {
					t.Fatalf("unexpected retry execution for item %q", input.ItemID)
				}
				return &StudioBatchGenerateExecutionOutput{
					BatchID: input.BatchID,
					ItemID:  input.ItemID,
					Response: &StudioDesignResponse{
						Images: []StudioGeneratedImage{{
							ID:       "design-2-retried",
							ImageURL: "https://cdn.example.com/design-2-retried.png",
						}},
					},
				}, nil
			},
			currentTime: func() time.Time { return now.Add(2 * time.Second) },
		}),
	})

	detail, err := svc.RetryStudioBatchItems(ctx, "batch-1", &RetryStudioBatchItemsRequest{
		ItemIDs: []string{"item-2"},
	})
	if err != nil {
		t.Fatalf("RetryStudioBatchItems() error = %v", err)
	}

	if len(detail.Items) != 2 {
		t.Fatalf("len(detail.Items) = %d, want 2", len(detail.Items))
	}
	if len(detail.Items[0].Attempts) != 1 || detail.Items[0].Designs[0].ID != "design-1" {
		t.Fatalf("item-1 detail = %+v, want untouched item-1 ownership", detail.Items[0])
	}
	if len(detail.Items[1].Attempts) != 2 {
		t.Fatalf("item-2 attempts = %+v, want second retry attempt", detail.Items[1].Attempts)
	}
	if len(detail.Items[1].Designs) != 1 || detail.Items[1].Designs[0].ID != "design-2-retried" {
		t.Fatalf("item-2 designs = %+v, want retried item-owned design", detail.Items[1].Designs)
	}
}

func TestServicePrepareRetryStudioBatchItemsResetsSelectedItemsWithoutRunningGeneration(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	if err := repo.CreateStudioBatchGraph(ctx, newStudioBatchRecordForTest("batch-1", now), []StudioBatchItemRecord{
		{
			ID:               "item-1",
			BatchID:          "batch-1",
			TargetGroupKey:   "size:1200x1200",
			TargetGroupLabel: "1200 x 1200",
			Status:           StudioBatchItemStatusReviewReady,
			SelectionCount:   1,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		{
			ID:               "item-2",
			BatchID:          "batch-1",
			TargetGroupKey:   "size:2000x2000",
			TargetGroupLabel: "2000 x 2000",
			Status:           StudioBatchItemStatusFailed,
			LastError:        "timed out",
			SelectionCount:   1,
			CreatedAt:        now.Add(time.Second),
			UpdatedAt:        now.Add(time.Second),
		},
	}, []StudioGenerationAttemptRecord{
		{
			ID:        "attempt-1",
			ItemID:    "item-1",
			AttemptNo: 1,
			Status:    StudioGenerationAttemptStatusMaterialized,
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:        "attempt-2",
			ItemID:    "item-2",
			AttemptNo: 1,
			Status:    StudioGenerationAttemptStatusFailed,
			CreatedAt: now.Add(time.Second),
			UpdatedAt: now.Add(time.Second),
		},
	}, []StudioMaterializedDesignRecord{
		{
			ID:               "design-1",
			BatchID:          "batch-1",
			ItemID:           "item-1",
			SourceAttemptID:  "attempt-1",
			TargetGroupKey:   "size:1200x1200",
			TargetGroupLabel: "1200 x 1200",
			ImageURL:         "https://cdn.example.com/design-1.png",
			SortOrder:        0,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
	}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	svc := newTaskStudioBatchService(taskStudioBatchServiceConfig{
		repo: repo,
		generator: newStudioBatchGenerationService(studioBatchGenerationServiceConfig{
			repo: repo,
			execute: func(_ context.Context, _ StudioBatchGenerateExecutionInput) (*StudioBatchGenerateExecutionOutput, error) {
				t.Fatal("execute should not run during prepare-only retry")
				return nil, nil
			},
		}),
	})

	detail, err := svc.PrepareRetryStudioBatchItems(ctx, "batch-1", &RetryStudioBatchItemsRequest{
		ItemIDs: []string{"item-2"},
	})
	if err != nil {
		t.Fatalf("PrepareRetryStudioBatchItems() error = %v", err)
	}

	if len(detail.Items) != 2 {
		t.Fatalf("len(detail.Items) = %d, want 2", len(detail.Items))
	}
	if got := detail.Items[1].Item.Status; got != StudioBatchItemStatusPending {
		t.Fatalf("item-2 status = %q, want pending after prepare", got)
	}
	if got := detail.Items[1].Item.LastError; got != "" {
		t.Fatalf("item-2 last error = %q, want cleared", got)
	}
	if len(detail.Items[1].Attempts) != 1 || detail.Items[1].Attempts[0].Status != StudioGenerationAttemptStatusFailed {
		t.Fatalf("item-2 attempts = %+v, want existing failed attempt preserved before async retry", detail.Items[1].Attempts)
	}
}

func TestServicePrepareRetryStudioBatchItemsResetsRelatedBatchRunState(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	runRepo := NewMemStudioBatchRunRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	if err := repo.CreateStudioBatchGraph(ctx, newStudioBatchRecordForTest("batch-1", now), []StudioBatchItemRecord{
		{
			ID:               "item-1",
			BatchID:          "batch-1",
			TargetGroupKey:   "size:2000x2000",
			TargetGroupLabel: "2000 x 2000",
			Status:           StudioBatchItemStatusFailed,
			LastError:        "timed out",
			SelectionCount:   1,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
	}, []StudioGenerationAttemptRecord{
		{
			ID:           "attempt-1",
			ItemID:       "item-1",
			AttemptNo:    1,
			Status:       StudioGenerationAttemptStatusFailed,
			ErrorMessage: "timed out",
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}, nil); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	run, items := mustCreateStudioBatchRunForTest(t, runRepo, ctx, "run-1", []string{"batch-1"})
	items[0].Status = StudioBatchRunItemStatusFailed
	items[0].AsyncJobID = "job-old-1"
	items[0].ErrorMessage = "timed out"
	items[0].StartedAt = timePtr(now.Add(-2 * time.Minute))
	items[0].FinishedAt = timePtr(now.Add(-time.Minute))
	if err := runRepo.UpdateStudioBatchRunItem(ctx, &items[0]); err != nil {
		t.Fatalf("UpdateStudioBatchRunItem() error = %v", err)
	}
	run.Status = StudioBatchRunStatusFailed
	run.FailedBatches = 1
	run.CompletedBatches = 1
	run.CancelRequested = true
	run.LastError = "timed out"
	run.StartedAt = timePtr(now.Add(-3 * time.Minute))
	run.FinishedAt = timePtr(now.Add(-time.Minute))
	if err := runRepo.UpdateStudioBatchRun(ctx, run); err != nil {
		t.Fatalf("UpdateStudioBatchRun() error = %v", err)
	}

	svc := newTaskStudioBatchService(taskStudioBatchServiceConfig{
		repo:         repo,
		batchRunRepo: runRepo,
		generator: newStudioBatchGenerationService(studioBatchGenerationServiceConfig{
			repo: repo,
			execute: func(_ context.Context, _ StudioBatchGenerateExecutionInput) (*StudioBatchGenerateExecutionOutput, error) {
				t.Fatal("execute should not run during prepare-only retry")
				return nil, nil
			},
		}),
	})

	if _, err := svc.PrepareRetryStudioBatchItems(ctx, "batch-1", &RetryStudioBatchItemsRequest{
		ItemIDs: []string{"item-1"},
	}); err != nil {
		t.Fatalf("PrepareRetryStudioBatchItems() error = %v", err)
	}

	gotRun, err := runRepo.GetStudioBatchRun(ctx, "run-1")
	if err != nil {
		t.Fatalf("GetStudioBatchRun() error = %v", err)
	}
	if gotRun.Status != StudioBatchRunStatusPending {
		t.Fatalf("run status = %q, want pending", gotRun.Status)
	}
	if gotRun.CompletedBatches != 0 {
		t.Fatalf("run completed batches = %d, want 0", gotRun.CompletedBatches)
	}
	if gotRun.FailedBatches != 0 {
		t.Fatalf("run failed batches = %d, want 0", gotRun.FailedBatches)
	}
	if gotRun.LastError != "" {
		t.Fatalf("run last error = %q, want cleared", gotRun.LastError)
	}
	if gotRun.CancelRequested {
		t.Fatal("run cancel requested = true, want cleared")
	}
	if gotRun.StartedAt != nil {
		t.Fatalf("run started at = %v, want nil", gotRun.StartedAt)
	}
	if gotRun.FinishedAt != nil {
		t.Fatalf("run finished at = %v, want nil", gotRun.FinishedAt)
	}

	gotItems, err := runRepo.ListStudioBatchRunItems(ctx, "run-1")
	if err != nil {
		t.Fatalf("ListStudioBatchRunItems() error = %v", err)
	}
	if gotItems[0].Status != StudioBatchRunItemStatusPending {
		t.Fatalf("run item status = %q, want pending", gotItems[0].Status)
	}
	if gotItems[0].AsyncJobID != "" {
		t.Fatalf("run item async job id = %q, want cleared", gotItems[0].AsyncJobID)
	}
	if gotItems[0].ErrorMessage != "" {
		t.Fatalf("run item error message = %q, want cleared", gotItems[0].ErrorMessage)
	}
	if gotItems[0].StartedAt != nil || gotItems[0].FinishedAt != nil {
		t.Fatalf("run item timestamps = start:%v finish:%v, want nil", gotItems[0].StartedAt, gotItems[0].FinishedAt)
	}
}

func TestRetryStudioBatchItems_RejectsItemWithCreatedTaskLinks(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	linkRepo := NewMemStudioBatchTaskLinkRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	if err := repo.CreateStudioBatchGraph(ctx, newStudioBatchRecordForTest("batch-1", now), []StudioBatchItemRecord{
		{
			ID:               "item-1",
			BatchID:          "batch-1",
			TargetGroupKey:   "size:1200x1200",
			TargetGroupLabel: "1200 x 1200",
			Status:           StudioBatchItemStatusFailed,
			LastError:        "previous generation failed after task creation",
			SelectionCount:   1,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
	}, []StudioGenerationAttemptRecord{
		{
			ID:           "attempt-1",
			ItemID:       "item-1",
			AttemptNo:    1,
			Status:       StudioGenerationAttemptStatusFailed,
			ErrorMessage: "previous generation failed after task creation",
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}, []StudioMaterializedDesignRecord{
		{
			ID:              "design-1",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			ImageURL:        "https://cdn.example.com/design-1.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusApproved,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}
	mustCreateStudioBatchTaskLinkForTest(t, linkRepo, ctx, &StudioBatchTaskLinkRecord{
		ID:               "link-1",
		BatchID:          "batch-1",
		ItemID:           "item-1",
		DesignID:         "design-1",
		SelectionID:      "selection-1",
		ListingKitTaskID: "task-1",
		CandidateKey:     "candidate-1",
		Status:           studioBatchTaskLinkStatusCreated,
		CreatedAt:        now,
		UpdatedAt:        now,
	})

	svc := newTaskStudioBatchService(taskStudioBatchServiceConfig{
		repo:              repo,
		batchTaskLinkRepo: linkRepo,
		generator: newStudioBatchGenerationService(studioBatchGenerationServiceConfig{
			repo: repo,
			execute: func(_ context.Context, _ StudioBatchGenerateExecutionInput) (*StudioBatchGenerateExecutionOutput, error) {
				t.Fatal("execute should not run when retry item has created task links")
				return nil, nil
			},
		}),
	})

	_, err := svc.RetryStudioBatchItems(ctx, "batch-1", &RetryStudioBatchItemsRequest{
		ItemIDs: []string{"item-1"},
	})
	if !errors.Is(err, ErrStudioBatchActionValidation) {
		t.Fatalf("RetryStudioBatchItems() error = %v, want validation error", err)
	}
	if !strings.Contains(err.Error(), "tasks_already_created") {
		t.Fatalf("RetryStudioBatchItems() error = %v, want tasks_already_created", err)
	}

	detail, err := repo.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}
	if got := detail.Items[0].Status; got != StudioBatchItemStatusFailed {
		t.Fatalf("item status = %q, want failed after rejected retry", got)
	}
	if got := detail.DesignsByItem["item-1"]; len(got) != 1 || got[0].ID != "design-1" {
		t.Fatalf("item designs = %+v, want original task-linked design preserved", got)
	}
}

func TestRetryStudioBatchItems_AllowsFailedItemWithoutTaskLinks(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	linkRepo := NewMemStudioBatchTaskLinkRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	batch := newStudioBatchRecordForTest("batch-1", now)
	batch.Status = StudioBatchStatusFailed
	if err := repo.CreateStudioBatchGraph(ctx, batch, []StudioBatchItemRecord{
		{
			ID:               "item-1",
			BatchID:          "batch-1",
			TargetGroupKey:   "size:1200x1200",
			TargetGroupLabel: "1200 x 1200",
			Status:           StudioBatchItemStatusFailed,
			LastError:        "timed out",
			SelectionCount:   1,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
	}, []StudioGenerationAttemptRecord{
		{
			ID:           "attempt-1",
			ItemID:       "item-1",
			AttemptNo:    1,
			Status:       StudioGenerationAttemptStatusFailed,
			ErrorMessage: "timed out",
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}, nil); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	svc := newTaskStudioBatchService(taskStudioBatchServiceConfig{
		repo:              repo,
		batchTaskLinkRepo: linkRepo,
		generator: newStudioBatchGenerationService(studioBatchGenerationServiceConfig{
			repo: repo,
			execute: func(_ context.Context, input StudioBatchGenerateExecutionInput) (*StudioBatchGenerateExecutionOutput, error) {
				return &StudioBatchGenerateExecutionOutput{
					BatchID: input.BatchID,
					ItemID:  input.ItemID,
					Response: &StudioDesignResponse{
						Images: []StudioGeneratedImage{{
							ID:       "design-1-retry",
							ImageURL: "https://cdn.example.com/design-1-retry.png",
						}},
					},
				}, nil
			},
			currentTime: func() time.Time { return now.Add(time.Second) },
		}),
	})

	detail, err := svc.RetryStudioBatchItems(ctx, "batch-1", &RetryStudioBatchItemsRequest{
		ItemIDs: []string{"item-1"},
	})
	if err != nil {
		t.Fatalf("RetryStudioBatchItems() error = %v", err)
	}
	if len(detail.Items) != 1 {
		t.Fatalf("len(detail.Items) = %d, want 1", len(detail.Items))
	}
	if got := detail.Items[0].Item.Status; got != StudioBatchItemStatusReviewReady {
		t.Fatalf("item status = %q, want review_ready after retry", got)
	}
	if got := detail.Items[0].Designs; len(got) != 1 || got[0].ID != "design-1-retry" {
		t.Fatalf("item designs = %+v, want retry design", got)
	}
}

func TestServiceRetryStudioBatchItemsRefreshesLatestDraftPromptBeforeRunning(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	if err := repo.CreateStudioBatchGraph(ctx, &StudioBatchRecord{
		ID:                    "batch-1",
		Status:                StudioBatchStatusFailed,
		Prompt:                "",
		StyleCount:            "1",
		VariationIntensity:    "medium",
		GroupedImageMode:      "shared_by_size",
		Selection:             SheinStudioSelectionSnapshot(testStudioBatchSelection(101, "Canvas Tote", "Red", 1200, 1200)),
		GroupedSelections:     SheinStudioGroupedSelectionList{{SelectionID: "7001:9001:101:layer-1:101", Selection: testStudioBatchSelection(101, "Canvas Tote", "Red", 1200, 1200), Eligible: true}},
		TransparentBackground: false,
		CreatedAt:             now,
		UpdatedAt:             now,
	}, []StudioBatchItemRecord{
		{
			ID:               "item-1",
			BatchID:          "batch-1",
			TargetGroupKey:   "size:1200x1200",
			TargetGroupLabel: "1200 x 1200",
			Status:           StudioBatchItemStatusFailed,
			LastError:        "invalid request: prompt is required",
			SelectionIDs:     []string{"7001:9001:101:layer-1:101"},
			SelectionCount:   1,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
	}, []StudioGenerationAttemptRecord{
		{
			ID:           "attempt-1",
			ItemID:       "item-1",
			AttemptNo:    1,
			Status:       StudioGenerationAttemptStatusFailed,
			ErrorMessage: "invalid request: prompt is required",
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}, nil); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	sessionRepo := &studioBatchGenerationSessionRepoStub{
		session: &SheinStudioSession{
			ID:                 "batch-1",
			SavedAsBatch:       true,
			Status:             SheinStudioSessionStatusSelecting,
			Prompt:             "fresh prompt from draft",
			StyleCount:         "1",
			VariationIntensity: "medium",
			GroupedImageMode:   "shared_by_size",
			Selection:          SheinStudioSelectionSnapshot(testStudioBatchSelection(101, "Canvas Tote", "Red", 1200, 1200)),
			GroupedSelections: SheinStudioGroupedSelectionList{
				{
					SelectionID: "7001:9001:101:layer-1:101",
					Selection:   testStudioBatchSelection(101, "Canvas Tote", "Red", 1200, 1200),
					Eligible:    true,
				},
			},
		},
	}

	var prompts []string
	svc := newTaskStudioBatchService(taskStudioBatchServiceConfig{
		repo:              repo,
		studioSessionRepo: sessionRepo,
		generator: newStudioBatchGenerationService(studioBatchGenerationServiceConfig{
			repo: repo,
			execute: func(_ context.Context, input StudioBatchGenerateExecutionInput) (*StudioBatchGenerateExecutionOutput, error) {
				prompts = append(prompts, input.Request.Prompt)
				return &StudioBatchGenerateExecutionOutput{
					BatchID: input.BatchID,
					ItemID:  input.ItemID,
					Response: &StudioDesignResponse{
						Images: []StudioGeneratedImage{{
							ID:       "design-1",
							ImageURL: "https://cdn.example.com/design-1.png",
						}},
					},
				}, nil
			},
			currentTime: func() time.Time { return now.Add(time.Second) },
		}),
	})

	detail, err := svc.RetryStudioBatchItems(ctx, "batch-1", &RetryStudioBatchItemsRequest{
		ItemIDs: []string{"item-1"},
	})
	if err != nil {
		t.Fatalf("RetryStudioBatchItems() error = %v", err)
	}

	if len(prompts) != 1 {
		t.Fatalf("len(prompts) = %d, want 1", len(prompts))
	}
	if got := prompts[0]; got != "fresh prompt from draft" {
		t.Fatalf("execution prompt = %q, want refreshed draft prompt", got)
	}
	if detail.Batch == nil || detail.Batch.Prompt != "fresh prompt from draft" {
		t.Fatalf("detail.Batch = %+v, want refreshed draft prompt", detail.Batch)
	}
}

func TestServiceRetryStudioBatchItemsRejectsMixedValidAndUnknownItemIDsAtomically(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	if err := repo.CreateStudioBatchGraph(ctx, newStudioBatchRecordForTest("batch-1", now), []StudioBatchItemRecord{
		{
			ID:               "item-1",
			BatchID:          "batch-1",
			TargetGroupKey:   "size:1200x1200",
			TargetGroupLabel: "1200 x 1200",
			Status:           StudioBatchItemStatusReviewReady,
			SelectionCount:   1,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
	}, []StudioGenerationAttemptRecord{
		{
			ID:        "attempt-1",
			ItemID:    "item-1",
			AttemptNo: 1,
			Status:    StudioGenerationAttemptStatusMaterialized,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, []StudioMaterializedDesignRecord{
		{
			ID:               "design-1",
			BatchID:          "batch-1",
			ItemID:           "item-1",
			SourceAttemptID:  "attempt-1",
			TargetGroupKey:   "size:1200x1200",
			TargetGroupLabel: "1200 x 1200",
			ImageURL:         "https://cdn.example.com/design-1.png",
			SortOrder:        0,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
	}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	svc := newTaskStudioBatchService(taskStudioBatchServiceConfig{
		repo: repo,
		generator: newStudioBatchGenerationService(studioBatchGenerationServiceConfig{
			repo: repo,
			execute: func(_ context.Context, _ StudioBatchGenerateExecutionInput) (*StudioBatchGenerateExecutionOutput, error) {
				t.Fatal("execute should not run when retry validation fails")
				return nil, nil
			},
		}),
	})

	_, err := svc.RetryStudioBatchItems(ctx, "batch-1", &RetryStudioBatchItemsRequest{
		ItemIDs: []string{"item-1", "missing"},
	})
	if !errors.Is(err, ErrStudioBatchActionValidation) {
		t.Fatalf("RetryStudioBatchItems() error = %v, want validation error", err)
	}

	detail, err := repo.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}
	if got := detail.Items[0].Status; got != StudioBatchItemStatusReviewReady {
		t.Fatalf("item status = %q, want review_ready after atomic validation failure", got)
	}
	if len(detail.AttemptsByItem["item-1"]) != 1 {
		t.Fatalf("attempts = %+v, want original attempts preserved", detail.AttemptsByItem["item-1"])
	}
}

func TestServiceRetryStudioBatchItemsRejectsActiveItemStates(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	if err := repo.CreateStudioBatchGraph(ctx, newStudioBatchRecordForTest("batch-1", now), []StudioBatchItemRecord{
		{
			ID:               "item-1",
			BatchID:          "batch-1",
			TargetGroupKey:   "size:1200x1200",
			TargetGroupLabel: "1200 x 1200",
			Status:           StudioBatchItemStatusGenerating,
			SelectionCount:   1,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
	}, []StudioGenerationAttemptRecord{
		{
			ID:        "attempt-1",
			ItemID:    "item-1",
			AttemptNo: 1,
			Status:    StudioGenerationAttemptStatusRunning,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	svc := newTaskStudioBatchService(taskStudioBatchServiceConfig{
		repo: repo,
		generator: newStudioBatchGenerationService(studioBatchGenerationServiceConfig{
			repo: repo,
			execute: func(_ context.Context, _ StudioBatchGenerateExecutionInput) (*StudioBatchGenerateExecutionOutput, error) {
				t.Fatal("execute should not run for active retry states")
				return nil, nil
			},
		}),
	})

	_, err := svc.RetryStudioBatchItems(ctx, "batch-1", &RetryStudioBatchItemsRequest{
		ItemIDs: []string{"item-1"},
	})
	if !errors.Is(err, ErrStudioBatchActionValidation) {
		t.Fatalf("RetryStudioBatchItems() error = %v, want validation error", err)
	}

	detail, err := repo.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}
	if got := detail.Items[0].Status; got != StudioBatchItemStatusGenerating {
		t.Fatalf("item status = %q, want generating to remain unchanged", got)
	}
	if len(detail.AttemptsByItem["item-1"]) != 1 || detail.AttemptsByItem["item-1"][0].Status != StudioGenerationAttemptStatusRunning {
		t.Fatalf("attempts = %+v, want running attempt unchanged", detail.AttemptsByItem["item-1"])
	}
}

func TestServiceCreateStudioBatchTasksUsesApprovedDesignOwnership(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	batch := newStudioBatchRecordForTest("batch-1", now)
	batch.GroupedSelections = SheinStudioGroupedSelectionList{
		studioBatchFanOutSelection("selection-1", 3003, "Red", "9001", "https://cdn.example.com/template.png", "https://cdn.example.com/mask.png"),
	}
	items := newStudioBatchItemsForTest("batch-1", now)
	items[0].SelectionIDs = SheinStudioStringList{"selection-1"}
	items[0].SelectionCount = 1
	if err := repo.CreateStudioBatchGraph(ctx, batch, items, newStudioBatchAttemptsForTest("item-1", now), []StudioMaterializedDesignRecord{
		{
			ID:              "design-1",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			ImageURL:        "https://cdn.example.com/design-1.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusApproved,
			SortOrder:       0,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		{
			ID:              "design-2",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			ImageURL:        "https://cdn.example.com/design-2.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusUnreviewed,
			SortOrder:       1,
			CreatedAt:       now.Add(time.Second),
			UpdatedAt:       now.Add(time.Second),
		},
	}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	svc := &service{studioDeps: studioDependencies{batchRepo: repo}}
	svc.repo = newStudioBatchTaskRepositoryStub()
	svc.taskDeps.taskSubmitter = &studioBatchTaskSubmitterStub{}
	svc.studioDeps.sessionRepo = &studioBatchTaskCreationSessionRepoStub{
		session: &SheinStudioSession{
			ID:            "batch-1",
			Prompt:        "retro cherries",
			ImageStrategy: "sds_official",
			Selection: SheinStudioSelectionSnapshot{
				ProductID:        1001,
				ParentProductID:  2002,
				VariantID:        3003,
				PrototypeGroupID: 4004,
				LayerID:          "layer-1",
				ProductName:      "Canvas Tote",
				PrintableWidth:   1200,
				PrintableHeight:  1200,
			},
		},
	}
	result, err := svc.CreateStudioBatchTasks(ctx, "batch-1", &CreateStudioBatchTasksRequest{
		DesignIDs: []string{"design-1"},
	})
	if err != nil {
		t.Fatalf("CreateStudioBatchTasks() error = %v", err)
	}

	if result.Batch == nil || result.Batch.Status != StudioBatchStatusTasksCreated {
		t.Fatalf("result.Batch = %+v, want tasks_created batch", result.Batch)
	}
	if len(result.CreatedTasks) != 1 {
		t.Fatalf("created tasks = %+v, want 1", result.CreatedTasks)
	}
	if result.CreatedTasks[0].DesignID != "design-1" {
		t.Fatalf("created task = %+v, want approved design ownership", result.CreatedTasks[0])
	}
	if _, err := svc.repo.GetTask(ctx, result.CreatedTasks[0].ID); err != nil {
		t.Fatalf("persisted listing task = %v, want created task in task repo", err)
	}

	detail, err := repo.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}
	if detail.Batch == nil || detail.Batch.Status != StudioBatchStatusTasksCreated {
		t.Fatalf("persisted batch = %+v, want tasks_created", detail.Batch)
	}
}

func TestServiceCreateStudioBatchTasksDoesNotCompleteBatchForPartialApprovedDesigns(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	batch := newStudioBatchRecordForTest("batch-1", now)
	batch.Status = StudioBatchStatusReviewReady
	batch.GroupedSelections = SheinStudioGroupedSelectionList{
		studioBatchFanOutSelection("selection-1", 3003, "Red", "9001", "https://cdn.example.com/template.png", "https://cdn.example.com/mask.png"),
	}
	items := newStudioBatchItemsForTest("batch-1", now)
	items[0].SelectionIDs = SheinStudioStringList{"selection-1"}
	items[0].SelectionCount = 1
	if err := repo.CreateStudioBatchGraph(ctx, batch, items, newStudioBatchAttemptsForTest("item-1", now), []StudioMaterializedDesignRecord{
		{
			ID:              "design-1",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			ImageURL:        "https://cdn.example.com/design-1.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusApproved,
			SortOrder:       0,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		{
			ID:              "design-2",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			ImageURL:        "https://cdn.example.com/design-2.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusApproved,
			SortOrder:       1,
			CreatedAt:       now.Add(time.Second),
			UpdatedAt:       now.Add(time.Second),
		},
	}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	svc := &service{studioDeps: studioDependencies{batchRepo: repo}}
	svc.repo = newStudioBatchTaskRepositoryStub()
	svc.taskDeps.taskSubmitter = &studioBatchTaskSubmitterStub{}
	svc.studioDeps.sessionRepo = &studioBatchTaskCreationSessionRepoStub{
		session: &SheinStudioSession{
			ID:            "batch-1",
			Prompt:        "retro cherries",
			ImageStrategy: "sds_official",
			Selection: SheinStudioSelectionSnapshot{
				ProductID:        1001,
				ParentProductID:  2002,
				VariantID:        3003,
				PrototypeGroupID: 4004,
				LayerID:          "layer-1",
				ProductName:      "Canvas Tote",
				PrintableWidth:   1200,
				PrintableHeight:  1200,
			},
		},
	}

	result, err := svc.CreateStudioBatchTasks(ctx, "batch-1", &CreateStudioBatchTasksRequest{
		DesignIDs: []string{"design-1"},
	})
	if err != nil {
		t.Fatalf("CreateStudioBatchTasks() error = %v", err)
	}
	if len(result.CreatedTasks) != 1 || result.CreatedTasks[0].DesignID != "design-1" {
		t.Fatalf("created tasks = %+v, want one design-1 task", result.CreatedTasks)
	}
	if result.Batch == nil || result.Batch.Status == StudioBatchStatusTasksCreated {
		t.Fatalf("result.Batch = %+v, want not tasks_created until every approved design is requested", result.Batch)
	}

	detail, err := repo.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}
	if detail.Batch == nil || detail.Batch.Status == StudioBatchStatusTasksCreated {
		t.Fatalf("persisted batch = %+v, want not tasks_created until every approved design is requested", detail.Batch)
	}
}

func TestServiceResumeStudioBatchTaskCreationDoesNotFinalizePartialRequest(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()
	batch := newStudioBatchRecordForTest("batch-1", now)
	items := newStudioBatchItemsForTest("batch-1", now)
	if err := repo.CreateStudioBatchGraph(ctx, batch, items, newStudioBatchAttemptsForTest("item-1", now), []StudioMaterializedDesignRecord{
		{
			ID:              "design-1",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			ImageURL:        "https://cdn.example.com/design-1.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusApproved,
			SortOrder:       0,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		{
			ID:              "design-2",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			ImageURL:        "https://cdn.example.com/design-2.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusApproved,
			SortOrder:       1,
			CreatedAt:       now.Add(time.Second),
			UpdatedAt:       now.Add(time.Second),
		},
	}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	sessionRepo := &studioBatchTaskCreationSessionRepoStub{
		session: &SheinStudioSession{
			ID:            "batch-1",
			Prompt:        "retro cherries",
			ImageStrategy: "sds_official",
			Selection: SheinStudioSelectionSnapshot{
				ProductID:        1001,
				ParentProductID:  2002,
				VariantID:        3003,
				PrototypeGroupID: 4004,
				LayerID:          "layer-1",
				ProductName:      "Canvas Tote",
				PrintableWidth:   1200,
				PrintableHeight:  1200,
			},
		},
	}
	svc := &service{studioDeps: studioDependencies{batchRepo: repo, sessionRepo: sessionRepo}}
	svc.repo = newStudioBatchTaskRepositoryStub()
	svc.taskDeps.taskSubmitter = &studioBatchTaskSubmitterStub{}

	if _, err := svc.PrepareCreateStudioBatchTasks(ctx, "batch-1", &CreateStudioBatchTasksRequest{
		DesignIDs: []string{"design-1"},
	}); err != nil {
		t.Fatalf("PrepareCreateStudioBatchTasks() error = %v", err)
	}
	if sessionRepo.session.Status != SheinStudioSessionStatusTasksCreating {
		t.Fatalf("prepared session status = %q, want tasks_creating", sessionRepo.session.Status)
	}

	detail, err := svc.ResumeStudioBatchGeneration(ctx, "batch-1")
	if err != nil {
		t.Fatalf("ResumeStudioBatchGeneration() error = %v", err)
	}
	if detail.Batch == nil || detail.Batch.Status == StudioBatchStatusTasksCreated {
		t.Fatalf("resumed batch = %+v, want not tasks_created for partial request", detail.Batch)
	}
	if sessionRepo.session.Status == SheinStudioSessionStatusTasksCreated {
		t.Fatalf("session status = %q, want not tasks_created for partial request", sessionRepo.session.Status)
	}
	if len(sessionRepo.session.PendingTaskDesignIDs) != 0 {
		t.Fatalf("pending task design ids = %+v, want cleared after resume", sessionRepo.session.PendingTaskDesignIDs)
	}
}

func TestServiceCreateStudioBatchTasksRejectsInFlightBatchWithoutPartialAllowance(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()
	batch := newStudioBatchRecordForTest("batch-1", now)
	batch.Status = StudioBatchStatusGenerating
	batch.GroupedSelections = SheinStudioGroupedSelectionList{
		studioBatchFanOutSelection("selection-1", 3003, "Red", "9001", "https://cdn.example.com/template.png", "https://cdn.example.com/mask.png"),
	}
	items := []StudioBatchItemRecord{
		{
			ID:               "item-1",
			BatchID:          "batch-1",
			TargetGroupKey:   "size:1200x1200",
			TargetGroupLabel: "1200 x 1200",
			SelectionIDs:     SheinStudioStringList{"selection-1"},
			Status:           StudioBatchItemStatusReviewReady,
			SelectionCount:   1,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		{
			ID:               "item-2",
			BatchID:          "batch-1",
			TargetGroupKey:   "size:1600x1600",
			TargetGroupLabel: "1600 x 1600",
			Status:           StudioBatchItemStatusGenerating,
			SelectionCount:   1,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
	}
	if err := repo.CreateStudioBatchGraph(ctx, batch, items, newStudioBatchAttemptsForTest("item-1", now), []StudioMaterializedDesignRecord{
		{
			ID:              "design-1",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			ImageURL:        "https://cdn.example.com/design-1.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusApproved,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	taskRepo := newStudioBatchTaskRepositoryStub()
	submitter := &studioBatchTaskSubmitterStub{}
	svc := &service{studioDeps: studioDependencies{batchRepo: repo}}
	svc.repo = taskRepo
	svc.taskDeps.taskSubmitter = submitter
	svc.studioDeps.sessionRepo = &studioBatchTaskCreationSessionRepoStub{
		session: &SheinStudioSession{
			ID:            "batch-1",
			Prompt:        "retro cherries",
			ImageStrategy: "sds_official",
			Selection: SheinStudioSelectionSnapshot{
				ProductID:       1001,
				ParentProductID: 2002,
				VariantID:       3003,
			},
		},
	}

	_, err := svc.CreateStudioBatchTasks(ctx, "batch-1", &CreateStudioBatchTasksRequest{
		DesignIDs: []string{"design-1"},
	})
	if !errors.Is(err, ErrStudioBatchActionValidation) {
		t.Fatalf("CreateStudioBatchTasks() error = %v, want validation error", err)
	}
	if got := len(taskRepo.tasks); got != 0 {
		t.Fatalf("created task count = %d, want 0", got)
	}
	if submitter.submitCount != 0 {
		t.Fatalf("submit count = %d, want 0", submitter.submitCount)
	}
}

func TestServiceCreateStudioBatchTasksAllowsExplicitPartialCreationWhileGenerating(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()
	batch := newStudioBatchRecordForTest("batch-1", now)
	batch.Status = StudioBatchStatusGenerating
	batch.GroupedSelections = SheinStudioGroupedSelectionList{
		studioBatchFanOutSelection("selection-1", 3003, "Red", "9001", "https://cdn.example.com/template.png", "https://cdn.example.com/mask.png"),
	}
	items := []StudioBatchItemRecord{
		{
			ID:               "item-1",
			BatchID:          "batch-1",
			TargetGroupKey:   "size:1200x1200",
			TargetGroupLabel: "1200 x 1200",
			SelectionIDs:     SheinStudioStringList{"selection-1"},
			Status:           StudioBatchItemStatusReviewReady,
			SelectionCount:   1,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		{
			ID:               "item-2",
			BatchID:          "batch-1",
			TargetGroupKey:   "size:1600x1600",
			TargetGroupLabel: "1600 x 1600",
			Status:           StudioBatchItemStatusGenerating,
			SelectionCount:   1,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
	}
	if err := repo.CreateStudioBatchGraph(ctx, batch, items, newStudioBatchAttemptsForTest("item-1", now), []StudioMaterializedDesignRecord{
		{
			ID:              "design-1",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			ImageURL:        "https://cdn.example.com/design-1.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusApproved,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	taskRepo := newStudioBatchTaskRepositoryStub()
	svc := &service{studioDeps: studioDependencies{batchRepo: repo}}
	svc.repo = taskRepo
	svc.taskDeps.taskSubmitter = &studioBatchTaskSubmitterStub{}
	svc.studioDeps.sessionRepo = &studioBatchTaskCreationSessionRepoStub{
		session: &SheinStudioSession{
			ID:            "batch-1",
			Prompt:        "retro cherries",
			ImageStrategy: "sds_official",
			Selection: SheinStudioSelectionSnapshot{
				ProductID:       1001,
				ParentProductID: 2002,
				VariantID:       3003,
			},
		},
	}

	result, err := svc.CreateStudioBatchTasks(ctx, "batch-1", &CreateStudioBatchTasksRequest{
		DesignIDs:                   []string{"design-1"},
		AllowPartialWhileGenerating: true,
	})
	if err != nil {
		t.Fatalf("CreateStudioBatchTasks() error = %v", err)
	}
	if len(result.CreatedTasks) != 1 || result.CreatedTasks[0].DesignID != "design-1" {
		t.Fatalf("created tasks = %+v, want design-1 task", result.CreatedTasks)
	}
}

func TestServiceCreateStudioBatchTasks_UsesBatchGraphWithoutSession(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	taskRepo := newStudioBatchTaskRepositoryStub()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	batch := newStudioBatchRecordForTest("batch-1", now)
	batch.GroupedSelections = SheinStudioGroupedSelectionList{
		studioBatchFanOutSelection("selection-1", 3003, "Red", "9001", "https://cdn.example.com/template.png", "https://cdn.example.com/mask.png"),
	}
	items := newStudioBatchItemsForTest("batch-1", now)
	items[0].SelectionIDs = SheinStudioStringList{"selection-1"}
	items[0].SelectionCount = 1
	if err := repo.CreateStudioBatchGraph(ctx, batch, items, newStudioBatchAttemptsForTest("item-1", now), []StudioMaterializedDesignRecord{
		{
			ID:              "design-1",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			ImageURL:        "https://cdn.example.com/design-1.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusApproved,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	svc := &service{studioDeps: studioDependencies{batchRepo: repo}}
	svc.repo = taskRepo
	svc.taskDeps.taskSubmitter = &studioBatchTaskSubmitterStub{}
	svc.studioDeps.sessionRepo = nil

	result, err := svc.CreateStudioBatchTasks(ctx, "batch-1", &CreateStudioBatchTasksRequest{
		DesignIDs: []string{"design-1"},
	})
	if err != nil {
		t.Fatalf("CreateStudioBatchTasks() error = %v", err)
	}
	if len(result.CreatedTasks) != 1 {
		t.Fatalf("created tasks = %+v, want 1", result.CreatedTasks)
	}
}

func TestServiceCreateStudioBatchTasks_FansOutEachDesignToEveryCompatibleSelection(t *testing.T) {
	t.Parallel()

	batchRepo := NewMemStudioBatchRepository()
	taskRepo := newStudioBatchTaskRepositoryStub()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	batch := newStudioBatchRecordForTest("batch-1", now)
	batch.GroupedImageMode = "shared_by_size"
	batch.GroupedSelections = SheinStudioGroupedSelectionList{
		studioBatchFanOutSelection("selection-1", 3001, "Red", "870", "https://cdn.example.com/template.png", "https://cdn.example.com/mask.png"),
		studioBatchFanOutSelection("selection-2", 3002, "Blue", "871", "https://cdn.example.com/template.png", "https://cdn.example.com/mask.png"),
		studioBatchFanOutSelection("selection-3", 3003, "Green", "872", "https://cdn.example.com/template.png", "https://cdn.example.com/mask.png"),
	}
	items := []StudioBatchItemRecord{{
		ID:               "item-1",
		BatchID:          "batch-1",
		TargetGroupKey:   "size:1200x1200",
		TargetGroupLabel: "1200 x 1200",
		SelectionIDs:     SheinStudioStringList{"selection-1", "selection-2", "selection-3"},
		GroupMode:        "shared_by_size",
		Status:           StudioBatchItemStatusReviewReady,
		SelectionCount:   3,
		CreatedAt:        now,
		UpdatedAt:        now,
	}}
	designs := []StudioMaterializedDesignRecord{
		{
			ID:              "design-1",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			ImageURL:        "https://cdn.example.com/design-1.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusApproved,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		{
			ID:              "design-2",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			ImageURL:        "https://cdn.example.com/design-2.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusApproved,
			CreatedAt:       now.Add(time.Second),
			UpdatedAt:       now.Add(time.Second),
		},
	}
	if err := batchRepo.CreateStudioBatchGraph(ctx, batch, items, nil, designs); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	svc := &service{repo: taskRepo, studioDeps: studioDependencies{batchRepo: batchRepo}}
	svc.taskDeps.taskSubmitter = &studioBatchTaskSubmitterStub{}

	result, err := svc.CreateStudioBatchTasks(ctx, "batch-1", &CreateStudioBatchTasksRequest{
		DesignIDs: []string{"design-1", "design-2"},
	})
	if err != nil {
		t.Fatalf("CreateStudioBatchTasks() error = %v", err)
	}
	if len(result.CreatedTasks) != 6 {
		t.Fatalf("created tasks = %+v, want 6", result.CreatedTasks)
	}
	if len(result.RejectedTasks) != 0 {
		t.Fatalf("rejected tasks = %+v, want none", result.RejectedTasks)
	}

	gotTuples := make(map[string]struct{}, len(result.CreatedTasks))
	for _, task := range result.CreatedTasks {
		if task.ItemID != "item-1" {
			t.Fatalf("created task item id = %q, want item-1 in %+v", task.ItemID, task)
		}
		if task.SelectionID == "" {
			t.Fatalf("created task = %+v, want selection_id", task)
		}
		gotTuples[task.DesignID+":"+task.SelectionID] = struct{}{}
		persisted, err := taskRepo.GetTask(ctx, task.ID)
		if err != nil {
			t.Fatalf("GetTask(%q) error = %v", task.ID, err)
		}
		if persisted.Request == nil || persisted.Request.Options == nil || persisted.Request.Options.SDS == nil {
			t.Fatalf("persisted task request = %+v, want SDS metadata", persisted.Request)
		}
	}

	wantTuples := []string{
		"design-1:selection-1",
		"design-1:selection-2",
		"design-1:selection-3",
		"design-2:selection-1",
		"design-2:selection-2",
		"design-2:selection-3",
	}
	for _, tuple := range wantTuples {
		if _, ok := gotTuples[tuple]; !ok {
			t.Fatalf("created design/selection tuples = %v, missing %s", gotTuples, tuple)
		}
	}
}

func TestServiceCreateStudioBatchTasks_FansOutHonorsRequestDesignOrder(t *testing.T) {
	t.Parallel()

	batchRepo := NewMemStudioBatchRepository()
	taskRepo := newStudioBatchTaskRepositoryStub()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	batch := newStudioBatchRecordForTest("batch-1", now)
	batch.GroupedImageMode = "shared_by_size"
	batch.GroupedSelections = SheinStudioGroupedSelectionList{
		studioBatchFanOutSelection("selection-1", 3001, "Red", "870", "https://cdn.example.com/template.png", "https://cdn.example.com/mask.png"),
		studioBatchFanOutSelection("selection-2", 3002, "Blue", "871", "https://cdn.example.com/template.png", "https://cdn.example.com/mask.png"),
	}
	items := []StudioBatchItemRecord{{
		ID:               "item-1",
		BatchID:          "batch-1",
		TargetGroupKey:   "size:1200x1200",
		TargetGroupLabel: "1200 x 1200",
		SelectionIDs:     SheinStudioStringList{"selection-1", "selection-2"},
		GroupMode:        "shared_by_size",
		Status:           StudioBatchItemStatusReviewReady,
		SelectionCount:   2,
		CreatedAt:        now,
		UpdatedAt:        now,
	}}
	designs := []StudioMaterializedDesignRecord{
		{
			ID:              "design-1",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			ImageURL:        "https://cdn.example.com/design-1.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusApproved,
			SortOrder:       0,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		{
			ID:              "design-2",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			ImageURL:        "https://cdn.example.com/design-2.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusApproved,
			SortOrder:       1,
			CreatedAt:       now.Add(time.Second),
			UpdatedAt:       now.Add(time.Second),
		},
	}
	if err := batchRepo.CreateStudioBatchGraph(ctx, batch, items, nil, designs); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	svc := &service{repo: taskRepo, studioDeps: studioDependencies{batchRepo: batchRepo}}
	svc.taskDeps.taskSubmitter = &studioBatchTaskSubmitterStub{}

	result, err := svc.CreateStudioBatchTasks(ctx, "batch-1", &CreateStudioBatchTasksRequest{
		DesignIDs: []string{"design-2", "design-1"},
	})
	if err != nil {
		t.Fatalf("CreateStudioBatchTasks() error = %v", err)
	}

	got := make([]string, 0, len(result.CreatedTasks))
	for _, task := range result.CreatedTasks {
		got = append(got, task.DesignID+":"+task.SelectionID)
	}
	want := []string{
		"design-2:selection-1",
		"design-2:selection-2",
		"design-1:selection-1",
		"design-1:selection-2",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("created task tuple order = %v, want %v", got, want)
	}
}

func TestBuildStudioBatchTaskCandidates_PerProductRequiresOneSelection(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	batch := newStudioBatchRecordForTest("batch-1", now)
	batch.GroupedImageMode = "per_product"
	batch.GroupedSelections = SheinStudioGroupedSelectionList{
		studioBatchFanOutSelection("selection-1", 3001, "Red", "870", "https://cdn.example.com/template.png", "https://cdn.example.com/mask.png"),
		studioBatchFanOutSelection("selection-2", 3002, "Blue", "871", "https://cdn.example.com/template.png", "https://cdn.example.com/mask.png"),
	}
	item := StudioBatchItemRecord{
		ID:           "item-1",
		BatchID:      "batch-1",
		SelectionIDs: SheinStudioStringList{"selection-1", "selection-2"},
		GroupMode:    "per_product",
	}
	design := StudioMaterializedDesignRecord{
		ID:           "design-1",
		BatchID:      "batch-1",
		ItemID:       "item-1",
		ImageURL:     "https://cdn.example.com/design-1.png",
		ReviewStatus: StudioMaterializedDesignReviewStatusApproved,
	}

	var svc taskStudioBatchService
	candidates, rejected, err := svc.buildStudioBatchTaskCandidates(context.Background(), nil, batch, &StudioBatchDetailGraph{
		Batch: batch,
		Items: []StudioBatchItemRecord{item},
	}, []StudioMaterializedDesignRecord{design})
	if err != nil {
		t.Fatalf("buildStudioBatchTaskCandidates() error = %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("candidates = %+v, want none", candidates)
	}
	if len(rejected) != 1 {
		t.Fatalf("rejected = %+v, want 1", rejected)
	}
	if got := rejected[0].ReasonCode; got != "selection_cardinality_mismatch" {
		t.Fatalf("reason code = %q, want selection_cardinality_mismatch", got)
	}
	if rejected[0].DesignID != "design-1" || rejected[0].ItemID != "item-1" {
		t.Fatalf("rejected task = %+v, want design/item relationship", rejected[0])
	}
}

func TestBuildStudioBatchTaskCandidates_SharedMismatchReturnsStructuredRejection(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	batch := newStudioBatchRecordForTest("batch-1", now)
	batch.GroupedImageMode = "shared_by_size"
	batch.GroupedSelections = SheinStudioGroupedSelectionList{
		studioBatchFanOutSelection("selection-1", 3001, "Red", "870", "https://cdn.example.com/template-a.png", "https://cdn.example.com/mask.png"),
		studioBatchFanOutSelection("selection-2", 3002, "Blue", "871", "https://cdn.example.com/template-b.png", "https://cdn.example.com/mask.png"),
	}
	item := StudioBatchItemRecord{
		ID:           "item-1",
		BatchID:      "batch-1",
		SelectionIDs: SheinStudioStringList{"selection-1", "selection-2"},
		GroupMode:    "shared_by_size",
	}
	design := StudioMaterializedDesignRecord{
		ID:           "design-1",
		BatchID:      "batch-1",
		ItemID:       "item-1",
		ImageURL:     "https://cdn.example.com/design-1.png",
		ReviewStatus: StudioMaterializedDesignReviewStatusApproved,
	}

	var svc taskStudioBatchService
	candidates, rejected, err := svc.buildStudioBatchTaskCandidates(context.Background(), nil, batch, &StudioBatchDetailGraph{
		Batch: batch,
		Items: []StudioBatchItemRecord{item},
	}, []StudioMaterializedDesignRecord{design})
	if err != nil {
		t.Fatalf("buildStudioBatchTaskCandidates() error = %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("candidates = %+v, want none", candidates)
	}
	if len(rejected) != 1 {
		t.Fatalf("rejected = %+v, want 1", rejected)
	}
	if got := rejected[0].ReasonCode; got != "compatibility_mismatch" {
		t.Fatalf("reason code = %q, want compatibility_mismatch", got)
	}
	if rejected[0].DesignID != "design-1" || rejected[0].ItemID != "item-1" || rejected[0].SelectionID != "selection-2" {
		t.Fatalf("rejected task = %+v, want structured mismatch on second selection", rejected[0])
	}
}

func TestBuildStudioBatchTaskCandidates_PartialMissingOwnedSelectionReturnsCandidateAndRejection(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	batch := newStudioBatchRecordForTest("batch-1", now)
	batch.GroupedImageMode = "shared_by_size"
	batch.GroupedSelections = SheinStudioGroupedSelectionList{
		studioBatchFanOutSelection("selection-1", 3001, "Red", "870", "https://cdn.example.com/template.png", "https://cdn.example.com/mask.png"),
	}
	item := StudioBatchItemRecord{
		ID:           "item-1",
		BatchID:      "batch-1",
		SelectionIDs: SheinStudioStringList{"selection-1", "selection-owned-missing"},
		GroupMode:    "shared_by_size",
	}
	design := StudioMaterializedDesignRecord{
		ID:           "design-1",
		BatchID:      "batch-1",
		ItemID:       "item-1",
		ImageURL:     "https://cdn.example.com/design-1.png",
		ReviewStatus: StudioMaterializedDesignReviewStatusApproved,
	}

	var svc taskStudioBatchService
	candidates, rejected, err := svc.buildStudioBatchTaskCandidates(context.Background(), nil, batch, &StudioBatchDetailGraph{
		Batch: batch,
		Items: []StudioBatchItemRecord{item},
	}, []StudioMaterializedDesignRecord{design})
	if err != nil {
		t.Fatalf("buildStudioBatchTaskCandidates() error = %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("candidates = %+v, want one valid owned selection candidate", candidates)
	}
	if candidates[0].SelectionID != "selection-1" {
		t.Fatalf("candidate selection id = %q, want selection-1", candidates[0].SelectionID)
	}
	if len(rejected) != 1 {
		t.Fatalf("rejected = %+v, want 1 missing-selection rejection", rejected)
	}
	if got := rejected[0].ReasonCode; got != "selection_not_in_batch" {
		t.Fatalf("reason code = %q, want selection_not_in_batch", got)
	}
	if rejected[0].DesignID != "design-1" || rejected[0].ItemID != "item-1" || rejected[0].SelectionID != "selection-owned-missing" {
		t.Fatalf("rejected task = %+v, want structured missing selection rejection", rejected[0])
	}
}

func TestBuildStudioBatchTaskCandidates_HydratesMissingSDSProductTables(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	productSize := `[[{"content":"尺码"},{"content":"胸围(cm/in)"}],[{"content":"S"},{"content":"92cm/36.2in"}]]`
	packagingSpecification := `[[{"content":"尺码"},{"content":"包装尺寸（cm）"}],[{"content":"S"},{"content":"40.0*30.0*1.0"}]]`
	batch := newStudioBatchRecordForTest("batch-1", now)
	batch.GroupedImageMode = "shared_by_size"
	batch.GroupedSelections = SheinStudioGroupedSelectionList{
		studioBatchFanOutSelection("selection-1", 3001, "Red", "870", "https://cdn.example.com/template.png", "https://cdn.example.com/mask.png"),
		studioBatchFanOutSelection("selection-2", 3002, "Blue", "870", "https://cdn.example.com/template.png", "https://cdn.example.com/mask.png"),
	}
	item := StudioBatchItemRecord{
		ID:           "item-1",
		BatchID:      "batch-1",
		SelectionIDs: SheinStudioStringList{"selection-1", "selection-2"},
		GroupMode:    "shared_by_size",
	}
	design := StudioMaterializedDesignRecord{
		ID:           "design-1",
		BatchID:      "batch-1",
		ItemID:       "item-1",
		ImageURL:     "https://cdn.example.com/design-1.png",
		ReviewStatus: StudioMaterializedDesignReviewStatusApproved,
	}
	svc := taskStudioBatchService{
		sdsProductDetailProvider: stubSDSBaselineRemoteProvider{
			productDetail: &sdstemplate.ProductDetail{
				ProductSummary: sdstemplate.ProductSummary{
					ProductDetails: sdstemplate.ProductDetails{
						ProductSize:            productSize,
						PackagingSpecification: packagingSpecification,
					},
				},
			},
		},
	}

	candidates, rejected, err := svc.buildStudioBatchTaskCandidates(context.Background(), nil, batch, &StudioBatchDetailGraph{
		Batch: batch,
		Items: []StudioBatchItemRecord{item},
	}, []StudioMaterializedDesignRecord{design})
	if err != nil {
		t.Fatalf("buildStudioBatchTaskCandidates() error = %v", err)
	}
	if len(rejected) != 0 {
		t.Fatalf("rejected = %+v, want none", rejected)
	}
	if len(candidates) != 2 {
		t.Fatalf("candidates = %+v, want two", candidates)
	}
	for _, candidate := range candidates {
		if candidate.SelectionSnapshot.ProductSize != productSize {
			t.Fatalf("ProductSize = %q, want %q", candidate.SelectionSnapshot.ProductSize, productSize)
		}
		if candidate.SelectionSnapshot.PackagingSpecification != packagingSpecification {
			t.Fatalf("PackagingSpecification = %q, want %q", candidate.SelectionSnapshot.PackagingSpecification, packagingSpecification)
		}
		if got, want := candidate.CompatibilityFingerprint, buildStudioBatchCompatibilityFingerprint(candidate.SelectionSnapshot); got != want {
			t.Fatalf("CompatibilityFingerprint = %q, want hydrated fingerprint %q", got, want)
		}
	}
	eligible, gateRejected, failed := svc.evaluateStudioBatchTaskCandidates(context.Background(), batch, &StudioBatchDetailGraph{
		Batch: batch,
		Items: []StudioBatchItemRecord{item},
	}, []StudioMaterializedDesignRecord{design}, candidates)
	if len(failed) != 0 {
		t.Fatalf("failed = %+v, want none", failed)
	}
	if len(gateRejected) != 0 {
		t.Fatalf("gate rejected = %+v, want none", gateRejected)
	}
	if len(eligible) != 2 {
		t.Fatalf("eligible = %+v, want two hydrated candidates", eligible)
	}
}

func TestBuildStudioBatchTaskCandidates_DoesNotFallbackToAllBatchSelectionsForOwnedItem(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	batch := newStudioBatchRecordForTest("batch-1", now)
	batch.GroupedImageMode = "shared_by_size"
	batch.GroupedSelections = SheinStudioGroupedSelectionList{
		studioBatchFanOutSelection("selection-other", 3001, "Red", "870", "https://cdn.example.com/template.png", "https://cdn.example.com/mask.png"),
	}
	item := StudioBatchItemRecord{
		ID:           "item-1",
		BatchID:      "batch-1",
		SelectionIDs: SheinStudioStringList{"selection-owned-missing"},
		GroupMode:    "shared_by_size",
	}
	design := StudioMaterializedDesignRecord{
		ID:           "design-1",
		BatchID:      "batch-1",
		ItemID:       "item-1",
		ImageURL:     "https://cdn.example.com/design-1.png",
		ReviewStatus: StudioMaterializedDesignReviewStatusApproved,
	}

	var svc taskStudioBatchService
	candidates, rejected, err := svc.buildStudioBatchTaskCandidates(context.Background(), nil, batch, &StudioBatchDetailGraph{
		Batch: batch,
		Items: []StudioBatchItemRecord{item},
	}, []StudioMaterializedDesignRecord{design})
	if err != nil {
		t.Fatalf("buildStudioBatchTaskCandidates() error = %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("candidates = %+v, want no fallback candidate from unrelated batch selections", candidates)
	}
	if len(rejected) != 1 {
		t.Fatalf("rejected = %+v, want 1", rejected)
	}
	if got := rejected[0].ReasonCode; got != "selection_not_in_batch" {
		t.Fatalf("reason code = %q, want selection_not_in_batch", got)
	}
	if got := rejected[0].SelectionID; got != "selection-owned-missing" {
		t.Fatalf("rejected selection id = %q, want selection-owned-missing", got)
	}
}

func TestServiceCreateStudioBatchTasks_RejectsCompatibilityMismatch(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	taskRepo := newStudioBatchTaskRepositoryStub()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	batch := newStudioBatchRecordForTest("batch-1", now)
	batch.GroupedImageMode = "shared_by_size"
	batch.GroupedSelections = SheinStudioGroupedSelectionList{
		{
			SelectionID:  "selection-1",
			SheinStoreID: "9001",
			Selection: SheinStudioSelection{
				ProductID:          1001,
				ParentProductID:    2002,
				VariantID:          3003,
				PrototypeGroupID:   4004,
				LayerID:            "layer-front",
				DesignType:         "material",
				PrintableWidth:     1200,
				PrintableHeight:    1200,
				TemplateImageURL:   "https://cdn.example.com/template-a.png",
				MaskImageURL:       "https://cdn.example.com/mask-a.png",
				SelectedVariantIDs: []int64{3003},
			},
			Eligible: true,
		},
		{
			SelectionID:  "selection-2",
			SheinStoreID: "9001",
			Selection: SheinStudioSelection{
				ProductID:          1001,
				ParentProductID:    2002,
				VariantID:          3004,
				PrototypeGroupID:   4004,
				LayerID:            "layer-front",
				DesignType:         "material",
				PrintableWidth:     1200,
				PrintableHeight:    1200,
				TemplateImageURL:   "https://cdn.example.com/template-b.png",
				MaskImageURL:       "https://cdn.example.com/mask-a.png",
				SelectedVariantIDs: []int64{3004},
			},
			Eligible: true,
		},
	}

	items := newStudioBatchItemsForTest("batch-1", now)
	items[0].SelectionIDs = SheinStudioStringList{"selection-1", "selection-2"}
	items[0].SelectionCount = 2
	if err := repo.CreateStudioBatchGraph(ctx, batch, items, newStudioBatchAttemptsForTest("item-1", now), []StudioMaterializedDesignRecord{
		{
			ID:              "design-1",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			ImageURL:        "https://cdn.example.com/design-1.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusApproved,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	svc := &service{studioDeps: studioDependencies{batchRepo: repo}}
	svc.repo = taskRepo
	svc.taskDeps.taskSubmitter = &studioBatchTaskSubmitterStub{}

	result, err := svc.CreateStudioBatchTasks(ctx, "batch-1", &CreateStudioBatchTasksRequest{
		DesignIDs: []string{"design-1"},
	})
	if err != nil {
		t.Fatalf("CreateStudioBatchTasks() error = %v", err)
	}
	if len(result.RejectedTasks) != 1 {
		t.Fatalf("rejected tasks = %+v, want 1", result.RejectedTasks)
	}
	if got := result.RejectedTasks[0].ReasonCode; got != "compatibility_mismatch" {
		t.Fatalf("reason code = %q, want compatibility_mismatch", got)
	}
}

func TestServiceCreateStudioBatchTasksCreatesRealListingKitTasks(t *testing.T) {
	t.Parallel()

	batchRepo := NewMemStudioBatchRepository()
	taskRepo := newStudioBatchTaskRepositoryStub()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	batch := newStudioBatchRecordForTest("batch-1", now)
	batch.GroupedSelections = SheinStudioGroupedSelectionList{
		studioBatchFanOutSelection("selection-1", 3003, "Red", "869", "https://cdn.example.com/template.png", "https://cdn.example.com/mask.png"),
	}
	items := newStudioBatchItemsForTest("batch-1", now)
	items[0].SelectionIDs = SheinStudioStringList{"selection-1"}
	items[0].SelectionCount = 1
	if err := batchRepo.CreateStudioBatchGraph(ctx, batch, items, newStudioBatchAttemptsForTest("item-1", now), []StudioMaterializedDesignRecord{
		{
			ID:               "design-1",
			BatchID:          "batch-1",
			ItemID:           "item-1",
			SourceAttemptID:  "attempt-1",
			ImageURL:         "https://cdn.example.com/design-1.png",
			ReviewStatus:     StudioMaterializedDesignReviewStatusApproved,
			TargetGroupKey:   "size:1200x1200",
			TargetGroupLabel: "Style 1",
			SortOrder:        0,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
	}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	svc := &service{
		repo: taskRepo,

		studioDeps: studioDependencies{
			sessionRepo: &studioBatchTaskCreationSessionRepoStub{
				session: &SheinStudioSession{
					ID:                "batch-1",
					Prompt:            "retro cherries",
					ImageStrategy:     "sds_official",
					SheinStoreID:      "869",
					SelectedSDSImages: nil,
					Selection: SheinStudioSelectionSnapshot{
						ProductID:              1001,
						ParentProductID:        2002,
						VariantID:              3003,
						PrototypeGroupID:       4004,
						LayerID:                "layer-1",
						ProductName:            "Canvas Tote",
						VariantLabel:           "Red / One Size",
						PrintableWidth:         1200,
						PrintableHeight:        1200,
						TemplateImageURL:       "https://cdn.example.com/template.png",
						MaskImageURL:           "https://cdn.example.com/mask.png",
						BlankDesignURL:         "https://cdn.example.com/blank.png",
						MockupImageURL:         "https://cdn.example.com/mockup.png",
						MockupImageURLs:        []string{"https://cdn.example.com/mockup.png"},
						SizeReferenceImageURLs: []string{"https://cdn.example.com/size.png"},
						Variants: []SheinStudioSelectionVariant{
							{
								VariantID:              3003,
								VariantSKU:             "SKU-RED",
								Size:                   "One Size",
								Color:                  "Red",
								PrototypeGroupID:       4004,
								LayerID:                "layer-1",
								TemplateImageURL:       "https://cdn.example.com/template.png",
								MaskImageURL:           "https://cdn.example.com/mask.png",
								BlankDesignURL:         "https://cdn.example.com/blank.png",
								MockupImageURL:         "https://cdn.example.com/mockup.png",
								MockupImageURLs:        []string{"https://cdn.example.com/mockup.png"},
								SizeReferenceImageURLs: []string{"https://cdn.example.com/size.png"},
							},
						},
					},
				},
			}, batchRepo: batchRepo,
		},
		taskDeps: taskDependencies{
			taskSubmitter: &studioBatchTaskSubmitterStub{},
		},
	}

	result, err := svc.CreateStudioBatchTasks(ctx, "batch-1", &CreateStudioBatchTasksRequest{
		DesignIDs: []string{"design-1"},
	})
	if err != nil {
		t.Fatalf("CreateStudioBatchTasks() error = %v", err)
	}
	if len(result.CreatedTasks) != 1 {
		t.Fatalf("created tasks = %+v, want 1", result.CreatedTasks)
	}

	createdTask, err := taskRepo.GetTask(ctx, result.CreatedTasks[0].ID)
	if err != nil {
		t.Fatalf("GetTask(%q) error = %v, want persisted listing task", result.CreatedTasks[0].ID, err)
	}
	if createdTask.Request == nil {
		t.Fatal("created task request = nil, want persisted generate request")
	}
	if got := createdTask.Request.Text; got != "retro cherries" {
		t.Fatalf("created task text = %q, want session prompt", got)
	}
	if got := createdTask.Request.SheinStoreID; got != 869 {
		t.Fatalf("created task shein store id = %d, want 869", got)
	}
	if len(createdTask.Request.ImageURLs) != 1 || createdTask.Request.ImageURLs[0] != "https://cdn.example.com/design-1.png" {
		t.Fatalf("created task image urls = %+v, want approved design image", createdTask.Request.ImageURLs)
	}
	if createdTask.Request.Options == nil || createdTask.Request.Options.SDS == nil {
		t.Fatalf("created task options = %+v, want SDS metadata", createdTask.Request.Options)
	}
	if got := createdTask.Request.Options.SDS.VariantID; got != 3003 {
		t.Fatalf("created task SDS variant id = %d, want 3003", got)
	}
}

func TestServiceCreateStudioBatchTasksUsesItemSelectionOwnershipForGroupedProducts(t *testing.T) {
	t.Parallel()

	batchRepo := NewMemStudioBatchRepository()
	taskRepo := newStudioBatchTaskRepositoryStub()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	batch := newStudioBatchRecordForTest("batch-1", now)
	batch.GroupedImageMode = "per_product"
	batch.Selection = SheinStudioSelectionSnapshot{
		ProductID:        1001,
		ParentProductID:  2002,
		VariantID:        3003,
		PrototypeGroupID: 4004,
		LayerID:          "layer-main",
		ProductName:      "Primary Tote",
		VariantLabel:     "Black",
		PrintableWidth:   1200,
		PrintableHeight:  1200,
	}
	batch.GroupedSelections = SheinStudioGroupedSelectionList{
		{
			SelectionID:  "2002:5005:4004:layer-group:4004",
			SheinStoreID: "870",
			Eligible:     true,
			Selection: SheinStudioSelection{
				ProductID:          1002,
				ParentProductID:    2002,
				VariantID:          4004,
				PrototypeGroupID:   5005,
				LayerID:            "layer-group",
				DesignType:         "material",
				ProductName:        "Grouped Wallet",
				VariantLabel:       "White",
				PrintableWidth:     1200,
				PrintableHeight:    1200,
				TemplateImageURL:   "https://cdn.example.com/group-template.png",
				MaskImageURL:       "https://cdn.example.com/group-mask.png",
				SelectedVariantIDs: []int64{4004},
			},
		},
	}

	items := []StudioBatchItemRecord{
		{
			ID:             "item-1",
			BatchID:        "batch-1",
			TargetGroupKey: "2002:4004:3003:layer-main:3003",
			SelectionIDs:   SheinStudioStringList{"2002:4004:3003:layer-main:3003"},
			Status:         StudioBatchItemStatusReviewReady,
			SelectionCount: 1,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		{
			ID:               "item-2",
			BatchID:          "batch-1",
			TargetGroupKey:   "2002:5005:4004:layer-group:4004",
			TargetGroupLabel: "Grouped Wallet",
			SelectionIDs:     SheinStudioStringList{"2002:5005:4004:layer-group:4004"},
			Status:           StudioBatchItemStatusReviewReady,
			SelectionCount:   1,
			CreatedAt:        now.Add(time.Second),
			UpdatedAt:        now.Add(time.Second),
		},
	}

	designs := []StudioMaterializedDesignRecord{
		{
			ID:               "design-grouped",
			BatchID:          "batch-1",
			ItemID:           "item-2",
			SourceAttemptID:  "attempt-2",
			ImageURL:         "https://cdn.example.com/design-grouped.png",
			ReviewStatus:     StudioMaterializedDesignReviewStatusApproved,
			TargetGroupKey:   "2002:5005:4004:layer-group:4004",
			TargetGroupLabel: "Grouped Wallet",
			SortOrder:        0,
			CreatedAt:        now.Add(time.Second),
			UpdatedAt:        now.Add(time.Second),
		},
	}

	if err := batchRepo.CreateStudioBatchGraph(ctx, batch, items, nil, designs); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	svc := &service{
		repo: taskRepo,

		studioDeps: studioDependencies{
			sessionRepo: &studioBatchTaskCreationSessionRepoStub{
				session: &SheinStudioSession{
					ID:                "batch-1",
					Prompt:            "retro cherries",
					ImageStrategy:     "sds_official",
					SheinStoreID:      "869",
					GroupedImageMode:  "per_product",
					Selection:         batch.Selection,
					GroupedSelections: batch.GroupedSelections,
				},
			}, batchRepo: batchRepo,
		},
		taskDeps: taskDependencies{
			taskSubmitter: &studioBatchTaskSubmitterStub{},
		},
	}

	result, err := svc.CreateStudioBatchTasks(ctx, "batch-1", &CreateStudioBatchTasksRequest{
		DesignIDs: []string{"design-grouped"},
	})
	if err != nil {
		t.Fatalf("CreateStudioBatchTasks() error = %v", err)
	}
	if len(result.CreatedTasks) != 1 {
		t.Fatalf("created tasks = %+v, want 1", result.CreatedTasks)
	}

	createdTask, err := taskRepo.GetTask(ctx, result.CreatedTasks[0].ID)
	if err != nil {
		t.Fatalf("GetTask(%q) error = %v", result.CreatedTasks[0].ID, err)
	}
	if createdTask.Request == nil || createdTask.Request.Options == nil || createdTask.Request.Options.SDS == nil {
		t.Fatalf("created task request = %+v, want SDS metadata", createdTask.Request)
	}
	if got := createdTask.Request.Options.SDS.VariantID; got != 4004 {
		t.Fatalf("created task SDS variant id = %d, want grouped selection variant 4004", got)
	}
	if got := createdTask.Request.Options.SDS.ProductName; got != "Grouped Wallet" {
		t.Fatalf("created task SDS product name = %q, want grouped product", got)
	}
	if got := createdTask.Request.SheinStoreID; got != 870 {
		t.Fatalf("created task shein store id = %d, want grouped selection store 870", got)
	}
}

func TestServiceCreateStudioBatchTasksDefaultsGroupedSelectionDesignType(t *testing.T) {
	t.Parallel()

	batchRepo := NewMemStudioBatchRepository()
	taskRepo := newStudioBatchTaskRepositoryStub()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	batch := newStudioBatchRecordForTest("batch-1", now)
	batch.GroupedImageMode = "per_product"
	batch.Selection.DesignType = "material"
	grouped := studioBatchFanOutSelection("selection-grouped", 4004, "White", "870", "https://cdn.example.com/group-template.png", "https://cdn.example.com/group-mask.png")
	grouped.Selection.DesignType = ""
	batch.GroupedSelections = SheinStudioGroupedSelectionList{grouped}

	items := []StudioBatchItemRecord{{
		ID:               "item-1",
		BatchID:          "batch-1",
		TargetGroupKey:   "selection-grouped",
		TargetGroupLabel: "Grouped Wallet",
		SelectionIDs:     SheinStudioStringList{"selection-grouped"},
		Status:           StudioBatchItemStatusReviewReady,
		SelectionCount:   1,
		CreatedAt:        now,
		UpdatedAt:        now,
	}}
	designs := []StudioMaterializedDesignRecord{{
		ID:               "design-grouped",
		BatchID:          "batch-1",
		ItemID:           "item-1",
		SourceAttemptID:  "attempt-1",
		ImageURL:         "https://cdn.example.com/design-grouped.png",
		ReviewStatus:     StudioMaterializedDesignReviewStatusApproved,
		TargetGroupKey:   "selection-grouped",
		TargetGroupLabel: "Grouped Wallet",
		CreatedAt:        now,
		UpdatedAt:        now,
	}}

	if err := batchRepo.CreateStudioBatchGraph(ctx, batch, items, nil, designs); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	svc := &service{
		repo: taskRepo,
		studioDeps: studioDependencies{
			sessionRepo: &studioBatchTaskCreationSessionRepoStub{
				session: &SheinStudioSession{
					ID:                "batch-1",
					Prompt:            "retro cherries",
					ImageStrategy:     "sds_official",
					SheinStoreID:      "869",
					GroupedImageMode:  "per_product",
					Selection:         batch.Selection,
					GroupedSelections: batch.GroupedSelections,
				},
			},
			batchRepo: batchRepo,
		},
		taskDeps: taskDependencies{taskSubmitter: &studioBatchTaskSubmitterStub{}},
	}

	result, err := svc.CreateStudioBatchTasks(ctx, "batch-1", &CreateStudioBatchTasksRequest{
		DesignIDs: []string{"design-grouped"},
	})
	if err != nil {
		t.Fatalf("CreateStudioBatchTasks() error = %v", err)
	}
	if len(result.CreatedTasks) != 1 {
		t.Fatalf("created tasks = %+v, want 1", result.CreatedTasks)
	}
	if len(result.RejectedTasks) != 0 {
		t.Fatalf("rejected tasks = %+v, want none", result.RejectedTasks)
	}

	createdTask, err := taskRepo.GetTask(ctx, result.CreatedTasks[0].ID)
	if err != nil {
		t.Fatalf("GetTask(%q) error = %v", result.CreatedTasks[0].ID, err)
	}
	if createdTask.Request == nil || createdTask.Request.Options == nil || createdTask.Request.Options.SDS == nil {
		t.Fatalf("created task request = %+v, want SDS metadata", createdTask.Request)
	}
	if got := createdTask.Request.Options.SDS.DesignType; got != "material" {
		t.Fatalf("created task SDS design type = %q, want material", got)
	}
}

func TestServiceCreateStudioBatchTasksAllowsGroupedSelectionWithoutMaskImage(t *testing.T) {
	t.Parallel()

	batchRepo := NewMemStudioBatchRepository()
	taskRepo := newStudioBatchTaskRepositoryStub()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	batch := newStudioBatchRecordForTest("batch-1", now)
	batch.GroupedImageMode = "per_product"
	grouped := studioBatchFanOutSelection("selection-grouped", 4004, "White", "870", "https://cdn.example.com/group-template.png", "")
	batch.GroupedSelections = SheinStudioGroupedSelectionList{grouped}
	items := []StudioBatchItemRecord{{
		ID:               "item-1",
		BatchID:          "batch-1",
		TargetGroupKey:   "selection-grouped",
		TargetGroupLabel: "Grouped Wallet",
		SelectionIDs:     SheinStudioStringList{"selection-grouped"},
		Status:           StudioBatchItemStatusReviewReady,
		SelectionCount:   1,
		CreatedAt:        now,
		UpdatedAt:        now,
	}}
	designs := []StudioMaterializedDesignRecord{{
		ID:               "design-grouped",
		BatchID:          "batch-1",
		ItemID:           "item-1",
		ImageURL:         "https://cdn.example.com/design-grouped.png",
		ReviewStatus:     StudioMaterializedDesignReviewStatusApproved,
		TargetGroupKey:   "selection-grouped",
		TargetGroupLabel: "Grouped Wallet",
		CreatedAt:        now,
		UpdatedAt:        now,
	}}
	if err := batchRepo.CreateStudioBatchGraph(ctx, batch, items, nil, designs); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	svc := &service{
		repo: taskRepo,
		studioDeps: studioDependencies{
			sessionRepo: &studioBatchTaskCreationSessionRepoStub{
				session: &SheinStudioSession{
					ID:                "batch-1",
					Prompt:            "retro cherries",
					ImageStrategy:     "sds_official",
					SheinStoreID:      "869",
					GroupedImageMode:  "per_product",
					Selection:         batch.Selection,
					GroupedSelections: batch.GroupedSelections,
				},
			},
			batchRepo: batchRepo,
		},
		taskDeps: taskDependencies{taskSubmitter: &studioBatchTaskSubmitterStub{}},
	}

	result, err := svc.CreateStudioBatchTasks(ctx, "batch-1", &CreateStudioBatchTasksRequest{
		DesignIDs: []string{"design-grouped"},
	})
	if err != nil {
		t.Fatalf("CreateStudioBatchTasks() error = %v", err)
	}
	if len(result.CreatedTasks) != 1 {
		t.Fatalf("created tasks = %+v, want 1", result.CreatedTasks)
	}
	if len(result.RejectedTasks) != 0 {
		t.Fatalf("rejected tasks = %+v, want none", result.RejectedTasks)
	}
}

func TestServiceCreateStudioBatchTasksReturnsPartialSuccessWhenQueueIsFull(t *testing.T) {
	t.Parallel()

	batchRepo := NewMemStudioBatchRepository()
	taskRepo := newStudioBatchTaskRepositoryStub()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	if err := batchRepo.CreateStudioBatchGraph(ctx, newStudioBatchRecordForTest("batch-1", now), newStudioBatchItemsForTest("batch-1", now), newStudioBatchAttemptsForTest("item-1", now), []StudioMaterializedDesignRecord{
		{
			ID:               "design-1",
			BatchID:          "batch-1",
			ItemID:           "item-1",
			SourceAttemptID:  "attempt-1",
			ImageURL:         "https://cdn.example.com/design-1.png",
			ReviewStatus:     StudioMaterializedDesignReviewStatusApproved,
			TargetGroupKey:   "size:1200x1200",
			TargetGroupLabel: "Style 1",
			SortOrder:        0,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		{
			ID:               "design-2",
			BatchID:          "batch-1",
			ItemID:           "item-1",
			SourceAttemptID:  "attempt-2",
			ImageURL:         "https://cdn.example.com/design-2.png",
			ReviewStatus:     StudioMaterializedDesignReviewStatusApproved,
			TargetGroupKey:   "size:1200x1200",
			TargetGroupLabel: "Style 2",
			SortOrder:        1,
			CreatedAt:        now.Add(time.Second),
			UpdatedAt:        now.Add(time.Second),
		},
	}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	svc := &service{
		repo: taskRepo,

		studioDeps: studioDependencies{
			sessionRepo: &studioBatchTaskCreationSessionRepoStub{
				session: &SheinStudioSession{
					ID:            "batch-1",
					Prompt:        "retro cherries",
					ImageStrategy: "sds_official",
					Selection: SheinStudioSelectionSnapshot{
						ProductID:        1001,
						ParentProductID:  2002,
						VariantID:        3003,
						PrototypeGroupID: 4004,
						LayerID:          "layer-1",
						ProductName:      "Canvas Tote",
						PrintableWidth:   1200,
						PrintableHeight:  1200,
					},
				},
			}, batchRepo: batchRepo,
		},
		taskDeps: taskDependencies{
			taskSubmitter: &studioBatchTaskSubmitterStub{failAfter: 1},
		},
	}

	result, err := svc.CreateStudioBatchTasks(ctx, "batch-1", &CreateStudioBatchTasksRequest{
		DesignIDs: []string{"design-1", "design-2"},
	})
	if err != nil {
		t.Fatalf("CreateStudioBatchTasks() error = %v", err)
	}
	if len(result.CreatedTasks) != 1 {
		t.Fatalf("created tasks = %+v, want 1", result.CreatedTasks)
	}
	if len(result.FailedTasks) != 1 {
		t.Fatalf("failed tasks = %+v, want 1", result.FailedTasks)
	}
	if got := result.FailedTasks[0].DesignID; got != "design-2" {
		t.Fatalf("failed task design id = %q, want design-2", got)
	}
	if got := result.FailedTasks[0].Message; !strings.Contains(got, "工作队列已满") {
		t.Fatalf("failed task message = %q, want queue full", got)
	}
	if _, err := taskRepo.GetTask(ctx, result.CreatedTasks[0].ID); err != nil {
		t.Fatalf("GetTask(%q) error = %v", result.CreatedTasks[0].ID, err)
	}
}

func TestServiceCreateStudioBatchTasksReusesExistingTasksForRepeatedRequests(t *testing.T) {
	t.Parallel()

	batchRepo := NewMemStudioBatchRepository()
	taskRepo := newStudioBatchTaskRepositoryStub()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	if err := batchRepo.CreateStudioBatchGraph(ctx, newStudioBatchRecordForTest("batch-1", now), newStudioBatchItemsForTest("batch-1", now), newStudioBatchAttemptsForTest("item-1", now), []StudioMaterializedDesignRecord{
		{
			ID:               "design-1",
			BatchID:          "batch-1",
			ItemID:           "item-1",
			SourceAttemptID:  "attempt-1",
			ImageURL:         "https://cdn.example.com/design-1.png",
			ReviewStatus:     StudioMaterializedDesignReviewStatusApproved,
			TargetGroupKey:   "size:1200x1200",
			TargetGroupLabel: "Style 1",
			SortOrder:        0,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
	}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	sessionRepo := &studioBatchTaskCreationSessionRepoStub{
		session: &SheinStudioSession{
			ID:            "batch-1",
			Prompt:        "retro cherries",
			ImageStrategy: "sds_official",
			Selection: SheinStudioSelectionSnapshot{
				ProductID:        1001,
				ParentProductID:  2002,
				VariantID:        3003,
				PrototypeGroupID: 4004,
				LayerID:          "layer-1",
				ProductName:      "Canvas Tote",
				PrintableWidth:   1200,
				PrintableHeight:  1200,
			},
		},
	}
	svc := &service{
		repo: taskRepo,

		studioDeps: studioDependencies{
			sessionRepo: sessionRepo, batchRepo: batchRepo,
		},
		taskDeps: taskDependencies{
			taskSubmitter: &studioBatchTaskSubmitterStub{},
		},
	}

	first, err := svc.CreateStudioBatchTasks(ctx, "batch-1", &CreateStudioBatchTasksRequest{
		DesignIDs: []string{"design-1"},
	})
	if err != nil {
		t.Fatalf("first CreateStudioBatchTasks() error = %v", err)
	}
	if len(first.CreatedTasks) != 1 {
		t.Fatalf("first created tasks = %+v, want 1", first.CreatedTasks)
	}

	second, err := svc.CreateStudioBatchTasks(ctx, "batch-1", &CreateStudioBatchTasksRequest{
		DesignIDs: []string{"design-1"},
	})
	if err != nil {
		t.Fatalf("second CreateStudioBatchTasks() error = %v", err)
	}
	if len(second.CreatedTasks) != 0 {
		t.Fatalf("second created tasks = %+v, want no newly created task", second.CreatedTasks)
	}
	if len(second.ReusedTasks) != 1 {
		t.Fatalf("second reused tasks = %+v, want 1 reused task", second.ReusedTasks)
	}
	if second.ReusedTasks[0].ID != first.CreatedTasks[0].ID {
		t.Fatalf("second reused task id = %q, want %q", second.ReusedTasks[0].ID, first.CreatedTasks[0].ID)
	}
	if got := len(taskRepo.tasks); got != 1 {
		t.Fatalf("persisted task count = %d, want 1 without duplicates", got)
	}
}

func TestServiceCreateStudioBatchTasks_ReusesDurableLinkedTaskWithoutSession(t *testing.T) {
	t.Parallel()

	batchRepo := NewMemStudioBatchRepository()
	taskRepo := newStudioBatchTaskRepositoryStub()
	linkRepo := NewMemStudioBatchTaskLinkRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	if err := batchRepo.CreateStudioBatchGraph(ctx, newStudioBatchRecordForTest("batch-1", now), newStudioBatchItemsForTest("batch-1", now), newStudioBatchAttemptsForTest("item-1", now), []StudioMaterializedDesignRecord{
		{
			ID:              "design-1",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			ImageURL:        "https://cdn.example.com/design-1.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusApproved,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	svc := &service{
		repo: taskRepo,
		studioDeps: studioDependencies{
			batchRepo:         batchRepo,
			batchTaskLinkRepo: linkRepo,
		},
		taskDeps: taskDependencies{
			taskSubmitter: &studioBatchTaskSubmitterStub{},
		},
	}

	first, err := svc.CreateStudioBatchTasks(ctx, "batch-1", &CreateStudioBatchTasksRequest{DesignIDs: []string{"design-1"}})
	if err != nil {
		t.Fatalf("first CreateStudioBatchTasks() error = %v", err)
	}
	if len(first.CreatedTasks) != 1 {
		t.Fatalf("first created tasks = %+v, want 1", first.CreatedTasks)
	}

	second, err := svc.CreateStudioBatchTasks(ctx, "batch-1", &CreateStudioBatchTasksRequest{DesignIDs: []string{"design-1"}})
	if err != nil {
		t.Fatalf("second CreateStudioBatchTasks() error = %v", err)
	}
	if len(second.CreatedTasks) != 0 {
		t.Fatalf("second created tasks = %+v, want no newly created task", second.CreatedTasks)
	}
	if len(second.ReusedTasks) != 1 {
		t.Fatalf("second reused tasks = %+v, want 1 reused durable task", second.ReusedTasks)
	}
	if second.ReusedTasks[0].ID != first.CreatedTasks[0].ID {
		t.Fatalf("second reused task id = %q, want %q", second.ReusedTasks[0].ID, first.CreatedTasks[0].ID)
	}
	if got := taskRepo.taskCount(); got != 1 {
		t.Fatalf("persisted task count = %d, want 1", got)
	}
}

func TestServiceCreateStudioBatchTasks_ReusesLegacyLinkWriteFailureSurfacesFailedTask(t *testing.T) {
	t.Parallel()

	batchRepo := NewMemStudioBatchRepository()
	taskRepo := newStudioBatchTaskRepositoryStub()
	linkRepo := &failingStudioBatchTaskLinkRepository{delegate: NewMemStudioBatchTaskLinkRepository(), failCreate: true}
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	if err := batchRepo.CreateStudioBatchGraph(ctx, newStudioBatchRecordForTest("batch-1", now), newStudioBatchItemsForTest("batch-1", now), newStudioBatchAttemptsForTest("item-1", now), []StudioMaterializedDesignRecord{
		{
			ID:              "design-1",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			ImageURL:        "https://cdn.example.com/design-1.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusApproved,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	taskRepo.tasks["legacy-task-1"] = &Task{
		ID:     "legacy-task-1",
		Status: TaskStatusPending,
		Request: &GenerateRequest{
			ImageURLs: []string{"https://cdn.example.com/design-1.png"},
			Options: &GenerateOptions{
				SheinStudio: &SheinStudioOptions{StyleID: buildStudioBatchTaskStyleID("design-1")},
				SDS: &SDSSyncOptions{
					VariantID:        3003,
					ParentProductID:  2002,
					PrototypeGroupID: 4004,
					LayerID:          "layer-1",
				},
			},
		},
	}
	sessionRepo := &studioBatchTaskCreationSessionRepoStub{session: &SheinStudioSession{
		ID: "batch-1",
		Selection: SheinStudioSelectionSnapshot{
			ParentProductID:  2002,
			VariantID:        3003,
			PrototypeGroupID: 4004,
			LayerID:          "layer-1",
		},
		CreatedTasks: SheinStudioCreatedTaskList{{ID: "legacy-task-1", DesignID: "design-1", Title: "Legacy style"}},
	}}
	svc := &service{
		repo: taskRepo,
		studioDeps: studioDependencies{
			batchRepo:         batchRepo,
			batchTaskLinkRepo: linkRepo,
			sessionRepo:       sessionRepo,
		},
		taskDeps: taskDependencies{taskSubmitter: &studioBatchTaskSubmitterStub{}},
	}

	result, err := svc.CreateStudioBatchTasks(ctx, "batch-1", &CreateStudioBatchTasksRequest{DesignIDs: []string{"design-1"}})
	if err != nil {
		t.Fatalf("CreateStudioBatchTasks() error = %v", err)
	}
	if len(result.CreatedTasks) != 0 {
		t.Fatalf("created tasks = %+v, want none when durable legacy backfill fails", result.CreatedTasks)
	}
	if len(result.FailedTasks) != 1 || !strings.Contains(result.FailedTasks[0].Message, "forced link create failure") {
		t.Fatalf("failed tasks = %+v, want surfaced durable link write failure", result.FailedTasks)
	}
	if got := taskRepo.taskCount(); got != 1 {
		t.Fatalf("persisted task count = %d, want no new duplicate task", got)
	}
}

func TestServiceCreateStudioBatchTasks_ConcurrentRequestsCreateOneTask(t *testing.T) {
	t.Parallel()

	batchRepo := NewMemStudioBatchRepository()
	taskRepo := newStudioBatchTaskRepositoryStub()
	linkRepo := NewMemStudioBatchTaskLinkRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	if err := batchRepo.CreateStudioBatchGraph(ctx, newStudioBatchRecordForTest("batch-1", now), newStudioBatchItemsForTest("batch-1", now), newStudioBatchAttemptsForTest("item-1", now), []StudioMaterializedDesignRecord{
		{
			ID:              "design-1",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			ImageURL:        "https://cdn.example.com/design-1.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusApproved,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	svc := &service{
		repo: taskRepo,
		studioDeps: studioDependencies{
			batchRepo:         batchRepo,
			batchTaskLinkRepo: linkRepo,
		},
		taskDeps: taskDependencies{
			taskSubmitter: &studioBatchTaskSubmitterStub{},
		},
	}

	var wg sync.WaitGroup
	results := make([]*CreateStudioBatchTasksResult, 2)
	errs := make([]error, 2)
	for index := 0; index < 2; index++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			results[index], errs[index] = svc.CreateStudioBatchTasks(ctx, "batch-1", &CreateStudioBatchTasksRequest{DesignIDs: []string{"design-1"}})
		}(index)
	}
	wg.Wait()

	for index, err := range errs {
		if err != nil {
			t.Fatalf("CreateStudioBatchTasks(%d) error = %v", index, err)
		}
		if ids := studioBatchResultTaskIDs(results[index]); len(ids) != 1 {
			t.Fatalf("CreateStudioBatchTasks(%d) result = %+v, want one task", index, results[index])
		}
	}
	leftIDs := studioBatchResultTaskIDs(results[0])
	rightIDs := studioBatchResultTaskIDs(results[1])
	if leftIDs[0] != rightIDs[0] {
		t.Fatalf("concurrent task ids = %q and %q, want same task", leftIDs[0], rightIDs[0])
	}
	if got := taskRepo.taskCount(); got != 1 {
		t.Fatalf("persisted task count = %d, want 1", got)
	}
}

func TestServiceCreateStudioBatchTasks_ConcurrentSlowOwnerReturnsOneTask(t *testing.T) {
	t.Parallel()

	batchRepo := NewMemStudioBatchRepository()
	taskRepo := newStudioBatchTaskRepositoryStub()
	linkRepo := NewMemStudioBatchTaskLinkRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	batch := newStudioBatchRecordForTest("batch-1", now)
	batch.GroupedSelections = SheinStudioGroupedSelectionList{
		studioBatchFanOutSelection("selection-1", 3001, "Red", "9001", "https://cdn.example.com/template.png", "https://cdn.example.com/mask.png"),
	}
	items := newStudioBatchItemsForTest("batch-1", now)
	items[0].SelectionIDs = SheinStudioStringList{"selection-1"}
	items[0].SelectionCount = 1
	if err := batchRepo.CreateStudioBatchGraph(ctx, batch, items, newStudioBatchAttemptsForTest("item-1", now), []StudioMaterializedDesignRecord{
		{
			ID:              "design-1",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			ImageURL:        "https://cdn.example.com/design-1.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusApproved,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	var createCalls atomic.Int64
	started := make(chan struct{})
	release := make(chan struct{})
	var startedOnce sync.Once
	svc := newTaskStudioBatchService(taskStudioBatchServiceConfig{
		repo:              batchRepo,
		batchTaskLinkRepo: linkRepo,
		createGenerateTask: func(ctx context.Context, req *GenerateRequest) (*Task, error) {
			startedOnce.Do(func() { close(started) })
			createCalls.Add(1)
			<-release
			task := &Task{ID: "slow-task-1", Status: TaskStatusPending, Request: req, CreatedAt: now, UpdatedAt: now}
			if err := taskRepo.CreateTask(ctx, task); err != nil {
				return nil, err
			}
			return task, nil
		},
		getTask: taskRepo.GetTask,
	})

	var wg sync.WaitGroup
	results := make([]*CreateStudioBatchTasksResult, 2)
	errs := make([]error, 2)
	firstDone := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(firstDone)
		results[0], errs[0] = svc.CreateStudioBatchTasks(ctx, "batch-1", &CreateStudioBatchTasksRequest{DesignIDs: []string{"design-1"}})
	}()
	select {
	case <-started:
	case <-firstDone:
		t.Fatalf("first CreateStudioBatchTasks returned before task creation started: result=%+v err=%v", results[0], errs[0])
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for first task creation to start")
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		results[1], errs[1] = svc.CreateStudioBatchTasks(ctx, "batch-1", &CreateStudioBatchTasksRequest{DesignIDs: []string{"design-1"}})
	}()
	time.Sleep(200 * time.Millisecond)
	close(release)
	wg.Wait()

	for index, err := range errs {
		if err != nil {
			t.Fatalf("CreateStudioBatchTasks(%d) error = %v", index, err)
		}
		if ids := studioBatchResultTaskIDs(results[index]); len(ids) != 1 {
			t.Fatalf("CreateStudioBatchTasks(%d) result = %+v, want one task", index, results[index])
		}
	}
	leftIDs := studioBatchResultTaskIDs(results[0])
	rightIDs := studioBatchResultTaskIDs(results[1])
	if leftIDs[0] != rightIDs[0] {
		t.Fatalf("concurrent task ids = %q and %q, want same task", leftIDs[0], rightIDs[0])
	}
	if got := createCalls.Load(); got != 1 {
		t.Fatalf("create calls = %d, want 1", got)
	}
}

func TestServiceCreateStudioBatchTasks_ConcurrentStaleCreatingRecoveryCreatesOneTask(t *testing.T) {
	t.Parallel()

	batchRepo := NewMemStudioBatchRepository()
	taskRepo := newStudioBatchTaskRepositoryStub()
	baseLinkRepo := NewMemStudioBatchTaskLinkRepository()
	linkRepo := newSynchronizedStaleCreatingLinkRepository(baseLinkRepo)
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	batch := newStudioBatchRecordForTest("batch-1", now)
	if err := batchRepo.CreateStudioBatchGraph(ctx, batch, newStudioBatchItemsForTest("batch-1", now), newStudioBatchAttemptsForTest("item-1", now), []StudioMaterializedDesignRecord{
		{
			ID:              "design-1",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			ImageURL:        "https://cdn.example.com/design-1.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusApproved,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}
	selection := batch.GroupedSelections[0]
	candidateKey := buildStudioBatchTaskCandidateKey(ctx, batch, studioBatchTaskCandidate{
		Design:                   StudioMaterializedDesignRecord{ID: "design-1"},
		Item:                     StudioBatchItemRecord{ID: "item-1"},
		Selection:                selection,
		SelectionSnapshot:        selection.Selection,
		SelectionID:              "selection-1",
		CompatibilityFingerprint: buildStudioBatchCompatibilityFingerprint(selection.Selection),
		SheinStoreID:             9001,
	})
	linkRepo.candidateKey = candidateKey
	if err := baseLinkRepo.CreateStudioBatchTaskLink(ctx, &StudioBatchTaskLinkRecord{
		ID:                       "creating-link-1",
		BatchID:                  "batch-1",
		ItemID:                   "item-1",
		DesignID:                 "design-1",
		SelectionID:              "selection-1",
		SheinStoreID:             9001,
		CompatibilityFingerprint: buildStudioBatchCompatibilityFingerprint(selection.Selection),
		CandidateKey:             candidateKey,
		Status:                   studioBatchTaskLinkStatusCreating,
		CreatedAt:                now.Add(-10 * time.Minute),
		UpdatedAt:                now.Add(-10 * time.Minute),
	}); err != nil {
		t.Fatalf("CreateStudioBatchTaskLink(creating) error = %v", err)
	}

	var sequence atomic.Int64
	svc := newTaskStudioBatchService(taskStudioBatchServiceConfig{
		repo:              batchRepo,
		batchTaskLinkRepo: linkRepo,
		createGenerateTask: func(ctx context.Context, req *GenerateRequest) (*Task, error) {
			id := fmt.Sprintf("stale-recovery-task-%d", sequence.Add(1))
			task := &Task{ID: id, Status: TaskStatusPending, Request: req, CreatedAt: now, UpdatedAt: now}
			if err := taskRepo.CreateTask(ctx, task); err != nil {
				return nil, err
			}
			return task, nil
		},
		getTask: taskRepo.GetTask,
	})

	var wg sync.WaitGroup
	results := make([]*CreateStudioBatchTasksResult, 2)
	errs := make([]error, 2)
	for index := range results {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			results[index], errs[index] = svc.CreateStudioBatchTasks(ctx, "batch-1", &CreateStudioBatchTasksRequest{DesignIDs: []string{"design-1"}})
		}(index)
	}
	wg.Wait()

	for index, err := range errs {
		if err != nil {
			t.Fatalf("CreateStudioBatchTasks(%d) error = %v", index, err)
		}
		if results[index] == nil || len(results[index].CreatedTasks) != 1 {
			t.Fatalf("CreateStudioBatchTasks(%d) result = %+v, want one task", index, results[index])
		}
	}
	if results[0].CreatedTasks[0].ID != results[1].CreatedTasks[0].ID {
		t.Fatalf("stale recovery task ids = %q and %q, want same task", results[0].CreatedTasks[0].ID, results[1].CreatedTasks[0].ID)
	}
	if got := taskRepo.taskCount(); got != 1 {
		t.Fatalf("persisted task count = %d, want exactly one task", got)
	}
}

func TestStudioBatchDetail_LoadsCreatedTasksFromDurableLinks(t *testing.T) {
	t.Parallel()

	batchRepo := NewMemStudioBatchRepository()
	taskRepo := newStudioBatchTaskRepositoryStub()
	linkRepo := NewMemStudioBatchTaskLinkRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	if err := batchRepo.CreateStudioBatchGraph(ctx, newStudioBatchRecordForTest("batch-1", now), newStudioBatchItemsForTest("batch-1", now), newStudioBatchAttemptsForTest("item-1", now), []StudioMaterializedDesignRecord{
		{
			ID:              "design-1",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			ImageURL:        "https://cdn.example.com/design-1.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusApproved,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	svc := &service{
		repo: taskRepo,
		studioDeps: studioDependencies{
			batchRepo:         batchRepo,
			batchTaskLinkRepo: linkRepo,
		},
		taskDeps: taskDependencies{
			taskSubmitter: &studioBatchTaskSubmitterStub{},
		},
	}
	created, err := svc.CreateStudioBatchTasks(ctx, "batch-1", &CreateStudioBatchTasksRequest{DesignIDs: []string{"design-1"}})
	if err != nil {
		t.Fatalf("CreateStudioBatchTasks() error = %v", err)
	}
	if len(created.CreatedTasks) != 1 {
		t.Fatalf("created tasks = %+v, want 1", created.CreatedTasks)
	}
	if got := created.CreatedTasks[0].Source; got != studioBatchTaskLinkSourceBatchCreated {
		t.Fatalf("created task source = %q, want %q", got, studioBatchTaskLinkSourceBatchCreated)
	}

	detail, err := svc.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}
	if len(detail.CreatedTasks) != 1 {
		t.Fatalf("detail created tasks = %+v, want durable linked task", detail.CreatedTasks)
	}
	if detail.CreatedTasks[0].ID != created.CreatedTasks[0].ID {
		t.Fatalf("detail task id = %q, want %q", detail.CreatedTasks[0].ID, created.CreatedTasks[0].ID)
	}
	if detail.CreatedTasks[0].Status != "task_created" {
		t.Fatalf("detail task status = %q, want task_created", detail.CreatedTasks[0].Status)
	}
	if got := detail.CreatedTasks[0].Source; got != studioBatchTaskLinkSourceBatchCreated {
		t.Fatalf("detail task source = %q, want %q", got, studioBatchTaskLinkSourceBatchCreated)
	}
}

func TestStudioBatchDetail_LoadsCreatedTasksPreservesLegacyMetadataFromDurableLinks(t *testing.T) {
	t.Parallel()

	batchRepo := NewMemStudioBatchRepository()
	taskRepo := newStudioBatchTaskRepositoryStub()
	linkRepo := NewMemStudioBatchTaskLinkRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	if err := batchRepo.CreateStudioBatchGraph(ctx, newStudioBatchRecordForTest("batch-1", now), newStudioBatchItemsForTest("batch-1", now), newStudioBatchAttemptsForTest("item-1", now), []StudioMaterializedDesignRecord{
		{
			ID:              "design-1",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			ImageURL:        "https://cdn.example.com/design-1.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusApproved,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}
	taskRepo.tasks["task-1"] = &Task{
		ID:     "task-1",
		Status: TaskStatusPending,
		Request: &GenerateRequest{Options: &GenerateOptions{
			SheinStudio: &SheinStudioOptions{StyleID: "legacy-style-1"},
		}},
	}
	mustCreateStudioBatchTaskLinkForTest(t, linkRepo, ctx, &StudioBatchTaskLinkRecord{
		ID:               "link-1",
		BatchID:          "batch-1",
		ItemID:           "item-1",
		DesignID:         "design-1",
		SelectionID:      "selection-1",
		SheinStoreID:     9001,
		ListingKitTaskID: "task-1",
		CandidateKey:     "candidate-1",
		Status:           studioBatchTaskLinkStatusCreated,
		CreatedAt:        now,
		UpdatedAt:        now,
	})
	sessionRepo := &studioBatchTaskCreationSessionRepoStub{session: &SheinStudioSession{
		ID:           "batch-1",
		SavedAsBatch: true,
		UpdatedAt:    now,
		CreatedTasks: SheinStudioCreatedTaskList{{
			ID:                       "task-1",
			Title:                    "Rich legacy title",
			DesignID:                 "design-1",
			ItemID:                   "item-1",
			SelectionID:              "selection-1",
			CompatibilityFingerprint: "legacy-fingerprint",
			Status:                   "ready_to_submit",
			Message:                  "legacy message",
		}},
	}}
	svc := &service{
		repo: taskRepo,
		studioDeps: studioDependencies{
			batchRepo:         batchRepo,
			batchTaskLinkRepo: linkRepo,
			sessionRepo:       sessionRepo,
		},
		taskDeps: taskDependencies{taskSubmitter: &studioBatchTaskSubmitterStub{}},
	}

	detail, err := svc.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}
	if len(detail.CreatedTasks) != 1 {
		t.Fatalf("created tasks = %+v, want one deduped task", detail.CreatedTasks)
	}
	if got := detail.CreatedTasks[0].Title; got != "Rich legacy title" {
		t.Fatalf("title = %q, want richer legacy title", got)
	}
	if got := detail.CreatedTasks[0].Message; got != "legacy message" {
		t.Fatalf("message = %q, want richer legacy message", got)
	}
	if got := detail.CreatedTasks[0].Source; got != studioBatchTaskLinkSourceLegacySessionBackfilled {
		t.Fatalf("inferred source = %q, want %q", got, studioBatchTaskLinkSourceLegacySessionBackfilled)
	}
}

func TestStudioBatchDetail_ProjectsCreatedTaskSubmissionStateFromListingKitTask(t *testing.T) {
	t.Parallel()

	batchRepo := NewMemStudioBatchRepository()
	taskRepo := newStudioBatchTaskRepositoryStub()
	linkRepo := NewMemStudioBatchTaskLinkRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	if err := batchRepo.CreateStudioBatchGraph(ctx, newStudioBatchRecordForTest("batch-1", now), newStudioBatchItemsForTest("batch-1", now), newStudioBatchAttemptsForTest("item-1", now), []StudioMaterializedDesignRecord{
		{
			ID:              "design-1",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			ImageURL:        "https://cdn.example.com/design-1.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusApproved,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}
	taskRepo.tasks["task-1"] = &Task{
		ID:     "task-1",
		Status: TaskStatusCompleted,
		Result: &ListingKitResult{
			Shein: &SheinPackage{
				Submission: &sheinpub.SubmissionReport{
					LastAction: "save_draft",
					LastStatus: sheinpub.SubmissionStatusSuccess,
				},
			},
		},
	}
	mustCreateStudioBatchTaskLinkForTest(t, linkRepo, ctx, &StudioBatchTaskLinkRecord{
		ID:                       "link-1",
		BatchID:                  "batch-1",
		ItemID:                   "item-1",
		DesignID:                 "design-1",
		SelectionID:              "selection-1",
		CompatibilityFingerprint: "compat-1",
		SheinStoreID:             9001,
		ListingKitTaskID:         "task-1",
		CandidateKey:             "candidate-1",
		Status:                   studioBatchTaskLinkStatusCreated,
		CreatedAt:                now,
		UpdatedAt:                now,
	})
	svc := &service{
		repo: taskRepo,
		studioDeps: studioDependencies{
			batchRepo:         batchRepo,
			batchTaskLinkRepo: linkRepo,
		},
		taskDeps: taskDependencies{taskSubmitter: &studioBatchTaskSubmitterStub{}},
	}

	detail, err := svc.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}
	if len(detail.CreatedTasks) != 1 {
		t.Fatalf("created tasks = %+v, want one task", detail.CreatedTasks)
	}
	task := detail.CreatedTasks[0]
	if task.ItemID != "item-1" || task.SelectionID != "selection-1" || task.CompatibilityFingerprint != "compat-1" {
		t.Fatalf("created task identity = %+v, want item/selection/fingerprint from durable link", task)
	}
	if task.Status != "draft_saved" {
		t.Fatalf("status = %q, want draft_saved from real submission state", task.Status)
	}
	if task.SubmissionState != sheinpub.SubmissionStatusSuccess {
		t.Fatalf("submission_state = %q, want success", task.SubmissionState)
	}
	if task.LastSubmissionAction != "save_draft" {
		t.Fatalf("last_submission_action = %q, want save_draft", task.LastSubmissionAction)
	}
}

func TestServiceCreateStudioBatchTasks_RecoversReservedCandidate(t *testing.T) {
	t.Parallel()

	batchRepo := NewMemStudioBatchRepository()
	taskRepo := newStudioBatchTaskRepositoryStub()
	linkRepo := NewMemStudioBatchTaskLinkRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	batch := newStudioBatchRecordForTest("batch-1", now)
	if err := batchRepo.CreateStudioBatchGraph(ctx, batch, newStudioBatchItemsForTest("batch-1", now), newStudioBatchAttemptsForTest("item-1", now), []StudioMaterializedDesignRecord{
		{
			ID:              "design-1",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			ImageURL:        "https://cdn.example.com/design-1.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusApproved,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	selection := batch.GroupedSelections[0]
	candidateKey := buildStudioBatchTaskCandidateKey(ctx, batch, studioBatchTaskCandidate{
		Design:                   StudioMaterializedDesignRecord{ID: "design-1"},
		Item:                     StudioBatchItemRecord{ID: "item-1"},
		Selection:                selection,
		SelectionSnapshot:        selection.Selection,
		SelectionID:              "selection-1",
		CompatibilityFingerprint: buildStudioBatchCompatibilityFingerprint(selection.Selection),
		SheinStoreID:             9001,
	})
	if err := linkRepo.CreateStudioBatchTaskLink(ctx, &StudioBatchTaskLinkRecord{
		ID:                       "reserved-link-1",
		BatchID:                  "batch-1",
		ItemID:                   "item-1",
		DesignID:                 "design-1",
		SelectionID:              "selection-1",
		CompatibilityFingerprint: buildStudioBatchCompatibilityFingerprint(selection.Selection),
		SheinStoreID:             9001,
		CandidateKey:             candidateKey,
		Status:                   "reserved",
		CreatedAt:                now.Add(-time.Minute),
		UpdatedAt:                now.Add(-time.Minute),
	}); err != nil {
		t.Fatalf("CreateStudioBatchTaskLink(reserved) error = %v", err)
	}

	svc := &service{
		repo: taskRepo,
		studioDeps: studioDependencies{
			batchRepo:         batchRepo,
			batchTaskLinkRepo: linkRepo,
		},
		taskDeps: taskDependencies{
			taskSubmitter: &studioBatchTaskSubmitterStub{},
		},
	}

	result, err := svc.CreateStudioBatchTasks(ctx, "batch-1", &CreateStudioBatchTasksRequest{DesignIDs: []string{"design-1"}})
	if err != nil {
		t.Fatalf("CreateStudioBatchTasks() error = %v", err)
	}
	if len(result.CreatedTasks) != 1 {
		t.Fatalf("created tasks = %+v, want one recovered reserved candidate", result.CreatedTasks)
	}
	link, err := linkRepo.GetStudioBatchTaskLinkByCandidateKey(ctx, candidateKey)
	if err != nil {
		t.Fatalf("GetStudioBatchTaskLinkByCandidateKey() error = %v", err)
	}
	if link.Status != "created" || link.ListingKitTaskID != result.CreatedTasks[0].ID {
		t.Fatalf("link after recovery = %+v, want created with task id %q", link, result.CreatedTasks[0].ID)
	}
}

func TestServiceCreateStudioBatchTasks_RecoversPostPersistDispatchFailureWithoutDuplicate(t *testing.T) {
	t.Parallel()

	batchRepo := NewMemStudioBatchRepository()
	taskRepo := newStudioBatchTaskRepositoryStub()
	linkRepo := NewMemStudioBatchTaskLinkRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	if err := batchRepo.CreateStudioBatchGraph(ctx, newStudioBatchRecordForTest("batch-1", now), newStudioBatchItemsForTest("batch-1", now), newStudioBatchAttemptsForTest("item-1", now), []StudioMaterializedDesignRecord{
		{
			ID:              "design-1",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			ImageURL:        "https://cdn.example.com/design-1.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusApproved,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}
	submitter := &studioBatchMutableSubmitter{err: fmt.Errorf("forced dispatch failure")}
	svc := &service{
		repo: taskRepo,
		studioDeps: studioDependencies{
			batchRepo:         batchRepo,
			batchTaskLinkRepo: linkRepo,
		},
		taskDeps: taskDependencies{taskSubmitter: submitter},
	}

	first, err := svc.CreateStudioBatchTasks(ctx, "batch-1", &CreateStudioBatchTasksRequest{DesignIDs: []string{"design-1"}})
	if err != nil {
		t.Fatalf("first CreateStudioBatchTasks() error = %v", err)
	}
	if len(first.FailedTasks) != 1 {
		t.Fatalf("first failed tasks = %+v, want dispatch failure", first.FailedTasks)
	}
	if got := taskRepo.taskCount(); got != 1 {
		t.Fatalf("task count after dispatch failure = %d, want persisted failed task", got)
	}
	links, err := linkRepo.ListStudioBatchTaskLinksByBatchID(ctx, "batch-1")
	if err != nil {
		t.Fatalf("ListStudioBatchTaskLinksByBatchID() error = %v", err)
	}
	if len(links) != 1 || strings.TrimSpace(links[0].ListingKitTaskID) == "" {
		t.Fatalf("links after dispatch failure = %+v, want captured persisted task id", links)
	}
	firstTaskID := links[0].ListingKitTaskID
	submitter.err = nil
	second, err := svc.CreateStudioBatchTasks(ctx, "batch-1", &CreateStudioBatchTasksRequest{DesignIDs: []string{"design-1"}})
	if err != nil {
		t.Fatalf("second CreateStudioBatchTasks() error = %v", err)
	}
	secondTaskIDs := studioBatchResultTaskIDs(second)
	if len(secondTaskIDs) != 1 || secondTaskIDs[0] != firstTaskID {
		t.Fatalf("second task ids = %+v, want existing persisted task %q", secondTaskIDs, firstTaskID)
	}
	if got := taskRepo.taskCount(); got != 1 {
		t.Fatalf("task count after retry = %d, want no duplicate task", got)
	}
}

func TestServiceCreateStudioBatchTasks_RecoversStaleCreatingCandidate(t *testing.T) {
	t.Parallel()

	batchRepo := NewMemStudioBatchRepository()
	taskRepo := newStudioBatchTaskRepositoryStub()
	linkRepo := NewMemStudioBatchTaskLinkRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	batch := newStudioBatchRecordForTest("batch-1", now)
	if err := batchRepo.CreateStudioBatchGraph(ctx, batch, newStudioBatchItemsForTest("batch-1", now), newStudioBatchAttemptsForTest("item-1", now), []StudioMaterializedDesignRecord{
		{
			ID:              "design-1",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			ImageURL:        "https://cdn.example.com/design-1.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusApproved,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}
	selection := batch.GroupedSelections[0]
	candidateKey := buildStudioBatchTaskCandidateKey(ctx, batch, studioBatchTaskCandidate{
		Design:                   StudioMaterializedDesignRecord{ID: "design-1"},
		Item:                     StudioBatchItemRecord{ID: "item-1"},
		Selection:                selection,
		SelectionSnapshot:        selection.Selection,
		SelectionID:              "selection-1",
		CompatibilityFingerprint: buildStudioBatchCompatibilityFingerprint(selection.Selection),
		SheinStoreID:             9001,
	})
	if err := linkRepo.CreateStudioBatchTaskLink(ctx, &StudioBatchTaskLinkRecord{
		ID:                       "creating-link-1",
		BatchID:                  "batch-1",
		ItemID:                   "item-1",
		DesignID:                 "design-1",
		SelectionID:              "selection-1",
		CompatibilityFingerprint: buildStudioBatchCompatibilityFingerprint(selection.Selection),
		SheinStoreID:             9001,
		CandidateKey:             candidateKey,
		Status:                   studioBatchTaskLinkStatusCreating,
		CreatedAt:                now.Add(-10 * time.Minute),
		UpdatedAt:                now.Add(-10 * time.Minute),
	}); err != nil {
		t.Fatalf("CreateStudioBatchTaskLink(creating) error = %v", err)
	}
	svc := newTaskStudioBatchService(taskStudioBatchServiceConfig{
		repo:              batchRepo,
		batchTaskLinkRepo: linkRepo,
		createGenerateTask: func(ctx context.Context, req *GenerateRequest) (*Task, error) {
			task := &Task{ID: "recovered-task-1", Status: TaskStatusPending, Request: req, CreatedAt: now, UpdatedAt: now}
			if err := taskRepo.CreateTask(ctx, task); err != nil {
				return nil, err
			}
			return task, nil
		},
		getTask: taskRepo.GetTask,
	})

	result, err := svc.CreateStudioBatchTasks(ctx, "batch-1", &CreateStudioBatchTasksRequest{DesignIDs: []string{"design-1"}})
	if err != nil {
		t.Fatalf("CreateStudioBatchTasks() error = %v", err)
	}
	if len(result.CreatedTasks) != 1 || result.CreatedTasks[0].ID != "recovered-task-1" {
		t.Fatalf("created tasks = %+v, want recovered new task", result.CreatedTasks)
	}
	link, err := linkRepo.GetStudioBatchTaskLinkByCandidateKey(ctx, candidateKey)
	if err != nil {
		t.Fatalf("GetStudioBatchTaskLinkByCandidateKey() error = %v", err)
	}
	if link.Status != studioBatchTaskLinkStatusCreated || link.ListingKitTaskID != "recovered-task-1" {
		t.Fatalf("link after stale recovery = %+v, want created recovered-task-1", link)
	}
}

func TestServiceCreateStudioBatchTasksRejectsUnapprovedDesignIDs(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	if err := repo.CreateStudioBatchGraph(ctx, newStudioBatchRecordForTest("batch-1", now), newStudioBatchItemsForTest("batch-1", now), newStudioBatchAttemptsForTest("item-1", now), []StudioMaterializedDesignRecord{
		{
			ID:              "design-1",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			ImageURL:        "https://cdn.example.com/design-1.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusUnreviewed,
			SortOrder:       0,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	svc := &service{
		repo: newStudioBatchTaskRepositoryStub(),

		studioDeps: studioDependencies{
			sessionRepo: &studioBatchTaskCreationSessionRepoStub{
				session: &SheinStudioSession{
					ID:            "batch-1",
					Prompt:        "retro cherries",
					ImageStrategy: "sds_official",
					Selection: SheinStudioSelectionSnapshot{
						ProductID:        1001,
						ParentProductID:  2002,
						VariantID:        3003,
						PrototypeGroupID: 4004,
						LayerID:          "layer-1",
						ProductName:      "Canvas Tote",
						PrintableWidth:   1200,
						PrintableHeight:  1200,
					},
				},
			}, batchRepo: repo,
		},
		taskDeps: taskDependencies{
			taskSubmitter: &studioBatchTaskSubmitterStub{},
		},
	}
	if _, err := svc.CreateStudioBatchTasks(ctx, "batch-1", &CreateStudioBatchTasksRequest{
		DesignIDs: []string{"design-1"},
	}); err == nil {
		t.Fatal("CreateStudioBatchTasks() error = nil, want approved-design validation failure")
	}
}

func TestServiceCreateStudioBatchTasksReusesLegacyStyleIDTasks(t *testing.T) {
	t.Parallel()

	batchRepo := NewMemStudioBatchRepository()
	taskRepo := newStudioBatchTaskRepositoryStub()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	if err := batchRepo.CreateStudioBatchGraph(ctx, newStudioBatchRecordForTest("batch-1", now), newStudioBatchItemsForTest("batch-1", now), newStudioBatchAttemptsForTest("item-1", now), []StudioMaterializedDesignRecord{
		{
			ID:               "design-1",
			BatchID:          "batch-1",
			ItemID:           "item-1",
			SourceAttemptID:  "attempt-1",
			ImageURL:         "https://cdn.example.com/design-1.png",
			ReviewStatus:     StudioMaterializedDesignReviewStatusApproved,
			TargetGroupKey:   "size:1200x1200",
			TargetGroupLabel: "Style 1",
			SortOrder:        0,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
	}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	sessionRepo := &studioBatchTaskCreationSessionRepoStub{
		session: &SheinStudioSession{
			ID:            "batch-1",
			Prompt:        "retro cherries",
			ImageStrategy: "sds_official",
			Selection: SheinStudioSelectionSnapshot{
				ProductID:        1001,
				ParentProductID:  2002,
				VariantID:        3003,
				PrototypeGroupID: 4004,
				LayerID:          "layer-1",
				ProductName:      "Canvas Tote",
				PrintableWidth:   1200,
				PrintableHeight:  1200,
			},
		},
	}
	taskRepo.tasks["legacy-task-1"] = &Task{
		ID:     "legacy-task-1",
		Status: TaskStatusPending,
		Request: &GenerateRequest{
			ImageURLs: []string{"https://cdn.example.com/design-1.png"},
			Options: &GenerateOptions{
				SheinStudio: &SheinStudioOptions{
					StyleID: buildStudioBatchTaskStyleID("design-1"),
				},
				SDS: &SDSSyncOptions{
					VariantID:        3003,
					ParentProductID:  7001,
					PrototypeGroupID: 9001,
					LayerID:          "layer-1",
				},
			},
		},
	}
	sessionRepo.session.CreatedTasks = SheinStudioCreatedTaskList{
		{
			ID:       "legacy-task-1",
			Title:    "Style 1",
			DesignID: "design-1",
		},
	}

	linkRepo := NewMemStudioBatchTaskLinkRepository()
	svc := &service{
		repo: taskRepo,
		studioDeps: studioDependencies{
			sessionRepo: sessionRepo, batchRepo: batchRepo, batchTaskLinkRepo: linkRepo,
		},
		taskDeps: taskDependencies{
			taskSubmitter: &studioBatchTaskSubmitterStub{},
		},
	}

	result, err := svc.CreateStudioBatchTasks(ctx, "batch-1", &CreateStudioBatchTasksRequest{
		DesignIDs: []string{"design-1"},
	})
	if err != nil {
		t.Fatalf("CreateStudioBatchTasks() error = %v", err)
	}
	resultTaskIDs := studioBatchResultTaskIDs(result)
	if len(resultTaskIDs) != 1 {
		t.Fatalf("task ids = %+v, want 1 reused task", resultTaskIDs)
	}
	if got := resultTaskIDs[0]; got != "legacy-task-1" {
		t.Fatalf("task id = %q, want legacy-task-1", got)
	}
	if len(result.ReusedTasks) != 1 {
		t.Fatalf("reused tasks = %+v, want one legacy reused task", result.ReusedTasks)
	}
	if got := result.ReusedTasks[0].Source; got != studioBatchTaskLinkSourceLegacySessionBackfilled {
		t.Fatalf("reused task source = %q, want %q", got, studioBatchTaskLinkSourceLegacySessionBackfilled)
	}
	if got := len(taskRepo.tasks); got != 1 {
		t.Fatalf("persisted task count = %d, want 1 without duplicates", got)
	}
	links, err := linkRepo.ListStudioBatchTaskLinksByBatchID(ctx, "batch-1")
	if err != nil {
		t.Fatalf("ListStudioBatchTaskLinksByBatchID() error = %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("links = %+v, want one legacy backfill link", links)
	}
	if got := links[0].Source; got != studioBatchTaskLinkSourceLegacySessionBackfilled {
		t.Fatalf("legacy link source = %q, want %q", got, studioBatchTaskLinkSourceLegacySessionBackfilled)
	}
}

type studioBatchTaskCreationSessionRepoStub struct {
	session *SheinStudioSession
}

func (s *studioBatchTaskCreationSessionRepoStub) FindLatestSessionBySelectionKey(context.Context, string) (*SheinStudioSession, error) {
	return nil, nil
}

func (s *studioBatchTaskCreationSessionRepoStub) CreateSession(context.Context, *SheinStudioSession) error {
	return nil
}

func (s *studioBatchTaskCreationSessionRepoStub) GetSession(context.Context, string) (*SheinStudioSession, error) {
	if s.session == nil {
		return nil, nil
	}
	cloned := *s.session
	return &cloned, nil
}

func (s *studioBatchTaskCreationSessionRepoStub) UpdateSession(_ context.Context, session *SheinStudioSession) error {
	if session == nil {
		return nil
	}
	cloned := *session
	s.session = &cloned
	return nil
}

func (s *studioBatchTaskCreationSessionRepoStub) DeleteSession(context.Context, string) error {
	return nil
}

func (s *studioBatchTaskCreationSessionRepoStub) ReplaceDesigns(context.Context, string, []string, []SheinStudioDesign) error {
	return nil
}

func (s *studioBatchTaskCreationSessionRepoStub) UpsertDesigns(context.Context, string, []string, []SheinStudioDesign) error {
	return nil
}

func (s *studioBatchTaskCreationSessionRepoStub) ListSessionDesigns(context.Context, string) ([]SheinStudioDesign, error) {
	return nil, nil
}

func (s *studioBatchTaskCreationSessionRepoStub) CountSessionDesignsBySessionIDs(context.Context, []string) (map[string]int, error) {
	return nil, nil
}

func (s *studioBatchTaskCreationSessionRepoStub) ListGalleryItems(context.Context, int) ([]SheinStudioSessionGalleryItem, error) {
	return nil, nil
}

func (s *studioBatchTaskCreationSessionRepoStub) ListBatchSessions(context.Context, int) ([]SheinStudioSession, error) {
	return nil, nil
}

func (s *studioBatchTaskCreationSessionRepoStub) ListTenantBatchNames(context.Context) ([]string, error) {
	return nil, nil
}

func studioBatchFanOutSelection(
	selectionID string,
	variantID int64,
	variantLabel string,
	storeID string,
	templateURL string,
	maskURL string,
) SheinStudioGroupedSelection {
	return SheinStudioGroupedSelection{
		SelectionID:  selectionID,
		SheinStoreID: storeID,
		Eligible:     true,
		Selection: SheinStudioSelection{
			ProductID:          variantID,
			ParentProductID:    7001,
			VariantID:          variantID,
			PrototypeGroupID:   9001,
			LayerID:            "layer-1",
			DesignType:         "material",
			ProductName:        "Canvas Tote",
			VariantLabel:       variantLabel,
			PrintableWidth:     1200,
			PrintableHeight:    1200,
			TemplateImageURL:   templateURL,
			MaskImageURL:       maskURL,
			SelectedVariantIDs: []int64{variantID},
		},
	}
}

func studioBatchResultTaskIDs(result *CreateStudioBatchTasksResult) []string {
	if result == nil {
		return nil
	}
	ids := make([]string, 0, len(result.CreatedTasks)+len(result.ReusedTasks))
	for _, task := range result.CreatedTasks {
		if strings.TrimSpace(task.ID) != "" {
			ids = append(ids, task.ID)
		}
	}
	for _, task := range result.ReusedTasks {
		if strings.TrimSpace(task.ID) != "" {
			ids = append(ids, task.ID)
		}
	}
	return ids
}

type studioBatchTaskRepositoryStub struct {
	mu    sync.Mutex
	tasks map[string]*Task
}

func newStudioBatchTaskRepositoryStub() *studioBatchTaskRepositoryStub {
	return &studioBatchTaskRepositoryStub{tasks: make(map[string]*Task)}
}

func (r *studioBatchTaskRepositoryStub) CreateTask(_ context.Context, task *Task) error {
	if task == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cloned := *task
	r.tasks[task.ID] = &cloned
	return nil
}

func (r *studioBatchTaskRepositoryStub) GetTask(_ context.Context, taskID string) (*Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	cloned := *task
	return &cloned, nil
}

func (r *studioBatchTaskRepositoryStub) taskCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.tasks)
}

func (r *studioBatchTaskRepositoryStub) ListTasks(context.Context, *TaskListQuery) ([]Task, int64, error) {
	return nil, 0, nil
}

func (r *studioBatchTaskRepositoryStub) MarkProcessing(context.Context, string) error { return nil }
func (r *studioBatchTaskRepositoryStub) MarkCompleted(context.Context, string, *ListingKitResult) error {
	return nil
}
func (r *studioBatchTaskRepositoryStub) MarkNeedsReview(context.Context, string, *ListingKitResult, string) error {
	return nil
}
func (r *studioBatchTaskRepositoryStub) MarkFailed(context.Context, string, string) error { return nil }
func (r *studioBatchTaskRepositoryStub) MarkBlockedRetryable(_ context.Context, taskID string, block *RetryableBlock, errorMsg string) error {
	task, ok := r.tasks[taskID]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	task.Status = TaskStatusBlockedRetryable
	task.RetryableBlock = block
	task.Error = errorMsg
	task.UpdatedAt = time.Now()
	return nil
}
func (r *studioBatchTaskRepositoryStub) ListRecoverableTasks(context.Context, *RecoverableTaskQuery) ([]Task, error) {
	return []Task{}, nil
}
func (r *studioBatchTaskRepositoryStub) RecoverBlockedTaskNow(_ context.Context, taskID string, recoveredAt time.Time) error {
	task, ok := r.tasks[taskID]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	task.Status = TaskStatusPending
	task.RetryableBlock = nil
	task.Error = ""
	task.UpdatedAt = recoveredAt
	return nil
}
func (r *studioBatchTaskRepositoryStub) BulkRecoverBlockedTasks(context.Context, *RecoverBlockedTasksQuery) (int64, error) {
	return 0, nil
}
func (r *studioBatchTaskRepositoryStub) PrepareRetry(context.Context, string) error { return nil }
func (r *studioBatchTaskRepositoryStub) IncrementRetryCount(context.Context, string) error {
	return nil
}
func (r *studioBatchTaskRepositoryStub) SaveTaskResult(context.Context, string, *ListingKitResult) error {
	return nil
}

type failingStudioBatchTaskLinkRepository struct {
	delegate   StudioBatchTaskLinkRepository
	failCreate bool
	failUpdate bool
}

func (r *failingStudioBatchTaskLinkRepository) GetStudioBatchTaskLinkByCandidateKey(ctx context.Context, candidateKey string) (*StudioBatchTaskLinkRecord, error) {
	return r.delegate.GetStudioBatchTaskLinkByCandidateKey(ctx, candidateKey)
}

func (r *failingStudioBatchTaskLinkRepository) CreateStudioBatchTaskLink(ctx context.Context, link *StudioBatchTaskLinkRecord) error {
	if r.failCreate {
		return fmt.Errorf("forced link create failure")
	}
	return r.delegate.CreateStudioBatchTaskLink(ctx, link)
}

func (r *failingStudioBatchTaskLinkRepository) UpdateStudioBatchTaskLink(ctx context.Context, link *StudioBatchTaskLinkRecord) error {
	if r.failUpdate {
		return fmt.Errorf("forced link update failure")
	}
	return r.delegate.UpdateStudioBatchTaskLink(ctx, link)
}

func (r *failingStudioBatchTaskLinkRepository) ListStudioBatchTaskLinksByBatchID(ctx context.Context, batchID string) ([]StudioBatchTaskLinkRecord, error) {
	return r.delegate.ListStudioBatchTaskLinksByBatchID(ctx, batchID)
}

func (r *failingStudioBatchTaskLinkRepository) ClaimStudioBatchTaskCandidate(ctx context.Context, candidateKey string, fromStatus string, toStatus string, updatedAt time.Time) (*StudioBatchTaskLinkRecord, bool, error) {
	return r.delegate.ClaimStudioBatchTaskCandidate(ctx, candidateKey, fromStatus, toStatus, updatedAt)
}

func (r *failingStudioBatchTaskLinkRepository) ClaimStudioBatchTaskCandidateUpdatedAt(ctx context.Context, candidateKey string, fromStatus string, observedUpdatedAt time.Time, toStatus string, updatedAt time.Time) (*StudioBatchTaskLinkRecord, bool, error) {
	return r.delegate.ClaimStudioBatchTaskCandidateUpdatedAt(ctx, candidateKey, fromStatus, observedUpdatedAt, toStatus, updatedAt)
}

type synchronizedStaleCreatingLinkRepository struct {
	delegate     StudioBatchTaskLinkRepository
	candidateKey string
	mu           sync.Mutex
	claimWaiters int
	claimRelease chan struct{}
}

func newSynchronizedStaleCreatingLinkRepository(delegate StudioBatchTaskLinkRepository) *synchronizedStaleCreatingLinkRepository {
	return &synchronizedStaleCreatingLinkRepository{
		delegate:     delegate,
		claimRelease: make(chan struct{}),
	}
}

func (r *synchronizedStaleCreatingLinkRepository) GetStudioBatchTaskLinkByCandidateKey(ctx context.Context, candidateKey string) (*StudioBatchTaskLinkRecord, error) {
	return r.delegate.GetStudioBatchTaskLinkByCandidateKey(ctx, candidateKey)
}

func (r *synchronizedStaleCreatingLinkRepository) CreateStudioBatchTaskLink(ctx context.Context, link *StudioBatchTaskLinkRecord) error {
	return r.delegate.CreateStudioBatchTaskLink(ctx, link)
}

func (r *synchronizedStaleCreatingLinkRepository) UpdateStudioBatchTaskLink(ctx context.Context, link *StudioBatchTaskLinkRecord) error {
	return r.delegate.UpdateStudioBatchTaskLink(ctx, link)
}

func (r *synchronizedStaleCreatingLinkRepository) ListStudioBatchTaskLinksByBatchID(ctx context.Context, batchID string) ([]StudioBatchTaskLinkRecord, error) {
	return r.delegate.ListStudioBatchTaskLinksByBatchID(ctx, batchID)
}

func (r *synchronizedStaleCreatingLinkRepository) ClaimStudioBatchTaskCandidate(ctx context.Context, candidateKey string, fromStatus string, toStatus string, updatedAt time.Time) (*StudioBatchTaskLinkRecord, bool, error) {
	if candidateKey == r.candidateKey && fromStatus == studioBatchTaskLinkStatusCreating && toStatus == studioBatchTaskLinkStatusCreating {
		r.mu.Lock()
		r.claimWaiters++
		if r.claimWaiters == 2 {
			close(r.claimRelease)
		}
		release := r.claimRelease
		r.mu.Unlock()
		select {
		case <-release:
		case <-time.After(time.Second):
		}
	}
	return r.delegate.ClaimStudioBatchTaskCandidate(ctx, candidateKey, fromStatus, toStatus, updatedAt)
}

func (r *synchronizedStaleCreatingLinkRepository) ClaimStudioBatchTaskCandidateUpdatedAt(ctx context.Context, candidateKey string, fromStatus string, observedUpdatedAt time.Time, toStatus string, updatedAt time.Time) (*StudioBatchTaskLinkRecord, bool, error) {
	if candidateKey == r.candidateKey && fromStatus == studioBatchTaskLinkStatusCreating && toStatus == studioBatchTaskLinkStatusCreating {
		r.mu.Lock()
		r.claimWaiters++
		if r.claimWaiters == 2 {
			close(r.claimRelease)
		}
		release := r.claimRelease
		r.mu.Unlock()
		select {
		case <-release:
		case <-time.After(time.Second):
		}
	}
	return r.delegate.ClaimStudioBatchTaskCandidateUpdatedAt(ctx, candidateKey, fromStatus, observedUpdatedAt, toStatus, updatedAt)
}

type studioBatchTaskSubmitterStub struct {
	submitCount int
	failAfter   int
}

func (s *studioBatchTaskSubmitterStub) Submit(string) error {
	s.submitCount++
	if s.failAfter > 0 && s.submitCount > s.failAfter {
		return fmt.Errorf("工作队列已满")
	}
	return nil
}

type studioBatchMutableSubmitter struct {
	err error
}

func (s *studioBatchMutableSubmitter) Submit(string) error {
	return s.err
}

type studioBatchGeneratorStub struct {
	runPending func(context.Context, string) error
	recover    func(context.Context, string) error
}

func (s studioBatchGeneratorStub) RunPendingStudioBatchItems(ctx context.Context, batchID string) error {
	if s.runPending == nil {
		return nil
	}
	return s.runPending(ctx, batchID)
}

func (s studioBatchGeneratorStub) RecoverStudioBatchMaterialization(ctx context.Context, batchID string) error {
	if s.recover == nil {
		return nil
	}
	return s.recover(ctx, batchID)
}

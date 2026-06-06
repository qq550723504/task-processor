package listingkit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

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

	svc := &service{studioBatchRepo: repo}
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

	svc := &service{
		studioBatchRepo: repo,
		studioSessionRepo: &studioBatchGenerationSessionRepoStub{
			session: &SheinStudioSession{
				ID:           "batch-1",
				SavedAsBatch: true,
				UpdatedAt:    draftUpdatedAt,
			},
		},
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

	svc := &service{studioBatchRepo: repo}
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

	svc := &service{studioBatchRepo: repo}
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

	svc := &service{studioBatchRepo: NewMemStudioBatchRepository()}
	collaborator := svc.taskStudioBatchOrDefault()
	if collaborator == nil {
		t.Fatal("taskStudioBatchOrDefault() = nil, want collaborator")
	}
	if svc.taskStudioBatch != collaborator {
		t.Fatal("expected collaborator to be cached on service field")
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

func TestTaskStudioBatchServiceContinueGenerationAutoRetriesPreviouslyFailedRetryableItem(t *testing.T) {
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

	detail, err := svc.continueStudioBatchGeneration(ctx, "batch-1")
	if err != nil {
		t.Fatalf("continueStudioBatchGeneration() error = %v", err)
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

	svc := &service{
		studioBatchRepo:   repo,
		studioSessionRepo: sessionRepo,
	}

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

	svc := &service{
		studioBatchRepo:   repo,
		studioSessionRepo: sessionRepo,
	}

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
	if detail.Items[0].Item.TargetGroupKey != "size:1200x1200" {
		t.Fatalf("detail.Items[0].Item.TargetGroupKey = %q, want size:1200x1200", detail.Items[0].Item.TargetGroupKey)
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

	if err := repo.CreateStudioBatchGraph(ctx, newStudioBatchRecordForTest("batch-1", now), newStudioBatchItemsForTest("batch-1", now), newStudioBatchAttemptsForTest("item-1", now), []StudioMaterializedDesignRecord{
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

	svc := &service{studioBatchRepo: repo}
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

	if err := repo.CreateStudioBatchGraph(ctx, newStudioBatchRecordForTest("batch-1", now), newStudioBatchItemsForTest("batch-1", now), newStudioBatchAttemptsForTest("item-1", now), []StudioMaterializedDesignRecord{
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

	svc := &service{studioBatchRepo: repo}
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

	if err := repo.CreateStudioBatchGraph(ctx, newStudioBatchRecordForTest("batch-1", now), newStudioBatchItemsForTest("batch-1", now), newStudioBatchAttemptsForTest("item-1", now), []StudioMaterializedDesignRecord{
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

	svc := &service{studioBatchRepo: repo}
	svc.repo = newStudioBatchTaskRepositoryStub()
	svc.taskSubmitter = &studioBatchTaskSubmitterStub{}
	svc.studioSessionRepo = &studioBatchTaskCreationSessionRepoStub{
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

func TestServiceCreateStudioBatchTasksCreatesRealListingKitTasks(t *testing.T) {
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

	svc := &service{
		repo:            taskRepo,
		taskSubmitter:   &studioBatchTaskSubmitterStub{},
		studioBatchRepo: batchRepo,
		studioSessionRepo: &studioBatchTaskCreationSessionRepoStub{
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
				ProductID:        1002,
				ParentProductID:  2002,
				VariantID:        4004,
				PrototypeGroupID: 5005,
				LayerID:          "layer-group",
				ProductName:      "Grouped Wallet",
				VariantLabel:     "White",
				PrintableWidth:   1200,
				PrintableHeight:  1200,
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
		repo:            taskRepo,
		taskSubmitter:   &studioBatchTaskSubmitterStub{},
		studioBatchRepo: batchRepo,
		studioSessionRepo: &studioBatchTaskCreationSessionRepoStub{
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
		repo:            taskRepo,
		taskSubmitter:   &studioBatchTaskSubmitterStub{failAfter: 1},
		studioBatchRepo: batchRepo,
		studioSessionRepo: &studioBatchTaskCreationSessionRepoStub{
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
		repo:              taskRepo,
		taskSubmitter:     &studioBatchTaskSubmitterStub{},
		studioBatchRepo:   batchRepo,
		studioSessionRepo: sessionRepo,
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
	if len(second.CreatedTasks) != 1 {
		t.Fatalf("second created tasks = %+v, want 1 reused task", second.CreatedTasks)
	}
	if second.CreatedTasks[0].ID != first.CreatedTasks[0].ID {
		t.Fatalf("second created task id = %q, want reused %q", second.CreatedTasks[0].ID, first.CreatedTasks[0].ID)
	}
	if got := len(taskRepo.tasks); got != 1 {
		t.Fatalf("persisted task count = %d, want 1 without duplicates", got)
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
		repo:            newStudioBatchTaskRepositoryStub(),
		taskSubmitter:   &studioBatchTaskSubmitterStub{},
		studioBatchRepo: repo,
		studioSessionRepo: &studioBatchTaskCreationSessionRepoStub{
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
		},
	}
	if _, err := svc.CreateStudioBatchTasks(ctx, "batch-1", &CreateStudioBatchTasksRequest{
		DesignIDs: []string{"design-1"},
	}); err == nil {
		t.Fatal("CreateStudioBatchTasks() error = nil, want approved-design validation failure")
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

type studioBatchTaskRepositoryStub struct {
	tasks map[string]*Task
}

func newStudioBatchTaskRepositoryStub() *studioBatchTaskRepositoryStub {
	return &studioBatchTaskRepositoryStub{tasks: make(map[string]*Task)}
}

func (r *studioBatchTaskRepositoryStub) CreateTask(_ context.Context, task *Task) error {
	if task == nil {
		return nil
	}
	cloned := *task
	r.tasks[task.ID] = &cloned
	return nil
}

func (r *studioBatchTaskRepositoryStub) GetTask(_ context.Context, taskID string) (*Task, error) {
	task, ok := r.tasks[taskID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	cloned := *task
	return &cloned, nil
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
func (r *studioBatchTaskRepositoryStub) PrepareRetry(context.Context, string) error       { return nil }
func (r *studioBatchTaskRepositoryStub) IncrementRetryCount(context.Context, string) error {
	return nil
}
func (r *studioBatchTaskRepositoryStub) SaveTaskResult(context.Context, string, *ListingKitResult) error {
	return nil
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

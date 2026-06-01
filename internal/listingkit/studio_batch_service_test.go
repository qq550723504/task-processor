package listingkit

import (
	"context"
	"encoding/json"
	"errors"
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

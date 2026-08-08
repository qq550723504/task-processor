package listingkit

import (
	"context"
	"testing"
	"time"
)

func TestRetryStudioBatchDesignBackgroundRemovalSnapshotsCurrentImageForManualRequest(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()
	if err := repo.CreateStudioBatchGraph(ctx, &StudioBatchRecord{
		ID:                        "batch-1",
		Status:                    StudioBatchStatusReviewReady,
		TransparentBackground:     true,
		TransparentBackgroundMode: StudioTransparencyModeNone,
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}, []StudioBatchItemRecord{{
		ID:        "item-1",
		BatchID:   "batch-1",
		Status:    StudioBatchItemStatusReviewReady,
		CreatedAt: now,
		UpdatedAt: now,
	}}, nil, []StudioMaterializedDesignRecord{{
		ID:                        "design-1",
		BatchID:                   "batch-1",
		ItemID:                    "item-1",
		ImageURL:                  "https://cdn.example.test/ordinary.png",
		TransparentBackgroundMode: StudioTransparencyModeNone,
		BackgroundRemovalStatus:   StudioBackgroundRemovalStatusNotRequested,
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	var sourceURL string
	svc := newTaskStudioBatchService(taskStudioBatchServiceConfig{
		repo: repo,
		retryBackgroundRemoval: func(_ context.Context, gotSourceURL string, _ string) (*studioBackgroundRemovalMaterialization, error) {
			sourceURL = gotSourceURL
			return &studioBackgroundRemovalMaterialization{ImageURL: "https://cdn.example.test/removed.png", Model: "rmbg-test"}, nil
		},
	})

	detail, err := svc.RetryStudioBatchDesignBackgroundRemoval(
		ctx,
		"batch-1",
		&RetryStudioBatchDesignBackgroundRemovalRequest{DesignIDs: []string{"design-1"}},
	)
	if err != nil {
		t.Fatalf("RetryStudioBatchDesignBackgroundRemoval() error = %v", err)
	}
	if sourceURL != "https://cdn.example.test/ordinary.png" {
		t.Fatalf("retry source URL = %q, want current image URL", sourceURL)
	}
	design := detail.Items[0].Designs[0]
	if design.OriginalImageURL != "https://cdn.example.test/ordinary.png" ||
		design.ImageURL != "https://cdn.example.test/removed.png" ||
		design.TransparentBackgroundMode != StudioTransparencyModeRemoval ||
		design.BackgroundRemovalStatus != StudioBackgroundRemovalStatusSucceeded {
		t.Fatalf("retried design = %+v, want original snapshot, removed image, removal mode, and succeeded status", design)
	}
}

func TestRetryStudioBatchDesignBackgroundRemovalUsesPersistedOriginalOnly(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()
	if err := repo.CreateStudioBatchGraph(ctx, &StudioBatchRecord{
		ID:                        "batch-1",
		Status:                    StudioBatchStatusReviewReady,
		TransparentBackground:     true,
		TransparentBackgroundMode: StudioTransparencyModeRemoval,
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}, []StudioBatchItemRecord{{
		ID:        "item-1",
		BatchID:   "batch-1",
		Status:    StudioBatchItemStatusReviewReady,
		CreatedAt: now,
		UpdatedAt: now,
	}}, nil, []StudioMaterializedDesignRecord{{
		ID:                        "design-1",
		BatchID:                   "batch-1",
		ItemID:                    "item-1",
		ImageURL:                  "https://cdn.example.test/prior-removed.png",
		OriginalImageURL:          "https://cdn.example.test/original.png",
		TransparentBackgroundMode: StudioTransparencyModeRemoval,
		BackgroundRemovalStatus:   StudioBackgroundRemovalStatusFailed,
		BackgroundRemovalError:    "previous provider timeout",
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	var sourceURL string
	svc := newTaskStudioBatchService(taskStudioBatchServiceConfig{
		repo: repo,
		retryBackgroundRemoval: func(_ context.Context, gotSourceURL string, _ string) (*studioBackgroundRemovalMaterialization, error) {
			sourceURL = gotSourceURL
			return &studioBackgroundRemovalMaterialization{ImageURL: "https://cdn.example.test/removed.png", Model: "rmbg-test"}, nil
		},
	})
	detail, err := svc.RetryStudioBatchDesignBackgroundRemoval(ctx, "batch-1", &RetryStudioBatchDesignBackgroundRemovalRequest{DesignIDs: []string{"design-1"}})
	if err != nil {
		t.Fatalf("RetryStudioBatchDesignBackgroundRemoval() error = %v", err)
	}
	if sourceURL != "https://cdn.example.test/original.png" {
		t.Fatalf("retry source URL = %q, want persisted original URL", sourceURL)
	}
	design := detail.Items[0].Designs[0]
	if design.ImageURL != "https://cdn.example.test/removed.png" || design.OriginalImageURL != sourceURL || design.BackgroundRemovalStatus != StudioBackgroundRemovalStatusSucceeded || design.BackgroundRemovalModel != "rmbg-test" || design.BackgroundRemovalError != "" {
		t.Fatalf("retried design = %+v, want updated final and success metadata", design)
	}
}

func TestRetryStudioBatchDesignBackgroundRemovalFailureFallsBackToOriginal(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()
	if err := repo.CreateStudioBatchGraph(ctx, &StudioBatchRecord{ID: "batch-1", Status: StudioBatchStatusReviewReady, CreatedAt: now, UpdatedAt: now}, []StudioBatchItemRecord{{ID: "item-1", BatchID: "batch-1", Status: StudioBatchItemStatusReviewReady, CreatedAt: now, UpdatedAt: now}}, nil, []StudioMaterializedDesignRecord{{
		ID:                        "design-1",
		BatchID:                   "batch-1",
		ItemID:                    "item-1",
		ImageURL:                  "https://cdn.example.test/original.png",
		OriginalImageURL:          "https://cdn.example.test/original.png",
		TransparentBackgroundMode: StudioTransparencyModeRemoval,
		BackgroundRemovalStatus:   StudioBackgroundRemovalStatusFailed,
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	svc := newTaskStudioBatchService(taskStudioBatchServiceConfig{
		repo: repo,
		retryBackgroundRemoval: func(context.Context, string, string) (*studioBackgroundRemovalMaterialization, error) {
			return nil, errStudioTestRemoval
		},
	})
	detail, err := svc.RetryStudioBatchDesignBackgroundRemoval(ctx, "batch-1", nil)
	if err != nil {
		t.Fatalf("RetryStudioBatchDesignBackgroundRemoval() error = %v", err)
	}
	design := detail.Items[0].Designs[0]
	if design.ImageURL != design.OriginalImageURL || design.BackgroundRemovalStatus != StudioBackgroundRemovalStatusFailed || design.BackgroundRemovalError != "removal failed" {
		t.Fatalf("failed retry design = %+v, want original fallback and error", design)
	}
}

func TestRetryStudioBatchDesignBackgroundRemovalValidatesAllRequestedDesignsBeforeMutation(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()
	if err := repo.CreateStudioBatchGraph(ctx, &StudioBatchRecord{
		ID:                        "batch-1",
		Status:                    StudioBatchStatusReviewReady,
		TransparentBackground:     true,
		TransparentBackgroundMode: StudioTransparencyModeRemoval,
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}, []StudioBatchItemRecord{{
		ID:        "item-1",
		BatchID:   "batch-1",
		Status:    StudioBatchItemStatusReviewReady,
		CreatedAt: now,
		UpdatedAt: now,
	}}, nil, []StudioMaterializedDesignRecord{{
		ID:                        "design-1",
		BatchID:                   "batch-1",
		ItemID:                    "item-1",
		ImageURL:                  "https://cdn.example.test/removed.png",
		OriginalImageURL:          "https://cdn.example.test/original.png",
		TransparentBackgroundMode: StudioTransparencyModeRemoval,
		BackgroundRemovalStatus:   StudioBackgroundRemovalStatusFailed,
		BackgroundRemovalError:    "previous provider timeout",
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	retryCalls := 0
	svc := newTaskStudioBatchService(taskStudioBatchServiceConfig{
		repo: repo,
		retryBackgroundRemoval: func(context.Context, string, string) (*studioBackgroundRemovalMaterialization, error) {
			retryCalls++
			return nil, errStudioTestRemoval
		},
	})
	_, err := svc.RetryStudioBatchDesignBackgroundRemoval(ctx, "batch-1", &RetryStudioBatchDesignBackgroundRemovalRequest{
		DesignIDs: []string{"design-1", "design-missing"},
	})
	if err == nil {
		t.Fatal("RetryStudioBatchDesignBackgroundRemoval() error = nil, want validation error")
	}
	if retryCalls != 0 {
		t.Fatalf("retry calls = %d, want no calls when request validation fails", retryCalls)
	}

	detail, err := svc.GetStudioBatchDetail(ctx, "batch-1")
	if err != nil {
		t.Fatalf("GetStudioBatchDetail() error = %v", err)
	}
	design := detail.Items[0].Designs[0]
	if design.ImageURL != "https://cdn.example.test/removed.png" || design.BackgroundRemovalStatus != StudioBackgroundRemovalStatusFailed || design.BackgroundRemovalError != "previous provider timeout" {
		t.Fatalf("design after rejected retry = %+v, want unchanged persisted state", design)
	}
}

func TestRetryStudioBatchDesignBackgroundRemovalClearsStaleModelOnFailure(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()
	if err := repo.CreateStudioBatchGraph(ctx, &StudioBatchRecord{ID: "batch-1", Status: StudioBatchStatusReviewReady, CreatedAt: now, UpdatedAt: now}, []StudioBatchItemRecord{{ID: "item-1", BatchID: "batch-1", Status: StudioBatchItemStatusReviewReady, CreatedAt: now, UpdatedAt: now}}, nil, []StudioMaterializedDesignRecord{{
		ID:                        "design-1",
		BatchID:                   "batch-1",
		ItemID:                    "item-1",
		ImageURL:                  "https://cdn.example.test/original.png",
		OriginalImageURL:          "https://cdn.example.test/original.png",
		TransparentBackgroundMode: StudioTransparencyModeRemoval,
		BackgroundRemovalStatus:   StudioBackgroundRemovalStatusFailed,
		BackgroundRemovalModel:    "stale-model",
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	svc := newTaskStudioBatchService(taskStudioBatchServiceConfig{
		repo: repo,
		retryBackgroundRemoval: func(context.Context, string, string) (*studioBackgroundRemovalMaterialization, error) {
			return nil, errStudioTestRemoval
		},
	})
	detail, err := svc.RetryStudioBatchDesignBackgroundRemoval(ctx, "batch-1", nil)
	if err != nil {
		t.Fatalf("RetryStudioBatchDesignBackgroundRemoval() error = %v", err)
	}
	if model := detail.Items[0].Designs[0].BackgroundRemovalModel; model != "" {
		t.Fatalf("failed retry model = %q, want stale model cleared", model)
	}
}

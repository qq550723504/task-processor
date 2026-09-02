package listingkit

import (
	"context"
	"testing"
	"time"

	"task-processor/internal/asset"
	"task-processor/internal/listingkit/core"
	"task-processor/internal/product/catalog"
	sheinpub "task-processor/internal/publishing/shein"
)

func TestProcessStandardProductLayerStartsPlatformAdaptTemporalWhenEnabled(t *testing.T) {
	t.Parallel()

	repo := NewInMemoryRepositoryForTest()
	platformClient := &stubPlatformAdaptWorkflowClient{}
	productSvc := &stubWorkflowProductSnapshotReader{
		task: &stubProductSnapshotTask{
			ID: "product-task-standard-layer-1",
			Request: &stubProductSnapshotRequest{
				Text: "standard layer request",
			},
		},
		product: &stubProductSnapshotFixture{
			Title: "Standard Layer Product",
		},
	}
	svc := seedWorkflowServices(seedWorkflowAssets(seedSupportDeps(&service{
		repo: repo,
		taskDeps: taskDependencies{
			platformAdaptWorkflowClient:  platformClient,
			platformAdaptWorkflowEnabled: true,
		},
	}, supportDependencySeed{
		assembler: NewAssemblerWithConfig(AssemblerConfig{AmazonBuilder: stubAmazonDraftBuilder{}}),
	}), nil, newDefaultAssetRecipeResolver(), newDefaultAssetBundleBuilder(), newDefaultAssetGenerationService()), productSvc, nil)

	task := &Task{
		ID:        "standard-layer-task-1",
		TenantID:  "tenant-1",
		Status:    core.TaskStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request: &GenerateRequest{ProductKey: "test-product",
			Text:      "standard layer request",
			Platforms: []string{"amazon", "shein"},
			TenantID:  "tenant-1",
			UserID:    "user-1",
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	snapshot, err := svc.ProcessStandardProductLayer(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("ProcessStandardProductLayer() error = %v", err)
	}
	if snapshot == nil || snapshot.CatalogProduct == nil {
		t.Fatalf("snapshot = %+v, want catalog product snapshot", snapshot)
	}
	if len(platformClient.calls) != 1 {
		t.Fatalf("platform temporal calls = %+v, want 1 call", platformClient.calls)
	}
	if platformClient.calls[0].TaskID != task.ID || platformClient.calls[0].Platform != "all" {
		t.Fatalf("platform temporal call = %+v, want task %s with all platforms", platformClient.calls[0], task.ID)
	}
}

func TestProcessStandardProductLayerPreservesExistingPlatformCache(t *testing.T) {
	t.Parallel()

	repo := NewInMemoryRepositoryForTest()
	productSvc := &stubWorkflowProductSnapshotReader{
		task: &stubProductSnapshotTask{
			ID: "product-task-standard-layer-cache",
			Request: &stubProductSnapshotRequest{
				Text: "standard layer cache request",
			},
		},
		product: &stubProductSnapshotFixture{
			Title: "Updated Standard Product",
		},
	}
	svc := seedWorkflowServices(seedWorkflowAssets(seedSupportDeps(&service{
		repo: repo,
	}, supportDependencySeed{
		assembler: NewAssemblerWithConfig(AssemblerConfig{AmazonBuilder: stubAmazonDraftBuilder{}}),
	}), nil, newDefaultAssetRecipeResolver(), newDefaultAssetBundleBuilder(), newDefaultAssetGenerationService()), productSvc, nil)

	task := &Task{
		ID:        "standard-layer-cache-task",
		TenantID:  "tenant-1",
		Status:    core.TaskStatusNeedsReview,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request: &GenerateRequest{ProductKey: "test-product",
			Text:      "standard layer cache request",
			Platforms: []string{"shein"},
			TenantID:  "tenant-1",
			UserID:    "user-1",
		},
		Result: &ListingKitResult{
			TaskID:          "standard-layer-cache-task",
			Status:          string(core.TaskStatusNeedsReview),
			Shein:           &sheinpub.Package{SpuName: "cached shein package"},
			SDSDesignResult: &SDSSyncSummary{Status: "completed", MockupImageURLs: []string{"https://cdn.sdspod.com/out/cached.jpg"}},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	snapshot, err := svc.ProcessStandardProductLayer(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("ProcessStandardProductLayer() error = %v", err)
	}
	if snapshot == nil || snapshot.CatalogProduct == nil {
		t.Fatalf("snapshot = %+v, want updated standard product snapshot", snapshot)
	}
	updated, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if updated.Result == nil || updated.Result.Shein == nil || updated.Result.Shein.SpuName != "cached shein package" {
		t.Fatalf("persisted shein package = %+v, want cached package preserved while platform layer reruns", updated.Result)
	}
	if updated.Result.StandardProductSnapshot == nil || updated.Result.StandardProductSnapshot.CatalogProduct == nil {
		t.Fatalf("persisted standard snapshot = %+v, want updated standard layer data", updated.Result.StandardProductSnapshot)
	}
}

func TestStandardSnapshotFromTaskRejectsResultWithoutPersistedSnapshot(t *testing.T) {
	t.Parallel()

	task := &Task{TenantID: "tenant-test", ID: "legacy-task-1",
		Result: &ListingKitResult{
			CatalogProduct:        &catalogProductForTest,
			AssetInventorySummary: &assetInventorySummaryForTest,
			SDSSync:               &SDSSyncSummary{Status: "completed"},
			WorkflowStages:        []WorkflowStage{{Kind: "sds_design_sync", Status: "completed"}},
		},
	}

	if _, err := standardSnapshotFromTask(task); err == nil {
		t.Fatal("standardSnapshotFromTask() error = nil, want persisted snapshot requirement")
	}
}

var catalogProductForTest = catalogProductStub()
var assetInventorySummaryForTest = asset.InventorySummary{TotalRecords: 1, SelectedCount: 1}

func catalogProductStub() catalog.ProductSnapshot {
	return catalog.ProductSnapshot{
		Title: "Legacy catalog product",
	}
}

package listingkit

import (
	"context"
	"testing"
	"time"

	"task-processor/internal/asset"
	"task-processor/internal/catalog"
	"task-processor/internal/productenrich"
)

func TestProcessStandardProductLayerStartsPlatformAdaptTemporalWhenEnabled(t *testing.T) {
	t.Parallel()

	repo := NewInMemoryRepositoryForTest()
	platformClient := &stubPlatformAdaptWorkflowClient{}
	productSvc := &stubWorkflowProductService{
		task: &productenrich.Task{
			ID: "product-task-standard-layer-1",
			Request: &productenrich.GenerateRequest{
				Text: "standard layer request",
			},
		},
		product: &productenrich.ProductJSON{
			Title: "Standard Layer Product",
		},
	}
	svc := seedWorkflowServices(seedWorkflowAssets(seedSupportDeps(&service{
		repo: repo,
		runtime: serviceRuntimeState{
			platformAdaptWorkflowClient:  platformClient,
			platformAdaptWorkflowEnabled: true,
		},
	}, supportDependencySeed{
		assembler: NewAssemblerWithConfig(AssemblerConfig{AmazonBuilder: stubAmazonDraftBuilder{}}),
	}), nil, newDefaultAssetRecipeResolver(), newDefaultAssetBundleBuilder(), newDefaultAssetGenerationService()), productSvc, nil)

	task := &Task{
		ID:        "standard-layer-task-1",
		TenantID:  "tenant-1",
		Status:    TaskStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request: &GenerateRequest{
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
	if snapshot == nil || snapshot.CanonicalProduct == nil {
		t.Fatalf("snapshot = %+v, want canonical product snapshot", snapshot)
	}
	if len(platformClient.calls) != 1 {
		t.Fatalf("platform temporal calls = %+v, want 1 call", platformClient.calls)
	}
	if platformClient.calls[0].TaskID != task.ID || platformClient.calls[0].Platform != "all" {
		t.Fatalf("platform temporal call = %+v, want task %s with all platforms", platformClient.calls[0], task.ID)
	}
}

func TestStandardSnapshotFromTaskBuildsFallbackFromLegacyResult(t *testing.T) {
	t.Parallel()

	task := &Task{
		ID: "legacy-task-1",
		Result: &ListingKitResult{
			CatalogProduct:        &catalogProductForTest,
			AssetInventorySummary: &assetInventorySummaryForTest,
			SDSSync:               &SDSSyncSummary{Status: "completed"},
			WorkflowStages:        []WorkflowStage{{Kind: "sds_design_sync", Status: "completed"}},
		},
	}

	snapshot, err := standardSnapshotFromTask(task)
	if err != nil {
		t.Fatalf("standardSnapshotFromTask() error = %v", err)
	}
	if snapshot == nil {
		t.Fatalf("snapshot = nil, want fallback snapshot")
	}
	if snapshot.CatalogProduct == nil || snapshot.AssetInventorySummary == nil || snapshot.SDSSync == nil {
		t.Fatalf("snapshot = %+v, want legacy result fields copied into snapshot", snapshot)
	}
	if len(snapshot.WorkflowStages) != 1 || snapshot.WorkflowStages[0].Kind != "sds_design_sync" {
		t.Fatalf("snapshot workflow stages = %+v, want preserved legacy stages", snapshot.WorkflowStages)
	}
}

var catalogProductForTest = catalogProductStub()
var assetInventorySummaryForTest = asset.InventorySummary{TotalRecords: 1, SelectedCount: 1}

func catalogProductStub() catalog.Product {
	return catalog.Product{
		Title: "Legacy catalog product",
	}
}

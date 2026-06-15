package listingkit

import (
	"context"
	"errors"
	"testing"

	"task-processor/internal/catalog"
	"task-processor/internal/catalog/canonical"
	"task-processor/internal/productimage"
	sdsadapter "task-processor/internal/sds/adapter"
	sdsdesign "task-processor/internal/sds/design"
	sdsworkflow "task-processor/internal/sds/workflow"
)

func TestRetryTaskChildTaskRejectsEmptyKind(t *testing.T) {
	t.Parallel()

	svc := &service{repo: NewInMemoryRepositoryForTest()}
	if _, err := svc.RetryTaskChildTask(context.Background(), "task-1", &RetryChildTaskRequest{}); !errors.Is(err, ErrChildTaskRetryInvalidRequest) {
		t.Fatalf("RetryTaskChildTask() error = %v, want ErrChildTaskRetryInvalidRequest", err)
	}
}

func TestRetryTaskChildTaskRejectsProcessingTask(t *testing.T) {
	t.Parallel()

	repo := NewInMemoryRepositoryForTest()
	task := &Task{
		ID:     "task-1",
		Status: TaskStatusProcessing,
		Result: &ListingKitResult{
			ChildTasks: []ChildTaskState{{Kind: "sds_design_sync", Status: string(TaskStatusFailed)}},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	svc := &service{repo: repo}
	if _, err := svc.RetryTaskChildTask(context.Background(), task.ID, &RetryChildTaskRequest{Kind: "sds_design_sync"}); !errors.Is(err, ErrChildTaskRetryConflict) {
		t.Fatalf("RetryTaskChildTask() error = %v, want ErrChildTaskRetryConflict", err)
	}
}

func TestRetryTaskChildTaskRetriesSDSDesignSync(t *testing.T) {
	t.Parallel()

	repo := NewInMemoryRepositoryForTest()
	task := &Task{
		ID:     "task-sds-design",
		Status: TaskStatusCompleted,
		Request: &GenerateRequest{
			ImageURLs: []string{"https://example.com/source.jpg"},
			Text:      "floor mat",
			Platforms: []string{"shein"},
			Country:   "US",
			Language:  "en_US",
			Options: &GenerateOptions{
				ProcessImages: false,
				SDS: &SDSSyncOptions{
					VariantID:        10947,
					ParentProductID:  10946,
					PrototypeGroupID: 10019364,
					DesignType:       "material",
					LayerID:          "10059417",
					FitLevel:         1,
					ResizeMode:       1,
					BlankDesignURL:   "https://example.com/blank.jpg",
				},
			},
		},
		Result: &ListingKitResult{
			TaskID:    "task-sds-design",
			Status:    string(TaskStatusCompleted),
			Platforms: []string{"shein"},
			CatalogProduct: &catalog.Product{
				Title: "Floor Mat",
			},
			CanonicalProduct: &canonical.Product{
				Title:  "Floor Mat",
				Images: []canonical.Image{{URL: "https://example.com/source.jpg"}},
			},
			ImageAssets: &productimage.ImageProcessResult{
				MainImage: &productimage.ImageAsset{URL: "https://example.com/main.jpg"},
			},
			SDSSync: &SDSSyncSummary{
				VariantID: 10947,
				Status:    "failed",
				Error:     "old failure",
			},
			ChildTasks:     []ChildTaskState{{Kind: "sds_design_sync", Status: string(TaskStatusFailed), Error: "old failure"}},
			WorkflowStages: []WorkflowStage{{Kind: "sds_design_sync", Status: WorkflowStageStatusFailed}},
			WorkflowIssues: []WorkflowIssue{{Stage: "sds_design_sync", Severity: WorkflowIssueSeverityWarning, Message: "old failure"}},
			Summary:        &GenerationSummary{Warnings: []string{"old failure", "keep me"}},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	sdsSvc := &stubWorkflowSDSSyncService{
		result: &sdsadapter.SyncResult{
			DesignSync: &sdsworkflow.SyncResult{
				DesignResult: &sdsdesign.PrepareSyncDesignResult{
					Page: &sdsdesign.DesignProductPage{
						Product: sdsdesign.DesignProduct{
							ID:            10947,
							ParentID:      10946,
							PrototypeID:   "10019364",
							PrototypeType: "FREE",
							Size:          "One size",
							SizeID:        1,
							ColorID:       1004,
							ColorName:     "white",
						},
					},
					Request: &sdsdesign.SyncDesignRequest{
						PrototypeGroupID: 10019364,
						Prototypes: []sdsdesign.SyncDesignPrototype{
							{
								Layers: []sdsdesign.SyncDesignLayer{
									{
										LayerID:    "10059417",
										Content:    "https://example.com/main.jpg",
										ImgWidth:   1200,
										ImgHeight:  1200,
										ResizeMode: 1,
										FitLevel:   1,
									},
								},
							},
						},
					},
					RenderedImageURLs: []string{"https://example.com/rendered.jpg"},
				},
			},
		},
	}

	svc := seedWorkflowAssets(seedWorkflowDepsFromMirrors(&service{
		repo: repo,
		mirrors: serviceDependencyMirrors{
			sdsSyncSvc:          sdsSvc,
			assembler:           NewAssemblerWithConfig(AssemblerConfig{AmazonBuilder: stubAmazonDraftBuilder{}}),
		},
	}), nil, newDefaultAssetRecipeResolver(), newDefaultAssetBundleBuilder(), newDefaultAssetGenerationService())

	result, err := svc.RetryTaskChildTask(context.Background(), task.ID, &RetryChildTaskRequest{Kind: "sds_design_sync"})
	if err != nil {
		t.Fatalf("RetryTaskChildTask() error = %v", err)
	}
	if result.Status != TaskStatusCompleted && result.Status != TaskStatusNeedsReview {
		t.Fatalf("task status = %q, want completed or needs_review", result.Status)
	}
	if result.Result == nil || result.Result.SDSSync == nil {
		t.Fatalf("task result = %+v, want sds sync payload", result.Result)
	}
	if result.Result.SDSSync.Status != "completed" {
		t.Fatalf("sds sync = %+v, want completed status", result.Result.SDSSync)
	}
	if len(result.Result.SDSSync.MockupImageURLs) != 1 || result.Result.SDSSync.MockupImageURLs[0] != "https://example.com/rendered.jpg" {
		t.Fatalf("sds sync = %+v, want refreshed mockup urls", result.Result.SDSSync)
	}
	if !hasChildTaskStatus(result.Result.ChildTasks, "sds_design_sync", string(TaskStatusCompleted)) {
		t.Fatalf("child tasks = %+v, want completed sds_design_sync", result.Result.ChildTasks)
	}
	var completedSDSStage bool
	for _, stage := range result.Result.WorkflowStages {
		if stage.Kind == "sds_design_sync" && stage.Status == WorkflowStageStatusCompleted {
			completedSDSStage = true
			break
		}
	}
	if !completedSDSStage {
		t.Fatalf("workflow stages = %+v, want appended completed sds_design_sync stage", result.Result.WorkflowStages)
	}
	for _, issue := range result.Result.WorkflowIssues {
		if issue.Stage == "sds_design_sync" {
			t.Fatalf("workflow issues = %+v, want previous SDS design issues cleared on successful retry", result.Result.WorkflowIssues)
		}
	}
	if result.Result.Summary != nil {
		for _, warning := range result.Result.Summary.Warnings {
			if warning == "old failure" {
				t.Fatalf("summary = %+v, want previous SDS warnings cleared", result.Result.Summary)
			}
		}
		foundUnrelated := false
		for _, warning := range result.Result.Summary.Warnings {
			if warning == "keep me" {
				foundUnrelated = true
				break
			}
		}
		if !foundUnrelated {
			t.Fatalf("summary = %+v, want unrelated warnings preserved", result.Result.Summary)
		}
	}
	if sdsSvc.lastInput.Sync.VariantID != 10947 {
		t.Fatalf("last sync input = %+v, want variant id 10947", sdsSvc.lastInput.Sync)
	}
}

func TestRetryTaskChildTaskRetriesSDSCatalogProduct(t *testing.T) {
	t.Parallel()

	repo := NewInMemoryRepositoryForTest()
	task := &Task{
		ID:     "task-sds-catalog",
		Status: TaskStatusNeedsReview,
		Request: &GenerateRequest{
			ImageURLs: []string{"https://example.com/source.jpg"},
			Text:      "fallback title",
			Platforms: []string{"shein"},
			Country:   "US",
			Language:  "en_US",
			Options: &GenerateOptions{
				ProcessImages: false,
				SDS: &SDSSyncOptions{
					VariantID:          5001,
					ProductName:        "Retry Catalog Product",
					ProductPerformance: "soft mat for bathroom",
				},
			},
		},
		Result: &ListingKitResult{
			TaskID:         "task-sds-catalog",
			Status:         string(TaskStatusNeedsReview),
			Platforms:      []string{"shein"},
			ChildTasks:     []ChildTaskState{{Kind: "sds_catalog_product", Status: string(TaskStatusFailed), Error: "catalog failed"}},
			WorkflowStages: []WorkflowStage{{Kind: "sds_catalog_product", Status: WorkflowStageStatusFailed}},
			WorkflowIssues: []WorkflowIssue{{Stage: "sds_catalog_product", Severity: WorkflowIssueSeverityBlocking, Message: "catalog failed"}},
			Summary:        &GenerationSummary{Warnings: []string{"catalog failed"}, NeedsReview: true},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	svc := seedWorkflowAssets(seedWorkflowDepsFromMirrors(&service{
		repo: repo,
		mirrors: serviceDependencyMirrors{
			assembler:           NewAssemblerWithConfig(AssemblerConfig{AmazonBuilder: stubAmazonDraftBuilder{}}),
		},
	}), nil, newDefaultAssetRecipeResolver(), newDefaultAssetBundleBuilder(), newDefaultAssetGenerationService())

	result, err := svc.RetryTaskChildTask(context.Background(), task.ID, &RetryChildTaskRequest{Kind: "sds_catalog_product"})
	if err != nil {
		t.Fatalf("RetryTaskChildTask() error = %v", err)
	}
	if result.Status != TaskStatusCompleted && result.Status != TaskStatusNeedsReview {
		t.Fatalf("task status = %q, want completed or needs_review", result.Status)
	}
	if result.Result == nil || result.Result.CanonicalProduct == nil {
		t.Fatalf("task result = %+v, want canonical product", result.Result)
	}
	if result.Result.CanonicalProduct.Title != "Retry Catalog Product" {
		t.Fatalf("canonical product = %+v, want rebuilt SDS catalog title", result.Result.CanonicalProduct)
	}
	if !hasChildTaskStatus(result.Result.ChildTasks, "sds_catalog_product", string(TaskStatusCompleted)) {
		t.Fatalf("child tasks = %+v, want completed sds_catalog_product", result.Result.ChildTasks)
	}
	var completedCatalogStage bool
	for _, stage := range result.Result.WorkflowStages {
		if stage.Kind == "sds_catalog_product" && stage.Status == WorkflowStageStatusCompleted {
			completedCatalogStage = true
			break
		}
	}
	if !completedCatalogStage {
		t.Fatalf("workflow stages = %+v, want appended completed sds_catalog_product stage", result.Result.WorkflowStages)
	}
	for _, issue := range result.Result.WorkflowIssues {
		if issue.Stage == "sds_catalog_product" {
			t.Fatalf("workflow issues = %+v, want previous SDS catalog issues cleared on successful retry", result.Result.WorkflowIssues)
		}
	}
}

func TestPruneChildTaskRetryArtifactsKeepsOtherStageWarnings(t *testing.T) {
	t.Parallel()

	result := &ListingKitResult{
		WorkflowIssues: []WorkflowIssue{
			{Stage: "sds_design_sync", Severity: WorkflowIssueSeverityWarning, Message: "design failed", Detail: "render timeout"},
			{Stage: "shein_review", Severity: WorkflowIssueSeverityReview, Message: "confirm category"},
		},
		Summary: &GenerationSummary{
			Warnings: []string{"design failed", "render timeout", "confirm category", "other warning"},
		},
	}

	pruneChildTaskRetryArtifacts(result, "sds_design_sync")

	if len(result.WorkflowIssues) != 1 || result.WorkflowIssues[0].Stage != "shein_review" {
		t.Fatalf("workflow issues = %+v, want non-retried stage issues preserved", result.WorkflowIssues)
	}
	if got := result.Summary.Warnings; len(got) != 2 || got[0] != "confirm category" || got[1] != "other warning" {
		t.Fatalf("summary warnings = %#v, want only retried-stage warnings removed", got)
	}
}

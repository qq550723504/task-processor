package amazonlisting

import (
	"context"
	"fmt"
	"testing"

	"task-processor/internal/productenrich"
)

type stubRecoveryWorkflow struct {
	failFirst bool
}

func (w *stubRecoveryWorkflow) Run(_ context.Context, task *Task) (*WorkflowArtifacts, error) {
	if w.failFirst {
		w.failFirst = false
		artifacts := &WorkflowArtifacts{
			Draft: &AmazonListingDraft{
				TaskID:        task.ID,
				Status:        string(TaskStatusProcessing),
				Marketplace:   task.Request.Marketplace,
				ProductTaskID: "product-task-1",
				ChildTasks: []ChildTaskState{
					{
						Kind:   "product_enrich",
						TaskID: "product-task-1",
						Status: string(TaskStatusFailed),
						Error:  "product enrichment failed",
					},
				},
			},
		}
		return nil, &WorkflowError{
			Artifacts: artifacts,
			Err:       fmt.Errorf("product enrichment failed: generated product JSON invalid"),
		}
	}

	productReq := &productenrich.GenerateRequest{Text: task.Request.Text}
	productJSON := &productenrich.ProductJSON{
		Title:       "Recovered Listing",
		Category:    []string{"Electronics", "Headphones"},
		Attributes:  map[string]string{"brand": "SoundPeak"},
		Description: "Recovered after retry.",
	}
	canonical := productenrich.BuildCanonicalProduct(productReq, productJSON)
	draft := &AmazonListingDraft{
		TaskID:             task.ID,
		Status:             string(TaskStatusCompleted),
		Marketplace:        task.Request.Marketplace,
		Title:              "Recovered Listing",
		Brand:              "SoundPeak",
		Description:        "Recovered after retry.",
		BulletPoints:       []string{"主动降噪", "长续航", "舒适佩戴"},
		CategoryPath:       []string{"Electronics", "Headphones"},
		ProductTaskID:      "product-task-1",
		ProductImageTaskID: "image-task-1",
		Images: &AmazonImageBundle{
			MainImage:     "https://cdn.example.com/main.jpg",
			WhiteBgImage:  "https://cdn.example.com/white.jpg",
			GalleryImages: []string{"https://cdn.example.com/gallery-1.jpg"},
		},
		Pricing: &AmazonPricingDraft{
			Currency:       "USD",
			SourceCost:     29.9,
			SuggestedPrice: 49.9,
			MinPrice:       39.9,
		},
		Export: &AmazonListingExport{
			ListingsAPI: &AmazonListingsAPIExport{
				SKU:           "RECOVERED-SKU-1",
				MarketplaceID: "ATVPDKIKX0DER",
				ProductType:   "Headphones",
				Requirements:  "LISTING",
				Attributes:    map[string]any{"item_name": []map[string]string{{"value": "Recovered Listing"}}},
			},
		},
		ChildTasks: []ChildTaskState{
			{Kind: "product_enrich", TaskID: "product-task-1", Status: "completed"},
			{Kind: "product_image", TaskID: "image-task-1", Status: "completed"},
		},
		Compliance: &AmazonComplianceReport{Ready: true},
		Review:     &AmazonReviewReport{NeedsReview: false},
	}

	return &WorkflowArtifacts{
		ProductTask:      &productenrich.Task{ID: "product-task-1", Request: productReq, Result: productJSON, Status: productenrich.TaskStatusCompleted},
		CanonicalProduct: canonical,
		Draft:            draft,
	}, nil
}

type spyTaskSubmitter struct {
	submitted []string
}

func (s *spyTaskSubmitter) Submit(taskID string) error {
	s.submitted = append(s.submitted, taskID)
	return nil
}

func TestProcessListing_SavesPartialDraftAndRetryCanRecover(t *testing.T) {
	repo := &stubRepository{}
	submitter := &spyTaskSubmitter{}
	workflow := &stubRecoveryWorkflow{failFirst: true}

	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: &stubProductService{},
		Assembler:      NewAssembler(),
		ExportBuilder:  NewExportBuilder(),
		Validator:      NewValidator(),
		AutoFixer:      NewAutoFixer(),
		Workflow:       workflow,
		TaskSubmitter:  submitter,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	task, err := svc.CreateGenerateTask(context.Background(), &GenerateRequest{
		Marketplace: "amazon",
		Text:        "高品质蓝牙耳机，支持主动降噪和长续航。",
		ImageURLs:   []string{"https://example.com/1.jpg", "https://example.com/2.jpg"},
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	if _, err := svc.ProcessListing(context.Background(), task); err == nil {
		t.Fatal("expected first ProcessListing call to fail")
	}

	failedResult, err := svc.GetTaskResult(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get failed task result: %v", err)
	}
	if failedResult.Status != TaskStatusFailed {
		t.Fatalf("failed result status = %s, want failed", failedResult.Status)
	}
	if failedResult.Result == nil {
		t.Fatal("expected partial draft to be saved")
	}
	if len(failedResult.Result.ChildTasks) != 1 {
		t.Fatalf("partial draft child tasks = %d, want 1", len(failedResult.Result.ChildTasks))
	}
	if failedResult.Result.ChildTasks[0].Kind != "product_enrich" || failedResult.Result.ChildTasks[0].Status != string(TaskStatusFailed) {
		t.Fatalf("unexpected child task state: %+v", failedResult.Result.ChildTasks[0])
	}

	workbench, err := svc.GetTaskWorkbench(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get workbench: %v", err)
	}
	if len(workbench.ChildTasks) != 1 {
		t.Fatalf("workbench child tasks = %d, want 1", len(workbench.ChildTasks))
	}
	if workbench.Ready {
		t.Fatal("failed workbench should not be ready")
	}

	if _, err := svc.ReviewTask(context.Background(), task.ID, &ReviewTaskRequest{Action: "retry"}); err != nil {
		t.Fatalf("retry task: %v", err)
	}
	if len(submitter.submitted) != 2 {
		t.Fatalf("submit count = %d, want 2", len(submitter.submitted))
	}

	retryTask, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get retry task: %v", err)
	}
	draft, err := svc.ProcessListing(context.Background(), retryTask)
	if err != nil {
		t.Fatalf("second ProcessListing call: %v", err)
	}
	if draft == nil || draft.Export == nil {
		t.Fatal("expected recovered draft with export")
	}
	if draft.ProductTaskID != "product-task-1" {
		t.Fatalf("recovered product task id = %q, want product-task-1", draft.ProductTaskID)
	}
	if len(draft.ChildTasks) != 2 {
		t.Fatalf("recovered child tasks = %d, want 2", len(draft.ChildTasks))
	}

	completedResult, err := svc.GetTaskResult(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get completed task result: %v", err)
	}
	if completedResult.Status == TaskStatusFailed {
		t.Fatalf("completed result status = %s, want non-failed", completedResult.Status)
	}
}

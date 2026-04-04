package amazonlisting

import (
	"context"
	"errors"
	"testing"

	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
)

type stubWorkflowProductService struct {
	productTask *productenrich.Task
	productJSON *productenrich.ProductJSON
	taskResult  *productenrich.TaskResult
	createErr   error
	processErr  error
}

func (s *stubWorkflowProductService) CreateGenerateTask(_ context.Context, req *productenrich.GenerateRequest) (*productenrich.Task, error) {
	if s.createErr != nil {
		return nil, s.createErr
	}
	if s.productTask != nil {
		return s.productTask, nil
	}
	return &productenrich.Task{ID: "product-task-1", Request: req}, nil
}

func (s *stubWorkflowProductService) GetTaskResult(_ context.Context, taskID string) (*productenrich.TaskResult, error) {
	if s.taskResult != nil && s.taskResult.TaskID == taskID {
		return s.taskResult, nil
	}
	return nil, errors.New("not found")
}

func (s *stubWorkflowProductService) ProcessProduct(_ context.Context, _ *productenrich.Task) (*productenrich.ProductJSON, error) {
	if s.processErr != nil {
		return nil, s.processErr
	}
	if s.productJSON != nil {
		return s.productJSON, nil
	}
	return &productenrich.ProductJSON{
		Title:       "Demo Product",
		Description: "Demo description for listing workflow.",
		Category:    []string{"Home", "Kitchen"},
	}, nil
}

type stubWorkflowImageService struct {
	imageTask   *productimage.Task
	imageResult *productimage.ImageProcessResult
	taskResult  *productimage.TaskResult
	createErr   error
	processErr  error
}

func (s *stubWorkflowImageService) CreateProcessTask(_ context.Context, req *productimage.ImageProcessRequest) (*productimage.Task, error) {
	if s.createErr != nil {
		return nil, s.createErr
	}
	if s.imageTask != nil {
		return s.imageTask, nil
	}
	return &productimage.Task{ID: "image-task-1", Request: req}, nil
}

func (s *stubWorkflowImageService) GetTaskResult(_ context.Context, taskID string) (*productimage.TaskResult, error) {
	if s.taskResult != nil && s.taskResult.TaskID == taskID {
		return s.taskResult, nil
	}
	return nil, errors.New("not found")
}

func (s *stubWorkflowImageService) ProcessImages(_ context.Context, _ *productimage.Task) (*productimage.ImageProcessResult, error) {
	if s.processErr != nil {
		return nil, s.processErr
	}
	if s.imageResult != nil {
		return s.imageResult, nil
	}
	return &productimage.ImageProcessResult{}, nil
}

func TestListingWorkflow_RunBuildsDraftAndTaskReferences(t *testing.T) {
	workflow := NewListingWorkflow(
		&stubWorkflowProductService{
			productTask: &productenrich.Task{ID: "product-task-123", Request: &productenrich.GenerateRequest{ProductURL: "https://detail.1688.com/offer/123.html"}},
			productJSON: &productenrich.ProductJSON{
				Title:       "Ceramic Mug",
				Description: "A ceramic mug for coffee and tea.",
				Category:    []string{"Home & Kitchen", "Drinkware"},
				Attributes:  map[string]string{"brand": "Acme"},
			},
		},
		&stubWorkflowImageService{
			imageTask:   &productimage.Task{ID: "image-task-456"},
			imageResult: &productimage.ImageProcessResult{MainImage: &productimage.ImageAsset{URL: "https://cdn.example.com/main.jpg"}},
		},
		NewAssembler(),
		NewAutoFixer(),
		NewExportBuilder(),
	)

	artifacts, err := workflow.Run(context.Background(), &Task{
		ID: "listing-task-1",
		Request: &GenerateRequest{
			Marketplace: "amazon",
			Country:     "US",
			ProductURL:  "https://detail.1688.com/offer/123.html",
			Options:     &GenerateOptions{ProcessImages: true},
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if artifacts == nil || artifacts.Draft == nil {
		t.Fatal("expected workflow draft")
	}
	if artifacts.Draft.ProductTaskID != "product-task-123" {
		t.Fatalf("ProductTaskID = %q, want product-task-123", artifacts.Draft.ProductTaskID)
	}
	if artifacts.Draft.ProductImageTaskID != "image-task-456" {
		t.Fatalf("ProductImageTaskID = %q, want image-task-456", artifacts.Draft.ProductImageTaskID)
	}
	if len(artifacts.Draft.ChildTasks) != 2 {
		t.Fatalf("len(ChildTasks) = %d, want 2", len(artifacts.Draft.ChildTasks))
	}
	if artifacts.CanonicalProduct == nil || artifacts.CanonicalProduct.Brand != "Acme" {
		t.Fatalf("expected canonical product brand Acme, got %+v", artifacts.CanonicalProduct)
	}
}

func TestListingWorkflow_RunReturnsImageServiceError(t *testing.T) {
	workflow := NewListingWorkflow(
		&stubWorkflowProductService{},
		&stubWorkflowImageService{processErr: errors.New("image boom")},
		NewAssembler(),
		NewAutoFixer(),
		NewExportBuilder(),
	)

	_, err := workflow.Run(context.Background(), &Task{
		ID: "listing-task-2",
		Request: &GenerateRequest{
			Marketplace: "amazon",
			Country:     "US",
			ProductURL:  "https://detail.1688.com/offer/123.html",
			Options:     &GenerateOptions{ProcessImages: true},
		},
	})
	if err == nil || err.Error() != "image processing failed: image boom" {
		t.Fatalf("unexpected error: %v", err)
	}
	var workflowErr *WorkflowError
	if !errors.As(err, &workflowErr) || workflowErr.Artifacts == nil || workflowErr.Artifacts.Draft == nil {
		t.Fatalf("expected workflow error with artifacts, got %v", err)
	}
	if len(workflowErr.Artifacts.Draft.ChildTasks) == 0 {
		t.Fatal("expected partial child task state to be captured")
	}
}

func TestListingWorkflow_RunReusesCompletedChildTasks(t *testing.T) {
	workflow := NewListingWorkflow(
		&stubWorkflowProductService{
			taskResult: &productenrich.TaskResult{
				TaskID: "product-task-reuse",
				Status: productenrich.TaskStatusCompleted,
				ProductJSON: &productenrich.ProductJSON{
					Title:       "Reuse Product",
					Description: "Reused product result.",
					Category:    []string{"Home", "Storage"},
				},
			},
		},
		&stubWorkflowImageService{
			taskResult: &productimage.TaskResult{
				TaskID: "image-task-reuse",
				Status: productimage.TaskStatusCompleted,
				Result: &productimage.ImageProcessResult{
					MainImage: &productimage.ImageAsset{URL: "https://cdn.example.com/reused-main.jpg"},
				},
			},
		},
		NewAssembler(),
		NewAutoFixer(),
		NewExportBuilder(),
	)

	artifacts, err := workflow.Run(context.Background(), &Task{
		ID: "listing-task-reuse",
		Request: &GenerateRequest{
			Marketplace: "amazon",
			Country:     "US",
			ProductURL:  "https://detail.1688.com/offer/123.html",
			Options:     &GenerateOptions{ProcessImages: true},
		},
		Result: &AmazonListingDraft{
			ProductTaskID:      "product-task-reuse",
			ProductImageTaskID: "image-task-reuse",
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if artifacts.Draft.ProductTaskID != "product-task-reuse" {
		t.Fatalf("expected reused product task id, got %q", artifacts.Draft.ProductTaskID)
	}
	if artifacts.Draft.ProductImageTaskID != "image-task-reuse" {
		t.Fatalf("expected reused image task id, got %q", artifacts.Draft.ProductImageTaskID)
	}
	if artifacts.Draft.Title != "Reuse Product" {
		t.Fatalf("expected reused title, got %q", artifacts.Draft.Title)
	}
	if len(artifacts.Draft.ChildTasks) != 2 {
		t.Fatalf("expected 2 child tasks, got %d", len(artifacts.Draft.ChildTasks))
	}
}

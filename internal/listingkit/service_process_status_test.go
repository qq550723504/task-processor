package listingkit

import (
	"context"
	"testing"
	"time"

	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
)

type stubProcessStatusAssembler struct {
	result *ListingKitResult
}

func (a *stubProcessStatusAssembler) Assemble(task *Task, canonical *productenrich.CanonicalProduct, image *productimage.ImageProcessResult) *ListingKitResult {
	if a.result == nil {
		return &ListingKitResult{Summary: &GenerationSummary{}}
	}
	copied := *a.result
	return &copied
}

func TestProcessListingKitMarksNeedsReviewWhenSummaryRequiresReview(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	productTask := &productenrich.Task{
		ID:      "product-task-1",
		Request: &productenrich.GenerateRequest{ProductURL: "https://example.com/product"},
	}
	productService := &stubWorkflowProductService{
		task: productTask,
		product: &productenrich.ProductJSON{
			Title:      "Travel Bag",
			Category:   []string{"bags"},
			Attributes: map[string]string{"color": "black"},
		},
	}

	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: productService,
		Assembler: &stubProcessStatusAssembler{
			result: &ListingKitResult{
				TaskID:  "listingkit-needs-review-1",
				Shein:   &SheinPackage{},
				Summary: &GenerationSummary{NeedsReview: true, Warnings: []string{"scene images require manual review"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	task := &Task{
		ID:        "listingkit-needs-review-1",
		Status:    TaskStatusPending,
		Request:   &GenerateRequest{ProductURL: "https://example.com/product", Platforms: []string{"shein"}},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	result, err := svc.ProcessListingKit(context.Background(), task)
	if err != nil {
		t.Fatalf("ProcessListingKit() error = %v", err)
	}
	if result.Status != string(TaskStatusNeedsReview) {
		t.Fatalf("result status = %q, want %q", result.Status, TaskStatusNeedsReview)
	}
	stored, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if stored.Status != TaskStatusNeedsReview {
		t.Fatalf("stored status = %q, want %q", stored.Status, TaskStatusNeedsReview)
	}
	if stored.Result == nil || stored.Result.Status != string(TaskStatusNeedsReview) {
		t.Fatalf("stored result = %+v, want needs_review result status", stored.Result)
	}
	if stored.Error == "" {
		t.Fatal("stored error/review reason is empty, want persisted review reason")
	}
}

func TestGetTaskResultTreatsNeedsReviewAsTerminal(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	now := time.Now()
	task := &Task{
		ID:        "listingkit-terminal-needs-review-1",
		Status:    TaskStatusNeedsReview,
		Request:   &GenerateRequest{Platforms: []string{"shein"}},
		Result:    &ListingKitResult{TaskID: "listingkit-terminal-needs-review-1"},
		CreatedAt: now.Add(-time.Minute),
		UpdatedAt: now,
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	svc := &service{repo: repo}
	result, err := svc.GetTaskResult(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTaskResult() error = %v", err)
	}
	if result.CompletedAt == nil {
		t.Fatal("CompletedAt = nil, want terminal timestamp for needs_review")
	}
	if !result.CompletedAt.Equal(now) {
		t.Fatalf("CompletedAt = %v, want %v", result.CompletedAt, now)
	}
}

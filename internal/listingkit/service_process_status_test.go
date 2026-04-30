package listingkit

import (
	"context"
	"testing"
	"time"

	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
	sheinpub "task-processor/internal/publishing/shein"
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
	if got, want := stored.Result.ReviewReasons, []string{"scene images require manual review"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("stored review reasons = %#v, want %#v", got, want)
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

func TestGetTaskResultReturnsStructuredReviewReasons(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	now := time.Now()
	task := &Task{
		ID:     "listingkit-review-reasons-1",
		Status: TaskStatusNeedsReview,
		Request: &GenerateRequest{
			Platforms: []string{"shein"},
		},
		Result: &ListingKitResult{
			TaskID:        "listingkit-review-reasons-1",
			Status:        string(TaskStatusNeedsReview),
			ReviewReasons: []string{"reason one", "reason two"},
			Summary:       &GenerationSummary{Warnings: []string{"legacy warning"}},
		},
		Error:     "legacy summary string",
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
	if got, want := result.ReviewReasons, []string{"reason one", "reason two"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("ReviewReasons = %#v, want %#v", got, want)
	}
}

func TestGetTaskResultFallsBackToTaskErrorForReviewReasons(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	now := time.Now()
	task := &Task{
		ID:     "listingkit-review-reasons-fallback-1",
		Status: TaskStatusNeedsReview,
		Request: &GenerateRequest{
			Platforms: []string{"shein"},
		},
		Error:     "single fallback reason",
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
	if got, want := result.ReviewReasons, []string{"single fallback reason"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("ReviewReasons = %#v, want %#v", got, want)
	}
}

func TestGetTaskResultFallsBackToSummaryWarningsForReviewReasons(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	now := time.Now()
	task := &Task{
		ID:     "listingkit-review-reasons-summary-1",
		Status: TaskStatusNeedsReview,
		Request: &GenerateRequest{
			Platforms: []string{"shein"},
		},
		Result: &ListingKitResult{
			TaskID:  "listingkit-review-reasons-summary-1",
			Status:  string(TaskStatusNeedsReview),
			Summary: &GenerationSummary{Warnings: []string{"reason one", "reason one", "reason two"}},
		},
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
	if got, want := result.ReviewReasons, []string{"reason one", "reason two"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("ReviewReasons = %#v, want %#v", got, want)
	}
}

func TestProcessListingKitInitializesDefaultSheinPricing(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	productTask := &productenrich.Task{
		ID:      "product-task-pricing-1",
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
				TaskID: "listingkit-pricing-1",
				Shein: &SheinPackage{
					RequestDraft: &sheinpub.RequestDraft{
						SKCList: []sheinpub.SKCRequestDraft{
							{
								SupplierCode: "SUP-1",
								SKUList: []sheinpub.SKUDraft{
									{
										SupplierSKU: "SKU-1",
										CostPrice:   "48.8",
									},
								},
							},
						},
					},
				},
				Summary: &GenerationSummary{},
			},
		},
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	task := &Task{
		ID:        "listingkit-pricing-1",
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
	if result.Shein == nil || result.Shein.Pricing == nil {
		t.Fatalf("result shein pricing = %+v, want initialized pricing", result.Shein)
	}
	if !result.Shein.Pricing.Ready {
		t.Fatalf("pricing ready = false, want true")
	}
	if len(result.Shein.Pricing.SKUPrices) != 1 {
		t.Fatalf("pricing sku prices = %+v, want 1 price review", result.Shein.Pricing.SKUPrices)
	}
	if got := result.Shein.Pricing.SKUPrices[0].FinalPrice; got <= 0 {
		t.Fatalf("final price = %v, want > 0", got)
	}
	if got := result.Shein.RequestDraft.SKCList[0].SKUList[0].BasePrice; got == "" {
		t.Fatalf("request draft base price = %q, want populated price", got)
	}
}

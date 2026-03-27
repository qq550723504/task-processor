package amazonlisting

import (
	"context"
	"testing"
	"time"

	amazonapi "task-processor/internal/amazon/api"
	"task-processor/internal/productenrich"
)

type mockListingSubmitter struct {
	previewResp  *amazonapi.ListingResponse
	previewQueue []*amazonapi.ListingResponse
	createResp   *amazonapi.ListingResponse
	updateResp   *amazonapi.ListingResponse
	err          error
}

type stubProductService struct{}

type stubRepository struct {
	task *Task
}

func (s *stubProductService) CreateGenerateTask(_ context.Context, _ *productenrich.GenerateRequest) (*productenrich.Task, error) {
	return nil, nil
}

func (s *stubProductService) ProcessProduct(_ context.Context, _ *productenrich.Task) (*productenrich.ProductJSON, error) {
	return nil, nil
}

func (r *stubRepository) CreateTask(_ context.Context, task *Task) error {
	copied := *task
	r.task = &copied
	return nil
}

func (r *stubRepository) GetTask(_ context.Context, taskID string) (*Task, error) {
	if r.task == nil || r.task.ID != taskID {
		return nil, ErrTaskNotFound
	}
	copied := *r.task
	return &copied, nil
}

func (r *stubRepository) MarkProcessing(_ context.Context, _ string) error { return nil }
func (r *stubRepository) MarkCompleted(_ context.Context, _ string, result *AmazonListingDraft) error {
	r.task.Result = result
	r.task.Status = TaskStatusCompleted
	return nil
}
func (r *stubRepository) MarkNeedsReview(_ context.Context, _ string, result *AmazonListingDraft, reason string) error {
	r.task.Result = result
	r.task.Status = TaskStatusNeedsReview
	r.task.Error = reason
	return nil
}
func (r *stubRepository) MarkRejected(_ context.Context, _ string, reason string) error {
	r.task.Status = TaskStatusRejected
	r.task.Error = reason
	return nil
}
func (r *stubRepository) MarkFailed(_ context.Context, _ string, errorMsg string) error {
	r.task.Status = TaskStatusFailed
	r.task.Error = errorMsg
	return nil
}
func (r *stubRepository) PrepareRetry(_ context.Context, _ string) error        { return nil }
func (r *stubRepository) IncrementRetryCount(_ context.Context, _ string) error { return nil }
func (r *stubRepository) UpdateTaskStatus(_ context.Context, _ string, status TaskStatus) error {
	r.task.Status = status
	return nil
}
func (r *stubRepository) UpdateTaskError(_ context.Context, _ string, errorMsg string) error {
	r.task.Error = errorMsg
	return nil
}
func (r *stubRepository) SaveTaskResult(_ context.Context, _ string, result *AmazonListingDraft) error {
	r.task.Result = result
	return nil
}
func (r *stubRepository) ResetForRetry(_ context.Context, _ string) error { return nil }

func (m *mockListingSubmitter) Preview(_ context.Context, _ *AmazonListingsAPIExport) (*amazonapi.ListingResponse, error) {
	if len(m.previewQueue) > 0 {
		resp := m.previewQueue[0]
		m.previewQueue = m.previewQueue[1:]
		return resp, m.err
	}
	return m.previewResp, m.err
}

func (m *mockListingSubmitter) Create(_ context.Context, _ *AmazonListingsAPIExport) (*amazonapi.ListingResponse, error) {
	return m.createResp, m.err
}

func (m *mockListingSubmitter) Update(_ context.Context, _ *AmazonListingsAPIExport) (*amazonapi.ListingResponse, error) {
	return m.updateResp, m.err
}

func TestSubmitTaskPreviewStoresSubmissionRecord(t *testing.T) {
	repo := &stubRepository{}
	svc, err := NewService(&ServiceConfig{
		Repository:       repo,
		ProductService:   &stubProductService{},
		Assembler:        NewAssembler(),
		ExportBuilder:    NewExportBuilder(),
		Validator:        NewValidator(),
		AutoFixer:        NewAutoFixer(),
		ListingSubmitter: &mockListingSubmitter{previewResp: &amazonapi.ListingResponse{SKU: "SKU-1", Status: "VALID"}},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	task := &Task{
		ID:        "task-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Result: &AmazonListingDraft{
			TaskID: "task-1",
			Export: &AmazonListingExport{
				ListingsAPI: &AmazonListingsAPIExport{
					SKU:                      "SKU-1",
					MarketplaceID:            "ATVPDKIKX0DER",
					ProductType:              "KITCHEN",
					Requirements:             "LISTING",
					Attributes:               map[string]any{"item_name": "demo"},
					ValidationPreviewRequest: &amazonapi.ListingRequest{SKU: "SKU-1", ProductType: "KITCHEN"},
					CreateRequest:            &amazonapi.ListingRequest{SKU: "SKU-1", ProductType: "KITCHEN"},
					UpdateRequest:            &amazonapi.ListingRequest{SKU: "SKU-1", ProductType: "KITCHEN"},
				},
			},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	result, err := svc.SubmitTask(context.Background(), "task-1", &SubmitTaskRequest{Action: "preview"})
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}
	if result.Result == nil || result.Result.Submission == nil {
		t.Fatalf("expected submission report")
	}
	if result.Result.Submission.LastAction != "preview" {
		t.Fatalf("unexpected last action: %s", result.Result.Submission.LastAction)
	}
	if result.Result.Submission.Preview == nil || result.Result.Submission.Preview.Status != "VALID" {
		t.Fatalf("expected preview response to be stored")
	}
}

func TestSubmitTaskPreviewAndFixAppliesAmazonIssueFixes(t *testing.T) {
	repo := &stubRepository{}
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: &stubProductService{},
		Assembler:      NewAssembler(),
		ExportBuilder:  NewExportBuilder(),
		Validator:      NewValidator(),
		AutoFixer:      NewAutoFixer(),
		ListingSubmitter: &mockListingSubmitter{previewQueue: []*amazonapi.ListingResponse{
			{
				SKU:    "SKU-1",
				Status: "INVALID",
				Issues: []struct {
					Code     string `json:"code"`
					Message  string `json:"message"`
					Severity string `json:"severity"`
				}{
					{Code: "9001", Message: "Missing required attribute brand", Severity: "ERROR"},
					{Code: "9002", Message: "Missing required bullet point", Severity: "ERROR"},
				},
			},
			{
				SKU:    "SKU-1",
				Status: "VALID",
				Issues: nil,
			},
		}},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	task := &Task{
		ID:        "task-fix",
		Request:   &GenerateRequest{Marketplace: "amazon", Country: "US", BrandHint: "Acme"},
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Result: &AmazonListingDraft{
			TaskID:       "task-fix",
			Marketplace:  "amazon",
			Country:      "US",
			Title:        "Ceramic Mug",
			Description:  "A ceramic mug for coffee and tea.",
			SearchTerms:  []string{"coffee mug", "ceramic cup"},
			ProductType:  "Kitchen",
			BulletPoints: nil,
			Export: &AmazonListingExport{
				ListingsAPI: &AmazonListingsAPIExport{
					SKU:                      "SKU-1",
					MarketplaceID:            "ATVPDKIKX0DER",
					ProductType:              "KITCHEN",
					Requirements:             "LISTING",
					Attributes:               map[string]any{"item_name": "demo"},
					ValidationPreviewRequest: &amazonapi.ListingRequest{SKU: "SKU-1", ProductType: "KITCHEN"},
					CreateRequest:            &amazonapi.ListingRequest{SKU: "SKU-1", ProductType: "KITCHEN"},
					UpdateRequest:            &amazonapi.ListingRequest{SKU: "SKU-1", ProductType: "KITCHEN"},
				},
			},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	result, err := svc.SubmitTask(context.Background(), "task-fix", &SubmitTaskRequest{Action: "preview_and_fix"})
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}
	if result.Result == nil {
		t.Fatalf("expected result")
	}
	if len(result.Result.LastAmazonIssues) != 0 {
		t.Fatalf("expected latest issues to be cleared after successful second preview")
	}
	if result.Result.Brand != "Acme" {
		t.Fatalf("expected brand to be fixed, got %s", result.Result.Brand)
	}
	if len(result.Result.BulletPoints) == 0 {
		t.Fatalf("expected bullet points to be rebuilt")
	}
	if len(result.Result.FixHistory) == 0 {
		t.Fatalf("expected fix history")
	}
	if result.Result.Export == nil || result.Result.Export.ListingsAPI == nil {
		t.Fatalf("expected export to be rebuilt")
	}
	if result.Result.Submission == nil || result.Result.Submission.PreviewBeforeFix == nil || result.Result.Submission.PreviewAfterFix == nil {
		t.Fatalf("expected before/after preview records")
	}
	if result.Result.Submission.PreviewAfterFix.Status != "VALID" {
		t.Fatalf("expected second preview to be valid")
	}
	if result.Result.Submission.FixEvaluation == nil {
		t.Fatalf("expected fix evaluation")
	}
	if !result.Result.Submission.FixEvaluation.FullyResolvedBlocking {
		t.Fatalf("expected blocking issues to be fully resolved")
	}
	if result.Status != TaskStatusCompleted {
		t.Fatalf("expected task status completed, got %s", result.Status)
	}
}

func TestSubmitTaskPreviewAndFixMarksNeedsReviewForManualIssues(t *testing.T) {
	repo := &stubRepository{}
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: &stubProductService{},
		Assembler:      NewAssembler(),
		ExportBuilder:  NewExportBuilder(),
		Validator:      NewValidator(),
		AutoFixer:      NewAutoFixer(),
		ListingSubmitter: &mockListingSubmitter{previewQueue: []*amazonapi.ListingResponse{
			{
				SKU:    "SKU-2",
				Status: "INVALID",
				Issues: []struct {
					Code     string `json:"code"`
					Message  string `json:"message"`
					Severity string `json:"severity"`
				}{
					{Code: "9999", Message: "Restricted product compliance approval required", Severity: "ERROR"},
				},
			},
			{
				SKU:    "SKU-2",
				Status: "INVALID",
				Issues: []struct {
					Code     string `json:"code"`
					Message  string `json:"message"`
					Severity string `json:"severity"`
				}{
					{Code: "9999", Message: "Restricted product compliance approval required", Severity: "ERROR"},
				},
			},
		}},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	task := &Task{
		ID:        "task-manual",
		Request:   &GenerateRequest{Marketplace: "amazon", Country: "US"},
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Result: &AmazonListingDraft{
			TaskID:      "task-manual",
			Marketplace: "amazon",
			Country:     "US",
			Title:       "Ceramic Mug",
			Description: "A ceramic mug for coffee and tea with durable material and daily use convenience.",
			Export: &AmazonListingExport{
				ListingsAPI: &AmazonListingsAPIExport{
					SKU:                      "SKU-2",
					MarketplaceID:            "ATVPDKIKX0DER",
					ProductType:              "KITCHEN",
					Requirements:             "LISTING",
					Attributes:               map[string]any{"item_name": "demo"},
					ValidationPreviewRequest: &amazonapi.ListingRequest{SKU: "SKU-2", ProductType: "KITCHEN"},
					CreateRequest:            &amazonapi.ListingRequest{SKU: "SKU-2", ProductType: "KITCHEN"},
					UpdateRequest:            &amazonapi.ListingRequest{SKU: "SKU-2", ProductType: "KITCHEN"},
				},
			},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	result, err := svc.SubmitTask(context.Background(), "task-manual", &SubmitTaskRequest{Action: "preview_and_fix"})
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}
	if result.Status != TaskStatusNeedsReview {
		t.Fatalf("expected needs_review status, got %s", result.Status)
	}
	if result.Result == nil || result.Result.Submission == nil || result.Result.Submission.IssueSummary == nil {
		t.Fatalf("expected issue summary")
	}
	if result.Result.Submission.IssueSummary.ManualCount != 1 {
		t.Fatalf("expected one manual issue")
	}
	if len(result.Result.Submission.IssueSummary.ManualAdvices) == 0 {
		t.Fatalf("expected manual advices for operators")
	}
	if len(result.Result.Submission.IssueSummary.ManualActions) == 0 {
		t.Fatalf("expected manual action enums")
	}
	if result.Result.Submission.IssueSummary.ManualActions[0] != OperatorActionCheckCompliance {
		t.Fatalf("expected compliance action enum, got %s", result.Result.Submission.IssueSummary.ManualActions[0])
	}
	if !result.Result.Review.NeedsReview {
		t.Fatalf("expected review to be required")
	}
}

func TestSubmitTaskRequiresConfiguredSubmitter(t *testing.T) {
	repo := &stubRepository{}
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: &stubProductService{},
		Assembler:      NewAssembler(),
		ExportBuilder:  NewExportBuilder(),
		Validator:      NewValidator(),
		AutoFixer:      NewAutoFixer(),
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = svc.SubmitTask(context.Background(), "missing", &SubmitTaskRequest{Action: "preview"})
	if err == nil {
		t.Fatalf("expected error when submitter is missing")
	}
}

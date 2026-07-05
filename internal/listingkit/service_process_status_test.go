package listingkit

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"task-processor/internal/catalog/canonical"
	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

type stubProcessStatusAssembler struct {
	result *ListingKitResult
}

type stubProcessStatusRepo struct {
	*stubGenerationRepo
	completedTaskID string
	completedCalls  int
	failedTaskID    string
	failedError     string
	failedCalls     int
	savedResults    []*ListingKitResult
}

func (a *stubProcessStatusAssembler) Assemble(task *Task, canonical *canonical.Product, image *productimage.ImageProcessResult) *ListingKitResult {
	if a.result == nil {
		return &ListingKitResult{Summary: &GenerationSummary{}}
	}
	copied := *a.result
	return &copied
}

func (r *stubProcessStatusRepo) MarkCompleted(ctx context.Context, taskID string, result *ListingKitResult) error {
	r.completedTaskID = taskID
	r.completedCalls++
	if err := r.SaveTaskResult(ctx, taskID, result); err != nil {
		return err
	}
	r.task.Status = TaskStatusCompleted
	return nil
}

func (r *stubProcessStatusRepo) MarkFailed(_ context.Context, taskID string, errorMsg string) error {
	r.failedTaskID = taskID
	r.failedError = errorMsg
	r.failedCalls++
	if r.task != nil && r.task.ID == taskID {
		r.task.Status = TaskStatusFailed
		r.task.Error = errorMsg
	}
	return nil
}

func (r *stubProcessStatusRepo) SaveTaskResult(ctx context.Context, taskID string, result *ListingKitResult) error {
	r.savedResults = append(r.savedResults, result)
	return r.stubGenerationRepo.SaveTaskResult(ctx, taskID, result)
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

	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestProductService(productService),
		withTestAssembler(&stubProcessStatusAssembler{
			result: &ListingKitResult{
				TaskID:  "listingkit-needs-review-1",
				Shein:   &SheinPackage{},
				Summary: &GenerationSummary{NeedsReview: true, Warnings: []string{"scene images require manual review"}},
			},
		}),
	))
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

func TestProcessListingKitMarksCompletedWhenSummaryDoesNotRequireReview(t *testing.T) {
	t.Parallel()

	repo := &stubProcessStatusRepo{stubGenerationRepo: &stubGenerationRepo{}}
	productTask := &productenrich.Task{
		ID:      "product-task-completed-1",
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

	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestProductService(productService),
		withTestAssembler(&stubProcessStatusAssembler{
			result: &ListingKitResult{
				TaskID:  "listingkit-completed-1",
				Shein:   &SheinPackage{},
				Summary: &GenerationSummary{},
			},
		}),
	))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	task := &Task{
		ID:        "listingkit-completed-1",
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
	if result.Status != string(TaskStatusCompleted) {
		t.Fatalf("result status = %q, want %q", result.Status, TaskStatusCompleted)
	}
	stored, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if stored.Status != TaskStatusCompleted {
		t.Fatalf("stored status = %q, want %q", stored.Status, TaskStatusCompleted)
	}
	if stored.Result == nil || stored.Result.Status != string(TaskStatusCompleted) {
		t.Fatalf("stored result = %+v, want completed result status", stored.Result)
	}
	if repo.completedCalls != 1 || repo.completedTaskID != task.ID {
		t.Fatalf("MarkCompleted calls = %d for %q, want 1 for %q", repo.completedCalls, repo.completedTaskID, task.ID)
	}
	if repo.failedCalls != 0 {
		t.Fatalf("MarkFailed calls = %d, want 0", repo.failedCalls)
	}
}

func TestDeriveProcessTerminalStatusMarksNeedsReviewWhenRequiredPODExecutionFails(t *testing.T) {
	t.Parallel()

	result := &ListingKitResult{
		TaskID: "listingkit-sds-failed-1",
		Summary: &GenerationSummary{
			Warnings: []string{"SDS render failed for selected color variants: white"},
		},
		PodExecution: &PodExecutionSummary{
			Provider:       podProviderSDS,
			DependencyMode: podDependencyModeRequired,
			Status:         podStatusFailedBlocking,
			FailureReason:  "SDS render failed for selected color variants: white",
		},
	}

	if status := deriveProcessTerminalStatus(result); status != TaskStatusNeedsReview {
		t.Fatalf("deriveProcessTerminalStatus() = %q, want %q", status, TaskStatusNeedsReview)
	}
	applied := applyProcessTerminalResult(result, TaskStatusNeedsReview)
	if applied.Status != string(TaskStatusNeedsReview) {
		t.Fatalf("result status = %q, want %q", applied.Status, TaskStatusNeedsReview)
	}
	if got := applied.ReviewReasons; len(got) != 1 || !strings.Contains(got[0], "SDS render failed") {
		t.Fatalf("review reasons = %#v, want SDS failure reason", got)
	}
}

func TestProcessListingKitMarksSheinCookieUnavailableAsBlockingIssue(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	productTask := &productenrich.Task{
		ID:      "product-task-cookie-1",
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
	cookieNote := "SHEIN 店铺 cookie 不可用，已降级为离线解析"

	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestProductService(productService),
		withTestAssembler(&stubProcessStatusAssembler{
			result: &ListingKitResult{
				TaskID: "listingkit-cookie-blocking-1",
				Shein: &SheinPackage{
					ReviewNotes: []string{cookieNote},
				},
				Summary: &GenerationSummary{NeedsReview: true, Warnings: []string{cookieNote}},
			},
		}),
	))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	task := &Task{
		ID:        "listingkit-cookie-blocking-1",
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
	var cookieIssue *WorkflowIssue
	for i := range result.WorkflowIssues {
		if result.WorkflowIssues[i].Code == sheinCookieUnavailableIssueCode {
			cookieIssue = &result.WorkflowIssues[i]
			break
		}
	}
	if cookieIssue == nil {
		t.Fatalf("workflow issues = %+v, want %s", result.WorkflowIssues, sheinCookieUnavailableIssueCode)
	}
	if cookieIssue.Severity != WorkflowIssueSeverityBlocking {
		t.Fatalf("cookie issue severity = %q, want blocking", cookieIssue.Severity)
	}
	if cookieIssue.Message != sheinCookieUnavailableMessage || cookieIssue.Detail != cookieNote {
		t.Fatalf("cookie issue = %+v, want message/detail from cookie note", cookieIssue)
	}
	if result.Summary == nil || result.Summary.BlockingCount != 1 || !result.Summary.NeedsReview {
		t.Fatalf("summary = %+v, want one blocking review issue", result.Summary)
	}
	if got, want := result.ReviewReasons, []string{sheinCookieUnavailableMessage}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("review reasons = %#v, want %#v", got, want)
	}
}

func TestProcessListingKitPersistsPartialResultBeforeMarkingFailed(t *testing.T) {
	t.Parallel()

	repo := &stubProcessStatusRepo{stubGenerationRepo: &stubGenerationRepo{}}
	productTask := &productenrich.Task{
		ID:      "product-task-failed-1",
		Request: &productenrich.GenerateRequest{ProductURL: "https://example.com/product"},
	}
	productService := &stubWorkflowProductService{
		task:       productTask,
		processErr: errors.New("upstream product enrich failed"),
	}

	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestProductService(productService),
		withTestAssembler(&stubProcessStatusAssembler{}),
	))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	task := &Task{
		ID:        "listingkit-failed-1",
		Status:    TaskStatusPending,
		Request:   &GenerateRequest{ProductURL: "https://example.com/product", Platforms: []string{"shein"}},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	result, err := svc.ProcessListingKit(context.Background(), task)
	if err == nil {
		t.Fatal("ProcessListingKit() error = nil, want workflow failure")
	}
	if result != nil {
		t.Fatalf("ProcessListingKit() result = %+v, want nil on failure", result)
	}
	if repo.failedCalls != 1 || repo.failedTaskID != task.ID {
		t.Fatalf("MarkFailed calls = %d for %q, want 1 for %q", repo.failedCalls, repo.failedTaskID, task.ID)
	}
	if repo.failedError == "" || !strings.Contains(repo.failedError, "product enrichment failed") {
		t.Fatalf("MarkFailed error = %q, want wrapped product enrichment error", repo.failedError)
	}
	if len(repo.savedResults) != 1 || repo.savedResults[0] == nil {
		t.Fatalf("saved results = %+v, want one partial result before failure", repo.savedResults)
	}
	stored, getErr := repo.GetTask(context.Background(), task.ID)
	if getErr != nil {
		t.Fatalf("GetTask() error = %v", getErr)
	}
	if stored.Result == nil {
		t.Fatal("stored result = nil, want persisted partial workflow result")
	}
	if stored.Status != TaskStatusFailed {
		t.Fatalf("stored status = %q, want %q", stored.Status, TaskStatusFailed)
	}
	foundFailedChild := false
	for _, child := range stored.Result.ChildTasks {
		if child.Kind == "product_enrich" && child.Status == string(TaskStatusFailed) {
			foundFailedChild = true
			break
		}
	}
	if !foundFailedChild {
		t.Fatalf("child tasks = %+v, want failed product_enrich child in persisted result", stored.Result.ChildTasks)
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

func TestGetTaskResultIncludesDerivedSheinSubmissionStatus(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	now := time.Now()
	task := &Task{
		ID:     "listingkit-shein-published-1",
		Status: TaskStatusCompleted,
		Request: &GenerateRequest{
			Platforms: []string{"shein"},
		},
		Result: &ListingKitResult{
			TaskID: "listingkit-shein-published-1",
			Shein: &SheinPackage{
				Submission: &sheinpub.SubmissionReport{
					LastAction:   "publish",
					LastStatus:   "success",
					RemoteStatus: "confirmed",
					Publish: &sheinpub.SubmissionRecord{
						Action: "publish",
						Status: "success",
					},
					LastResult: &sheinpub.SubmissionResponse{
						Success: true,
						SPUName: "SHEIN-SPU-1",
					},
				},
			},
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
	if result.SheinWorkflowStatus != SheinWorkflowStatusPublished {
		t.Fatalf("SheinWorkflowStatus = %q, want %q", result.SheinWorkflowStatus, SheinWorkflowStatusPublished)
	}
	if result.SheinLatestSubmissionStatus != "success" {
		t.Fatalf("SheinLatestSubmissionStatus = %q, want success", result.SheinLatestSubmissionStatus)
	}
	if result.SheinSubmissionRemoteStatus != "confirmed" {
		t.Fatalf("SheinSubmissionRemoteStatus = %q, want confirmed", result.SheinSubmissionRemoteStatus)
	}
}

func TestGetTaskResultDerivesPodExecutionForLegacyStoredResults(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	now := time.Now()
	task := &Task{
		ID:     "listingkit-pod-execution-legacy-1",
		Status: TaskStatusCompleted,
		Request: &GenerateRequest{
			Platforms: []string{"shein"},
			ImageURLs: []string{"https://cdn.example.com/source.png"},
			Options: &GenerateOptions{
				ProcessImages: false,
				SDS:           &SDSSyncOptions{ParentProductID: 1001, VariantID: 2002},
			},
		},
		Result: &ListingKitResult{
			TaskID: "listingkit-pod-execution-legacy-1",
			SDSDesignResult: &SDSSyncSummary{
				Status: "failed",
				Error:  "mockup sync timeout",
			},
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
	if result.Result == nil || result.Result.PodExecution == nil {
		t.Fatalf("result = %+v, want derived pod execution", result.Result)
	}
	if result.Result.PodExecution.Provider != podProviderSDS {
		t.Fatalf("provider = %q, want %q", result.Result.PodExecution.Provider, podProviderSDS)
	}
	if result.Result.PodExecution.DependencyMode != podDependencyModeRequired {
		t.Fatalf("dependency mode = %q, want %q", result.Result.PodExecution.DependencyMode, podDependencyModeRequired)
	}
	if result.Result.PodExecution.Status != podStatusFailedBlocking {
		t.Fatalf("status = %q, want %q", result.Result.PodExecution.Status, podStatusFailedBlocking)
	}
	if result.Result.PodExecution.FailureReason != "mockup sync timeout" {
		t.Fatalf("failure reason = %q, want mockup sync timeout", result.Result.PodExecution.FailureReason)
	}
}

func TestGetTaskResultPrefersWorkflowIssuesForReviewReasons(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	now := time.Now()
	task := &Task{
		ID:     "listingkit-review-reasons-workflow-1",
		Status: TaskStatusNeedsReview,
		Request: &GenerateRequest{
			Platforms: []string{"shein"},
		},
		Result: &ListingKitResult{
			TaskID:        "listingkit-review-reasons-workflow-1",
			Status:        string(TaskStatusNeedsReview),
			ReviewReasons: []string{"legacy review reason"},
			WorkflowIssues: []WorkflowIssue{
				{
					Code:     "shein_review_required",
					Severity: WorkflowIssueSeverityReview,
					Stage:    "shein_review",
					Message:  "structured workflow review reason",
				},
			},
			Summary: &GenerationSummary{Warnings: []string{"legacy warning"}},
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
	if got, want := result.ReviewReasons, []string{"structured workflow review reason"}; len(got) != len(want) || got[0] != want[0] {
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
			Summary: &GenerationSummary{NeedsReview: true, Warnings: []string{"reason one", "reason one", "reason two"}},
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

func TestGetTaskResultRefreshesStaleSheinCookieReviewState(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	now := time.Now()
	task := &Task{
		ID:     "listingkit-review-reasons-cookie-refresh-1",
		Status: TaskStatusNeedsReview,
		Request: &GenerateRequest{
			Platforms: []string{"shein"},
		},
		Result: &ListingKitResult{
			TaskID:        "listingkit-review-reasons-cookie-refresh-1",
			Status:        string(TaskStatusNeedsReview),
			ReviewReasons: []string{sheinCookieUnavailableMessage},
			WorkflowIssues: []WorkflowIssue{{
				Code:     sheinCookieUnavailableIssueCode,
				Severity: WorkflowIssueSeverityBlocking,
				Stage:    "shein_review",
				Message:  sheinCookieUnavailableMessage,
				Detail:   "SHEIN 店铺 cookie 不可用，已降级为离线解析",
			}},
			Summary: &GenerationSummary{
				NeedsReview: true,
				Warnings: []string{
					sheinCookieUnavailableMessage,
					"SHEIN 销售属性尚未完成真实 sale attribute 映射，当前仍需要人工确认变体规格",
				},
			},
			Shein: &SheinPackage{
				CategoryID:  10489,
				ReviewNotes: []string{"SHEIN 店铺 cookie 不可用，已降级为离线解析"},
				CategoryResolution: &sheinpub.CategoryResolution{
					Status:      "resolved",
					CategoryID:  10489,
					MatchedPath: []string{"运动&户外", "露营&远足", "野餐和营地厨房", "户外保温包"},
				},
				AttributeResolution: &sheinpub.AttributeResolution{
					Status:        "resolved",
					ResolvedCount: 16,
				},
				SaleAttributeResolution: &sheinpub.SaleAttributeResolution{
					Status:      "partial",
					ReviewNotes: []string{"SHEIN 店铺 cookie 不可用，已降级为离线解析"},
				},
			},
		},
		Error:     sheinCookieUnavailableMessage,
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
	if len(result.ReviewReasons) != 1 || result.ReviewReasons[0] != "SHEIN 销售属性尚未完成真实 sale attribute 映射，当前仍需要人工确认变体规格" {
		t.Fatalf("ReviewReasons = %#v, want refreshed sale attribute review reason", result.ReviewReasons)
	}
	if result.Error != "SHEIN 销售属性尚未完成真实 sale attribute 映射，当前仍需要人工确认变体规格" {
		t.Fatalf("Error = %q, want refreshed sale attribute review reason", result.Error)
	}
	for _, issue := range result.Result.WorkflowIssues {
		if issue.Code == sheinCookieUnavailableIssueCode {
			t.Fatalf("WorkflowIssues = %+v, want stale shein cookie issue removed", result.Result.WorkflowIssues)
		}
	}
	if len(result.Result.Shein.ReviewNotes) != 1 || strings.Contains(result.Result.Shein.ReviewNotes[0], "cookie 不可用") {
		t.Fatalf("Shein review notes = %#v, want cookie note removed", result.Result.Shein.ReviewNotes)
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

	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestProductService(productService),
		withTestAssembler(&stubProcessStatusAssembler{
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
		}),
	))
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

func TestProcessListingKitReusesPublishedSheinPricingCache(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	cacheStore := &submitResolutionCacheStore{}
	productTask := &productenrich.Task{
		ID:      "product-task-pricing-cache-1",
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

	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestProductService(productService),
		withTestConfig(func(cfg *ServiceConfig) {
			cfg.Shein.SheinResolutionCacheStore = cacheStore
		}),
		withTestAssembler(&stubProcessStatusAssembler{
			result: &ListingKitResult{
				TaskID: "listingkit-pricing-cache-1",
				Shein: &SheinPackage{
					SpuName:        "Travel Bag SKU-1",
					ProductNameEn:  "Travel Bag",
					CategoryPath:   []string{"Bags", "Travel Bags"},
					CategoryID:     3221,
					CategoryIDList: []int{1, 2, 3221},
					ProductAttributes: []common.Attribute{
						{Name: "sku", Value: "SKU-1"},
						{Name: "material", Value: "Canvas"},
					},
					RequestDraft: &sheinpub.RequestDraft{
						SKCList: []sheinpub.SKCRequestDraft{
							{
								SupplierCode: "SUP-1",
								SKUList: []sheinpub.SKUDraft{
									{
										SupplierSKU: "SKU-1",
										CostPrice:   "48.8",
										BasePrice:   "19.99",
										Currency:    "USD",
										SitePriceList: []sheinpub.SitePrice{{
											SubSite:   "US",
											BasePrice: "19.99",
											Currency:  "USD",
										}},
									},
								},
							},
						},
					},
					PreviewProduct: &sheinproduct.Product{
						SKCList: []sheinproduct.SKC{{
							SKUS: []sheinproduct.SKU{{
								SupplierSKU: "SKU-1",
								PriceInfoList: []sheinproduct.PriceInfo{{
									SubSite:   "US",
									BasePrice: 19.99,
									Currency:  "USD",
								}},
							}},
						}},
					},
				},
				Summary: &GenerationSummary{},
			},
		}),
	))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	seedTask := &Task{
		ID: "seed-pricing-cache-1",
		Request: &GenerateRequest{
			Platforms:    []string{"shein"},
			SheinStoreID: 869,
		},
		Result: &ListingKitResult{
			Shein: &SheinPackage{
				SpuName:        "Travel Bag SKU-1",
				ProductNameEn:  "Travel Bag",
				CategoryPath:   []string{"Bags", "Travel Bags"},
				CategoryID:     3221,
				CategoryIDList: []int{1, 2, 3221},
				ProductAttributes: []common.Attribute{
					{Name: "sku", Value: "SKU-1"},
					{Name: "material", Value: "Canvas"},
				},
				RequestDraft: &sheinpub.RequestDraft{
					SKCList: []sheinpub.SKCRequestDraft{
						{
							SupplierCode: "SUP-1",
							SKUList: []sheinpub.SKUDraft{
								{
									SupplierSKU: "SKU-1",
									CostPrice:   "48.8",
									BasePrice:   "27.99",
									Currency:    "USD",
									SitePriceList: []sheinpub.SitePrice{{
										SubSite:   "US",
										BasePrice: "27.99",
										Currency:  "USD",
									}},
								},
							},
						},
					},
				},
				Pricing: &sheinpub.PricingReview{
					RuleSnapshot: &sheinpub.PricingRule{
						SourceCurrency:   "CNY",
						TargetCurrency:   "USD",
						ExchangeRate:     7.2,
						MarkupMultiplier: 2,
						MinimumPrice:     9.99,
						RoundTo:          0.01,
					},
					SKUPrices: []sheinpub.SKUPriceReview{{
						SupplierSKU:     "SKU-1",
						SupplierCode:    "SUP-1",
						CostCNY:         48.8,
						CalculatedPrice: 19.99,
						FinalPrice:      27.99,
						Currency:        "USD",
						Manual:          true,
					}},
					Ready: true,
				},
			},
		},
	}
	svc.(*service).rememberSheinSubmittedPricing(seedTask, "publish")

	task := &Task{
		ID:        "listingkit-pricing-cache-1",
		Status:    TaskStatusPending,
		Request:   &GenerateRequest{ProductURL: "https://example.com/product", Platforms: []string{"shein"}, SheinStoreID: 869},
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
		t.Fatalf("result shein pricing = %+v, want cached pricing", result.Shein)
	}
	if result.Shein.Pricing.Cache == nil || result.Shein.Pricing.Cache.Source != "manual_cache" {
		t.Fatalf("pricing cache = %+v, want manual_cache", result.Shein.Pricing.Cache)
	}
	if result.Shein.Pricing.Cache.HitSource != sheinpub.ResolutionCacheHitSourcePersistentManualCache {
		t.Fatalf("pricing hit source = %q, want %q", result.Shein.Pricing.Cache.HitSource, sheinpub.ResolutionCacheHitSourcePersistentManualCache)
	}
	if got := result.Shein.Pricing.SKUPrices[0].FinalPrice; got != 27.99 {
		t.Fatalf("final price = %v, want cached 27.99", got)
	}
	if got := result.Shein.RequestDraft.SKCList[0].SKUList[0].BasePrice; got != "27.99" {
		t.Fatalf("request draft base price = %q, want cached 27.99", got)
	}
	preview := buildSheinPreviewPayload(result.Shein, result.PodExecution, result.CanonicalProduct, nil, nil)
	if preview == nil || preview.ResolutionCache == nil || preview.ResolutionCache.Pricing == nil {
		t.Fatalf("preview resolution cache = %+v, want pricing cache summary", preview)
	}
	if preview.ResolutionCache.Pricing.Source != "manual_cache" {
		t.Fatalf("preview pricing cache source = %q, want manual_cache", preview.ResolutionCache.Pricing.Source)
	}
	if preview.ResolutionCache.Pricing.HitSource != sheinpub.ResolutionCacheHitSourcePersistentManualCache {
		t.Fatalf("preview pricing hit source = %q, want %q", preview.ResolutionCache.Pricing.HitSource, sheinpub.ResolutionCacheHitSourcePersistentManualCache)
	}
	if preview.ResolutionCache.Pricing.UpdatedAt == nil {
		t.Fatalf("preview pricing updated_at = nil, want cache timestamp")
	}
	if preview.ResolutionCache.Pricing.DisplayValue == "" {
		t.Fatalf("preview pricing display_value = empty, want summary")
	}
}

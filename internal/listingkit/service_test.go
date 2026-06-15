package listingkit

import (
	"context"
	"sync"
	"testing"
	"time"

	"task-processor/internal/catalog/canonical"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/infra/worker"
	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func TestNormalizeGenerateRequestDefaults(t *testing.T) {
	t.Parallel()

	req := &GenerateRequest{
		Text:      "demo",
		Platforms: []string{" Amazon ", "shein", "amazon", "invalid", "TEMU"},
	}

	normalizeGenerateRequest(req)

	if req.Country != "US" {
		t.Fatalf("country = %q, want US", req.Country)
	}
	if req.Language != "en_US" {
		t.Fatalf("language = %q, want en_US", req.Language)
	}
	if req.Options == nil || !req.Options.ProcessImages {
		t.Fatal("expected default options with process_images=true")
	}
	if got, want := len(req.Platforms), 3; got != want {
		t.Fatalf("platform count = %d, want %d", got, want)
	}
	if req.Platforms[0] != "amazon" || req.Platforms[1] != "shein" || req.Platforms[2] != "temu" {
		t.Fatalf("normalized platforms = %#v", req.Platforms)
	}
}

func TestNormalizeGenerateRequestAbsolutizesUploadedImageURLs(t *testing.T) {
	t.Parallel()

	req := &GenerateRequest{
		Text:      "demo",
		ImageURLs: []string{"/api/v1/listing-kits/uploads/files/20260610/demo.png", " https://example.com/keep.png "},
		Platforms: []string{"shein"},
	}

	normalizeGenerateRequest(req)

	if got, want := req.ImageURLs[0], "http://localhost:3000/api/v1/listing-kits/uploads/files/20260610/demo.png"; got != want {
		t.Fatalf("first image url = %q, want %q", got, want)
	}
	if got, want := req.ImageURLs[1], "https://example.com/keep.png"; got != want {
		t.Fatalf("second image url = %q, want %q", got, want)
	}
}

func TestNormalizeGenerateRequestEnablesProcessImagesWhenSceneOptionsProvided(t *testing.T) {
	t.Parallel()

	req := &GenerateRequest{
		ProductURL: "https://detail.1688.com/offer/123.html",
		Platforms:  []string{"shein"},
		Options: &GenerateOptions{
			Scene: &productimage.SceneGenerationOptions{
				SceneCategory: "shoes",
			},
		},
	}

	normalizeGenerateRequest(req)

	if req.Options == nil {
		t.Fatal("expected options to remain present")
	}
	if !req.Options.ProcessImages {
		t.Fatal("expected process_images=true when scene options are provided")
	}
}

func TestApplyGenerateRequestDefaultsSetsSheinStoreIDForSingleStoreConfig(t *testing.T) {
	t.Parallel()

	req := &GenerateRequest{
		Text:      "demo",
		Platforms: []string{"shein"},
	}

	applyGenerateRequestDefaults(req, generateRequestDefaults{sheinDefaultStoreID: 873})

	if req.SheinStoreID != 873 {
		t.Fatalf("shein_store_id = %d, want 873", req.SheinStoreID)
	}
}

func TestApplyGenerateRequestDefaultsKeepsExplicitSheinStoreID(t *testing.T) {
	t.Parallel()

	req := &GenerateRequest{
		Text:         "demo",
		Platforms:    []string{"shein"},
		SheinStoreID: 431,
	}

	applyGenerateRequestDefaults(req, generateRequestDefaults{sheinDefaultStoreID: 873})

	if req.SheinStoreID != 431 {
		t.Fatalf("shein_store_id = %d, want 431", req.SheinStoreID)
	}
}

func TestValidateRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		req     *GenerateRequest
		wantErr bool
	}{
		{
			name: "valid text request",
			req: &GenerateRequest{
				Text:      "demo",
				Platforms: []string{"amazon"},
			},
		},
		{
			name: "missing inputs",
			req: &GenerateRequest{
				Platforms: []string{"amazon"},
			},
			wantErr: true,
		},
		{
			name: "missing platforms",
			req: &GenerateRequest{
				Text: "demo",
			},
			wantErr: true,
		},
		{
			name: "too many images",
			req: &GenerateRequest{
				ImageURLs: []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11"},
				Platforms: []string{"amazon"},
			},
			wantErr: true,
		},
		{
			name: "shein studio gallery ratio mismatch",
			req: &GenerateRequest{
				Text:      "demo",
				Platforms: []string{"shein"},
				Options: &GenerateOptions{
					SheinStudio: &SheinStudioOptions{
						SourceDesignWidth:  1400,
						SourceDesignHeight: 1000,
					},
					SDS: &SDSSyncOptions{
						PrintableWidth:  1000,
						PrintableHeight: 1000,
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRequest(tt.req)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestListTasksFiltersSheinWorkflowStatusBeforePagination(t *testing.T) {
	t.Parallel()

	now := time.Now()
	repo := &stubTaskListRepo{
		tasks: []Task{
			makeTaskListFixture("task-1", now.Add(-time.Minute), SheinWorkflowStatusPendingConfirmation, ""),
			makeTaskListFixture("task-2", now.Add(-2*time.Minute), SheinWorkflowStatusDraftSaved, ""),
			makeTaskListFixture("task-3", now.Add(-3*time.Minute), SheinWorkflowStatusPublished, ""),
		},
	}
	svc := &service{repo: repo}

	page, err := svc.ListTasks(context.Background(), &TaskListQuery{
		Page:                1,
		PageSize:            1,
		SheinWorkflowStatus: SheinWorkflowStatusPublished,
	})
	if err != nil {
		t.Fatalf("ListTasks error = %v", err)
	}
	if repo.lastQuery == nil || repo.lastQuery.SheinWorkflowStatus != SheinWorkflowStatusPublished {
		t.Fatalf("repo query = %+v, want shein_workflow_status propagated", repo.lastQuery)
	}
	if page.Total != 1 {
		t.Fatalf("page total = %d, want 1", page.Total)
	}
	if len(page.Items) != 1 || page.Items[0].TaskID != "task-3" {
		t.Fatalf("page items = %+v, want task-3 only", page.Items)
	}
	if page.Taxonomy == nil || len(page.Taxonomy.SheinWorkflowStatuses) == 0 {
		t.Fatalf("page taxonomy = %+v, want workflow descriptors", page.Taxonomy)
	}
}

func TestListTasksFiltersBySheinBlockerKey(t *testing.T) {
	t.Parallel()

	now := time.Now()
	repo := &stubTaskListRepo{
		tasks: []Task{
			makeTaskListFixture("task-final-review", now.Add(-time.Minute), SheinWorkflowStatusPendingConfirmation, "final_review"),
			makeTaskListFixture("task-category", now.Add(-2*time.Minute), SheinWorkflowStatusPendingConfirmation, "category"),
			makeTaskListFixture("task-published", now.Add(-3*time.Minute), SheinWorkflowStatusPublished, ""),
		},
	}
	svc := &service{repo: repo}

	page, err := svc.ListTasks(context.Background(), &TaskListQuery{
		Page:            1,
		PageSize:        10,
		SheinBlockerKey: "final_review",
	})
	if err != nil {
		t.Fatalf("ListTasks error = %v", err)
	}
	if repo.lastQuery == nil || repo.lastQuery.SheinBlockerKey != "final_review" {
		t.Fatalf("repo query = %+v, want shein_blocker_key propagated", repo.lastQuery)
	}
	if page.Total != 1 {
		t.Fatalf("page total = %d, want 1", page.Total)
	}
	if len(page.Items) != 1 || page.Items[0].TaskID != "task-final-review" {
		t.Fatalf("page items = %+v, want task-final-review only", page.Items)
	}
}

func TestListTasksFiltersBySheinWorkQueue(t *testing.T) {
	t.Parallel()

	now := time.Now()
	repo := &stubTaskListRepo{
		tasks: []Task{
			makeTaskListFixture("task-submit-ready", now.Add(-time.Minute), SheinWorkflowStatusPendingConfirmation, "ready"),
			makeTaskListFixture("task-review", now.Add(-2*time.Minute), SheinWorkflowStatusPendingConfirmation, "warning"),
			makeTaskListFixture("task-repair", now.Add(-3*time.Minute), SheinWorkflowStatusPendingConfirmation, "final_review"),
		},
	}
	svc := &service{repo: repo}

	page, err := svc.ListTasks(context.Background(), &TaskListQuery{
		Page:           1,
		PageSize:       10,
		SheinWorkQueue: SheinWorkQueueSubmitReady,
	})
	if err != nil {
		t.Fatalf("ListTasks error = %v", err)
	}
	if repo.lastQuery == nil || repo.lastQuery.SheinWorkQueue != SheinWorkQueueSubmitReady {
		t.Fatalf("repo query = %+v, want shein_work_queue propagated", repo.lastQuery)
	}
	if page.Total != 1 {
		t.Fatalf("page total = %d, want 1", page.Total)
	}
	if len(page.Items) != 1 || page.Items[0].TaskID != "task-submit-ready" {
		t.Fatalf("page items = %+v, want task-submit-ready only", page.Items)
	}
}

func TestListTasksFiltersBySheinWarningKey(t *testing.T) {
	t.Parallel()

	now := time.Now()
	repo := &stubTaskListRepo{
		tasks: []Task{
			makeTaskListFixture("task-review", now.Add(-time.Minute), SheinWorkflowStatusPendingConfirmation, "warning"),
			makeTaskListFixture("task-ready", now.Add(-2*time.Minute), SheinWorkflowStatusPendingConfirmation, "ready"),
			makeTaskListFixture("task-repair", now.Add(-3*time.Minute), SheinWorkflowStatusPendingConfirmation, "final_review"),
		},
	}
	svc := &service{repo: repo}

	page, err := svc.ListTasks(context.Background(), &TaskListQuery{
		Page:            1,
		PageSize:        10,
		SheinWarningKey: "manual_notes",
	})
	if err != nil {
		t.Fatalf("ListTasks error = %v", err)
	}
	if repo.lastQuery == nil || repo.lastQuery.SheinWarningKey != "manual_notes" {
		t.Fatalf("repo query = %+v, want shein_warning_key propagated", repo.lastQuery)
	}
	if page.Total != 1 {
		t.Fatalf("page total = %d, want 1", page.Total)
	}
	if len(page.Items) != 1 || page.Items[0].TaskID != "task-review" {
		t.Fatalf("page items = %+v, want task-review only", page.Items)
	}
}

func TestListTasksFiltersBySheinActionQueue(t *testing.T) {
	t.Parallel()

	now := time.Now()
	repo := &stubTaskListRepo{
		tasks: []Task{
			makeTaskListFixture("task-classification", now.Add(-time.Minute), SheinWorkflowStatusPendingConfirmation, "category"),
			makeTaskListFixture("task-review", now.Add(-2*time.Minute), SheinWorkflowStatusPendingConfirmation, "warning"),
			makeTaskListFixture("task-ready", now.Add(-3*time.Minute), SheinWorkflowStatusPendingConfirmation, "ready"),
		},
	}
	svc := &service{repo: repo}

	page, err := svc.ListTasks(context.Background(), &TaskListQuery{
		Page:             1,
		PageSize:         10,
		SheinActionQueue: SheinActionQueueClassification,
		IncludeSummary:   true,
	})
	if err != nil {
		t.Fatalf("ListTasks error = %v", err)
	}
	if repo.lastQuery == nil || repo.lastQuery.SheinActionQueue != SheinActionQueueClassification {
		t.Fatalf("repo query = %+v, want shein_action_queue propagated", repo.lastQuery)
	}
	if page.Total != 1 {
		t.Fatalf("page total = %d, want 1", page.Total)
	}
	if len(page.Items) != 1 || page.Items[0].TaskID != "task-classification" {
		t.Fatalf("page items = %+v, want task-classification only", page.Items)
	}
	if page.Summary == nil || page.Summary.SheinActionQueueCounts[SheinActionQueueClassification] != 1 || page.Summary.SheinActionQueueCounts[SheinActionQueueManualReview] != 1 {
		t.Fatalf("page summary = %+v, want filtered-universe action queue counts", page.Summary)
	}
	if page.Taxonomy == nil || len(page.Taxonomy.SheinActionQueues) == 0 || page.Taxonomy.SheinActionQueues[0].Key == "" {
		t.Fatalf("page taxonomy = %+v, want action queue descriptors", page.Taxonomy)
	}
}

func TestCreateGenerateTaskRunsInlineWithoutSubmitter(t *testing.T) {
	t.Parallel()

	productTask := &productenrich.Task{
		ID: "product-task-inline",
		Request: &productenrich.GenerateRequest{
			Text: "inline product",
		},
	}
	productSvc := &stubWorkflowProductService{
		task: productTask,
		product: &productenrich.ProductJSON{
			Title:       "Inline Product",
			Description: "Inline description",
			Category:    []string{"Home"},
			Images:      []string{"https://example.com/source-1.jpg"},
		},
	}
	imageSvc := &stubWorkflowImageService{
		task: &productimage.Task{ID: "image-task-inline"},
		result: &productimage.ImageProcessResult{
			MainImage: &productimage.ImageAsset{URL: "https://cdn.example.com/main.jpg"},
		},
	}
	repo := NewInMemoryRepositoryForTest()
	svc := seedWorkflowServices(seedWorkflowAssets(seedSupportDeps(&service{
		repo: repo,
	}, supportDependencySeed{
		assembler: NewAssemblerWithConfig(AssemblerConfig{AmazonBuilder: stubAmazonDraftBuilder{}}),
	}), nil, newDefaultAssetRecipeResolver(), newDefaultAssetBundleBuilder(), newDefaultAssetGenerationService()), productSvc, imageSvc)

	task, err := svc.CreateGenerateTask(context.Background(), &GenerateRequest{
		Text:      "inline listing kit",
		Platforms: []string{"amazon"},
	})
	if err != nil {
		t.Fatalf("CreateGenerateTask error = %v", err)
	}
	if task.Status == TaskStatusPending {
		t.Fatalf("task status = %q, want non-pending after inline execution", task.Status)
	}
	if task.Result == nil {
		t.Fatalf("task result = nil, want inline workflow result")
	}
}

func TestCreateGenerateTaskPersistsSheinStoreResolutionSnapshot(t *testing.T) {
	t.Parallel()

	repo := NewInMemoryRepositoryForTest()
	storeProfileRepo := newInMemoryStoreProfileRepository()
	svc := &service{
		repo: repo,
		adminDeps: adminDependencies{
			storeProfileRepo: storeProfileRepo,
		},
		submissionDeps: submissionDependencies{
			storeProfileRepo: storeProfileRepo,
		},
		runtime: serviceRuntimeState{
			taskSubmitter: noopTaskSubmitter{},
		},
	}
	ctx := openaiclient.WithIdentity(context.Background(), openaiclient.Identity{TenantID: "606", UserID: "user-f"})

	profile, err := svc.UpsertSheinStoreProfile(ctx, &ListingKitStoreProfile{
		StoreID:       903,
		Enabled:       true,
		Priority:      10,
		Site:          "GB",
		WarehouseCode: "WH-GB-1",
		DefaultStock:  66,
		MatchRules: []ListingKitStoreMatchRule{
			{Kind: "country", Values: []string{"GB"}},
		},
	})
	if err != nil {
		t.Fatalf("UpsertSheinStoreProfile error = %v", err)
	}

	task, err := svc.CreateGenerateTask(ctx, &GenerateRequest{
		Text:         "snapshot demo",
		Platforms:    []string{"shein"},
		Country:      "GB",
		SheinStoreID: 903,
	})
	if err != nil {
		t.Fatalf("CreateGenerateTask error = %v", err)
	}
	if task.SheinStoreResolutionSnapshot == nil {
		t.Fatal("expected shein store resolution snapshot")
	}
	if task.SheinStoreResolutionSnapshot.StoreID != 903 {
		t.Fatalf("snapshot store id = %d, want 903", task.SheinStoreResolutionSnapshot.StoreID)
	}
	if task.SheinStoreResolutionSnapshot.Site != "GB" || task.SheinStoreResolutionSnapshot.WarehouseCode != "WH-GB-1" {
		t.Fatalf("snapshot store context = %+v, want persisted profile fields", task.SheinStoreResolutionSnapshot)
	}
	if task.SheinStoreResolutionSnapshot.Strategy != "manual" {
		t.Fatalf("snapshot strategy = %q, want manual", task.SheinStoreResolutionSnapshot.Strategy)
	}
	if len(task.SheinStoreResolutionSnapshot.MatchedRuleKinds) != 0 {
		t.Fatalf("snapshot matched rules = %+v, want empty", task.SheinStoreResolutionSnapshot.MatchedRuleKinds)
	}
	if task.SheinStoreResolutionSnapshot.MatchedProfileID != profile.ID {
		t.Fatalf("snapshot matched profile id = %d, want %d", task.SheinStoreResolutionSnapshot.MatchedProfileID, profile.ID)
	}
	if task.SheinStoreResolutionSnapshot.ResolvedAt.IsZero() {
		t.Fatalf("snapshot resolved at = %v, want non-zero time", task.SheinStoreResolutionSnapshot.ResolvedAt)
	}

	stored, err := repo.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTask error = %v", err)
	}
	if stored.SheinStoreResolutionSnapshot == nil || stored.SheinStoreResolutionSnapshot.StoreID != 903 {
		t.Fatalf("stored snapshot = %+v, want persisted snapshot", stored.SheinStoreResolutionSnapshot)
	}
}

func TestCreateGenerateTaskDoesNotInferSheinStoreResolutionSnapshotFromRoutingRules(t *testing.T) {
	t.Parallel()

	repo := NewInMemoryRepositoryForTest()
	storeProfileRepo := newInMemoryStoreProfileRepository()
	svc := &service{
		repo: repo,
		adminDeps: adminDependencies{
			storeProfileRepo: storeProfileRepo,
		},
		submissionDeps: submissionDependencies{
			storeProfileRepo: storeProfileRepo,
		},
		runtime: serviceRuntimeState{
			taskSubmitter: noopTaskSubmitter{},
		},
	}
	ctx := openaiclient.WithIdentity(context.Background(), openaiclient.Identity{TenantID: "445", UserID: "user-routing"})

	if _, err := svc.UpsertSheinStoreProfile(ctx, &ListingKitStoreProfile{
		StoreID:       903,
		Enabled:       true,
		Priority:      10,
		Site:          "GB",
		WarehouseCode: "WH-GB-1",
		DefaultStock:  66,
		MatchRules: []ListingKitStoreMatchRule{
			{Kind: "country", Values: []string{"GB"}},
		},
	}); err != nil {
		t.Fatalf("UpsertSheinStoreProfile error = %v", err)
	}
	task, err := svc.CreateGenerateTask(ctx, &GenerateRequest{
		Text:      "snapshot demo",
		Platforms: []string{"shein"},
		Country:   "GB",
	})
	if err != nil {
		t.Fatalf("CreateGenerateTask error = %v", err)
	}
	if task.SheinStoreResolutionSnapshot != nil {
		t.Fatalf("snapshot = %+v, want nil without explicit shein_store_id", task.SheinStoreResolutionSnapshot)
	}
}

func TestCreateGenerateTaskStartsStandardProductTemporalWhenEnabled(t *testing.T) {
	t.Parallel()

	repo := NewInMemoryRepositoryForTest()
	standardClient := &stubStandardProductWorkflowClient{}
	svc := &service{
		repo: repo,
		runtime: serviceRuntimeState{
			taskSubmitter:                  noopTaskSubmitter{},
			standardProductWorkflowClient:  standardClient,
			standardProductWorkflowEnabled: true,
		},
	}

	task, err := svc.CreateGenerateTask(context.Background(), &GenerateRequest{
		Text:      "temporal standard task",
		Platforms: []string{"amazon"},
	})
	if err != nil {
		t.Fatalf("CreateGenerateTask error = %v", err)
	}
	if len(standardClient.calls) != 1 || standardClient.calls[0].TaskID != task.ID {
		t.Fatalf("standard temporal calls = %+v, want single call for %s", standardClient.calls, task.ID)
	}
	if task.Status != TaskStatusPending {
		t.Fatalf("task status = %q, want pending while temporal workflow runs", task.Status)
	}
	if task.Result != nil {
		t.Fatalf("task result = %+v, want nil before temporal processing", task.Result)
	}
}

func TestCreateGenerateTaskRetriesQueueFullAsynchronously(t *testing.T) {
	t.Parallel()

	repo := NewInMemoryRepositoryForTest()
	submitter := &retryQueueFullTaskSubmitter{
		results: []error{worker.ErrQueueFull, nil},
		calls:   make(chan int, 2),
	}
	svc := &service{
		repo: repo,
		runtime: serviceRuntimeState{
			taskSubmitter: submitter,
		},
	}

	task, err := svc.CreateGenerateTask(context.Background(), &GenerateRequest{
		Text:      "async retry task",
		Platforms: []string{"amazon"},
	})
	if err != nil {
		t.Fatalf("CreateGenerateTask error = %v, want nil when queue is temporarily full", err)
	}
	if task.Status != TaskStatusPending {
		t.Fatalf("task status = %q, want pending while waiting for async enqueue retry", task.Status)
	}

	select {
	case <-submitter.calls:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for initial submit attempt")
	}

	select {
	case <-submitter.calls:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for async enqueue retry")
	}

	stored, getErr := repo.GetTask(context.Background(), task.ID)
	if getErr != nil {
		t.Fatalf("GetTask error = %v", getErr)
	}
	if stored.Status != TaskStatusPending {
		t.Fatalf("stored task status = %q, want pending after async retry scheduling", stored.Status)
	}
	if stored.Error != "" {
		t.Fatalf("stored task error = %q, want empty because queue-full should not fail the task", stored.Error)
	}
}

func TestCreateGenerateTaskKeepsTaskPendingWhileQueueRemainsFull(t *testing.T) {
	t.Parallel()

	repo := NewInMemoryRepositoryForTest()
	submitter := &retryQueueFullTaskSubmitter{
		results: []error{worker.ErrQueueFull},
		calls:   make(chan int, 8),
	}
	svc := &service{
		repo: repo,
		runtime: serviceRuntimeState{
			taskSubmitter: submitter,
		},
	}

	task, err := svc.CreateGenerateTask(context.Background(), &GenerateRequest{
		Text:      "persistent queue full task",
		Platforms: []string{"amazon"},
	})
	if err != nil {
		t.Fatalf("CreateGenerateTask error = %v, want nil while queue stays full", err)
	}

	for i := 0; i < 3; i++ {
		select {
		case <-submitter.calls:
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for submit attempt %d", i+1)
		}
	}

	stored, getErr := repo.GetTask(context.Background(), task.ID)
	if getErr != nil {
		t.Fatalf("GetTask error = %v", getErr)
	}
	if stored.Status != TaskStatusPending {
		t.Fatalf("stored task status = %q, want pending while async retry continues", stored.Status)
	}
	if stored.Error != "" {
		t.Fatalf("stored task error = %q, want empty while queue remains full", stored.Error)
	}
}

type stubTaskListRepo struct {
	tasks      []Task
	lastQuery  *TaskListQuery
	listErr    error
	saveResult *ListingKitResult
}

func (r *stubTaskListRepo) CreateTask(context.Context, *Task) error { return nil }
func (r *stubTaskListRepo) GetTask(context.Context, string) (*Task, error) {
	return nil, ErrTaskNotFound
}
func (r *stubTaskListRepo) ListTasks(_ context.Context, query *TaskListQuery) ([]Task, int64, error) {
	if query != nil {
		copied := *query
		r.lastQuery = &copied
	}
	if r.listErr != nil {
		return nil, 0, r.listErr
	}
	filtered := make([]Task, 0, len(r.tasks))
	for _, task := range r.tasks {
		if query != nil && query.SheinWorkflowStatus != "" {
			if buildTaskListItem(&task).SheinWorkflowStatus != query.SheinWorkflowStatus {
				continue
			}
		}
		if query != nil && query.SheinBlockerKey != "" {
			matched := false
			for _, key := range buildTaskListItem(&task).SheinBlockingKeys {
				if key == query.SheinBlockerKey {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		if query != nil && query.SheinWarningKey != "" {
			matched := false
			for _, key := range buildTaskListItem(&task).SheinWarningKeys {
				if key == query.SheinWarningKey {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		if query != nil && query.SheinWorkQueue != "" {
			if buildTaskListItem(&task).SheinWorkQueue != query.SheinWorkQueue {
				continue
			}
		}
		if query != nil && query.SheinActionQueue != "" {
			if buildTaskListItem(&task).SheinActionQueue != query.SheinActionQueue {
				continue
			}
		}
		filtered = append(filtered, task)
	}
	total := int64(len(filtered))
	if query == nil {
		return filtered, total, nil
	}
	start := (query.Page - 1) * query.PageSize
	if start >= len(filtered) {
		return []Task{}, total, nil
	}
	end := start + query.PageSize
	if end > len(filtered) {
		end = len(filtered)
	}
	return filtered[start:end], total, nil
}
func (r *stubTaskListRepo) MarkProcessing(context.Context, string) error { return nil }
func (r *stubTaskListRepo) MarkCompleted(context.Context, string, *ListingKitResult) error {
	return nil
}
func (r *stubTaskListRepo) MarkNeedsReview(context.Context, string, *ListingKitResult, string) error {
	return nil
}
func (r *stubTaskListRepo) MarkFailed(context.Context, string, string) error { return nil }
func (r *stubTaskListRepo) MarkBlockedRetryable(context.Context, string, *RetryableBlock, string) error {
	return nil
}
func (r *stubTaskListRepo) ListRecoverableTasks(context.Context, *RecoverableTaskQuery) ([]Task, error) {
	return []Task{}, nil
}
func (r *stubTaskListRepo) RecoverBlockedTaskNow(context.Context, string, time.Time) error {
	return nil
}
func (r *stubTaskListRepo) BulkRecoverBlockedTasks(context.Context, *RecoverBlockedTasksQuery) (int64, error) {
	return 0, nil
}
func (r *stubTaskListRepo) PrepareRetry(context.Context, string) error        { return nil }
func (r *stubTaskListRepo) IncrementRetryCount(context.Context, string) error { return nil }
func (r *stubTaskListRepo) SaveTaskResult(_ context.Context, _ string, result *ListingKitResult) error {
	r.saveResult = result
	return nil
}

func (r *stubTaskListRepo) ListTaskSummaryTasks(_ context.Context, query *TaskListQuery) ([]Task, error) {
	filtered := make([]Task, 0, len(r.tasks))
	for _, task := range r.tasks {
		if TaskMatchesListQuery(&task, query) {
			filtered = append(filtered, task)
		}
	}
	return filtered, nil
}

func makeTaskListFixture(id string, createdAt time.Time, workflowStatus string, blockerKey string) Task {
	colorValueID := 271
	task := Task{
		ID:        id,
		Status:    TaskStatusCompleted,
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
		Request: &GenerateRequest{
			Text:      id,
			Platforms: []string{"shein"},
		},
		Result: &ListingKitResult{
			TaskID: id,
			Shein: &SheinPackage{
				RequestDraft: &SheinRequestDraft{
					ImageInfo: &sheinpub.ImageDraft{
						MainImage: "https://cdn.example.com/main.png",
						Gallery:   []string{"https://cdn.example.com/gallery.png"},
					},
					SKCList: []SheinSKCRequestDraft{{
						SupplierCode: "SKC-1",
						SaleAttribute: &SheinResolvedSaleAttribute{
							Name:             "Color",
							AttributeID:      27,
							AttributeValueID: &colorValueID,
						},
						ImageInfo: &sheinpub.ImageDraft{
							MainImage: "https://cdn.example.com/skc-main.png",
						},
						SKUList: []sheinpub.SKUDraft{{
							SupplierSKU: "SKU-1",
							BasePrice:   "19.99",
							SitePriceList: []sheinpub.SitePrice{{
								SubSite:   "US",
								BasePrice: "19.99",
								Currency:  "USD",
							}},
						}},
					}},
				},
				PreviewProduct: &sheinproduct.Product{},
				SkcList: []SheinSKCPackage{{
					SupplierCode: "SKC-1",
					Attributes: map[string]string{
						"Color": "Black",
					},
					SKUs: []PlatformVariant{{
						SKU: "SKU-1",
					}},
				}},
				CategoryResolution: &SheinCategoryResolution{
					Status:     "resolved",
					CategoryID: 3001,
				},
				CategoryID:     3001,
				CategoryIDList: []int{1, 2, 3001},
				ProductTypeID:  intPtr(901),
				AttributeResolution: &SheinAttributeResolution{
					Status:        "resolved",
					ResolvedCount: 1,
				},
				ResolvedAttributes: []SheinResolvedAttribute{{
					Name:        "Material",
					AttributeID: 160,
				}},
				SaleAttributeResolution: &SheinSaleAttributeResolution{
					Status:             "resolved",
					PrimaryAttributeID: 27,
				},
			},
		},
	}
	switch blockerKey {
	case "final_review":
		task.Result.Shein.FinalDraft = &sheinpub.FinalDraft{Confirmed: false}
	case "category":
		task.Result.Shein.CategoryResolution.Status = "partial"
		task.Result.Shein.CategoryID = 0
		task.Result.Shein.ProductTypeID = nil
		task.Result.Shein.CategoryIDList = nil
	case "warning":
		task.Result.Shein.FinalDraft = &sheinpub.FinalDraft{
			Confirmed:    true,
			MainImageURL: "https://cdn.example.com/main.png",
			ImageRoleOverrides: map[string]string{
				"https://cdn.example.com/skc-main.png": "swatch",
			},
		}
		task.Result.Shein.ReviewNotes = []string{"需要人工确认吊牌文案"}
	case "ready":
		task.Result.Shein.FinalDraft = &sheinpub.FinalDraft{
			Confirmed:    true,
			MainImageURL: "https://cdn.example.com/main.png",
			ImageRoleOverrides: map[string]string{
				"https://cdn.example.com/skc-main.png": "swatch",
			},
		}
	default:
		task.Result.Shein.FinalDraft = &sheinpub.FinalDraft{
			Confirmed:    true,
			MainImageURL: "https://cdn.example.com/main.png",
			ImageRoleOverrides: map[string]string{
				"https://cdn.example.com/skc-main.png": "swatch",
			},
		}
	}
	switch workflowStatus {
	case SheinWorkflowStatusPublished:
		task.Result.Shein.SubmissionEvents = []sheinpub.SubmissionEvent{{
			Action: "publish",
			Status: "success",
		}}
	case SheinWorkflowStatusDraftSaved:
		task.Result.Shein.SubmissionEvents = []sheinpub.SubmissionEvent{{
			Action: "save_draft",
			Status: "success",
		}}
	}
	return task
}

func NewInMemoryRepositoryForTest() Repository {
	return &stubInlineTaskRepo{tasks: map[string]*Task{}}
}

type stubInlineTaskRepo struct {
	tasks             map[string]*Task
	sdsBaselineCache  map[string]*SDSBaselineCacheEntry
	sdsBaselineGetErr error
}

func (r *stubInlineTaskRepo) CreateTask(_ context.Context, task *Task) error {
	copied := *task
	r.tasks[task.ID] = &copied
	return nil
}

func (r *stubInlineTaskRepo) GetTask(_ context.Context, taskID string) (*Task, error) {
	task, ok := r.tasks[taskID]
	if !ok {
		return nil, ErrTaskNotFound
	}
	copied := *task
	return &copied, nil
}

func (r *stubInlineTaskRepo) ListTasks(context.Context, *TaskListQuery) ([]Task, int64, error) {
	return nil, 0, nil
}

func (r *stubInlineTaskRepo) MarkProcessing(_ context.Context, taskID string) error {
	task := r.tasks[taskID]
	task.Status = TaskStatusProcessing
	task.UpdatedAt = time.Now()
	return nil
}

func (r *stubInlineTaskRepo) MarkCompleted(_ context.Context, taskID string, result *ListingKitResult) error {
	task := r.tasks[taskID]
	task.Result = result
	task.Status = TaskStatusCompleted
	task.Error = ""
	task.UpdatedAt = time.Now()
	return nil
}

func (r *stubInlineTaskRepo) MarkNeedsReview(_ context.Context, taskID string, result *ListingKitResult, reason string) error {
	task := r.tasks[taskID]
	task.Result = result
	task.Status = TaskStatusNeedsReview
	task.Error = reason
	task.UpdatedAt = time.Now()
	return nil
}

func (r *stubInlineTaskRepo) MarkFailed(_ context.Context, taskID string, errorMsg string) error {
	task := r.tasks[taskID]
	task.Status = TaskStatusFailed
	task.Error = errorMsg
	task.UpdatedAt = time.Now()
	return nil
}

func (r *stubInlineTaskRepo) MarkBlockedRetryable(_ context.Context, taskID string, block *RetryableBlock, errorMsg string) error {
	task := r.tasks[taskID]
	task.Status = TaskStatusBlockedRetryable
	task.RetryableBlock = block
	task.Error = errorMsg
	task.UpdatedAt = time.Now()
	return nil
}

func (r *stubInlineTaskRepo) ListRecoverableTasks(context.Context, *RecoverableTaskQuery) ([]Task, error) {
	return []Task{}, nil
}

func (r *stubInlineTaskRepo) RecoverBlockedTaskNow(_ context.Context, taskID string, recoveredAt time.Time) error {
	task := r.tasks[taskID]
	task.Status = TaskStatusPending
	task.RetryableBlock = nil
	task.Error = ""
	task.UpdatedAt = recoveredAt
	return nil
}

func (r *stubInlineTaskRepo) BulkRecoverBlockedTasks(context.Context, *RecoverBlockedTasksQuery) (int64, error) {
	return 0, nil
}

func (r *stubInlineTaskRepo) PrepareRetry(context.Context, string) error        { return nil }
func (r *stubInlineTaskRepo) IncrementRetryCount(context.Context, string) error { return nil }
func (r *stubInlineTaskRepo) SaveTaskResult(_ context.Context, taskID string, result *ListingKitResult) error {
	task := r.tasks[taskID]
	task.Result = result
	task.UpdatedAt = time.Now()
	return nil
}

func (r *stubInlineTaskRepo) GetCanonicalProductCache(context.Context, string) (*canonical.Product, error) {
	return nil, nil
}

func (r *stubInlineTaskRepo) SaveCanonicalProductCache(context.Context, string, *canonical.Product, string) error {
	return nil
}

func (r *stubInlineTaskRepo) GetSDSBaselineCache(ctx context.Context, tenantID, baselineKey string) (*SDSBaselineCacheEntry, error) {
	if r.sdsBaselineGetErr != nil {
		return nil, r.sdsBaselineGetErr
	}
	resolvedTenantID, logicalKey, storageKey, err := ResolveSDSBaselineCacheScope(ctx, tenantID, baselineKey)
	if err != nil {
		return nil, err
	}
	if storageKey == "" || r.sdsBaselineCache == nil {
		return nil, nil
	}
	entry, err := r.sdsBaselineCache[storageKey].Clone()
	if err != nil || entry == nil {
		return entry, err
	}
	entry.TenantID = resolvedTenantID
	entry.BaselineKey = logicalKey
	return entry, nil
}

func (r *stubInlineTaskRepo) SaveSDSBaselineCache(ctx context.Context, entry *SDSBaselineCacheEntry) error {
	if entry == nil {
		return nil
	}
	resolvedTenantID, logicalKey, storageKey, err := ResolveSDSBaselineCacheScope(ctx, entry.TenantID, entry.BaselineKey)
	if err != nil {
		return err
	}
	cloned, err := entry.Clone()
	if err != nil {
		return err
	}
	cloned.TenantID = resolvedTenantID
	cloned.BaselineKey = logicalKey
	if r.sdsBaselineCache == nil {
		r.sdsBaselineCache = map[string]*SDSBaselineCacheEntry{}
	}
	r.sdsBaselineCache[storageKey] = cloned
	return nil
}

func (r *stubInlineTaskRepo) MutateTaskResult(_ context.Context, taskID string, mutate TaskResultMutation) (*Task, error) {
	task, ok := r.tasks[taskID]
	if !ok {
		return nil, ErrTaskNotFound
	}
	if mutate != nil {
		if err := mutate(task); err != nil {
			return nil, err
		}
	}
	task.UpdatedAt = time.Now()
	copied := *task
	return &copied, nil
}

type retryQueueFullTaskSubmitter struct {
	mu      sync.Mutex
	results []error
	calls   chan int
	count   int
}

func (s *retryQueueFullTaskSubmitter) Submit(string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.count++
	if s.calls != nil {
		s.calls <- s.count
	}
	if len(s.results) == 0 {
		return nil
	}
	result := s.results[0]
	if len(s.results) > 1 {
		s.results = s.results[1:]
	}
	return result
}

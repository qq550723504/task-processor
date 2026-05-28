package store_test

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/store"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func TestTaskRepositoryListTasksFiltersBeforePaginationAndPreservesOrder(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&listingkit.Task{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := store.NewTaskRepository(db)
	ctx := listingkit.WithTenantID(context.Background(), "tenant-a")
	base := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)

	tasks := []*listingkit.Task{
		makeTaskRepoFixture("task-newest-unmatched", base.Add(3*time.Minute), []string{"amazon"}, "", ""),
		makeTaskRepoFixture("task-middle-match", base.Add(2*time.Minute), []string{"shein"}, "published", ""),
		makeTaskRepoFixture("task-oldest-match", base.Add(1*time.Minute), []string{"shein"}, "published", ""),
	}
	for _, task := range tasks {
		if err := repo.CreateTask(ctx, task); err != nil {
			t.Fatalf("create task %s: %v", task.ID, err)
		}
	}

	page1, total, err := repo.ListTasks(ctx, &listingkit.TaskListQuery{
		Platform:            "shein",
		SheinWorkflowStatus: "published",
		Page:                1,
		PageSize:            1,
	})
	if err != nil {
		t.Fatalf("list page1: %v", err)
	}
	if total != 2 {
		t.Fatalf("page1 total = %d, want 2", total)
	}
	if len(page1) != 1 || page1[0].ID != "task-middle-match" {
		t.Fatalf("page1 tasks = %+v, want task-middle-match", page1)
	}

	page2, total, err := repo.ListTasks(ctx, &listingkit.TaskListQuery{
		Platform:            "shein",
		SheinWorkflowStatus: "published",
		Page:                2,
		PageSize:            1,
	})
	if err != nil {
		t.Fatalf("list page2: %v", err)
	}
	if total != 2 {
		t.Fatalf("page2 total = %d, want 2", total)
	}
	if len(page2) != 1 || page2[0].ID != "task-oldest-match" {
		t.Fatalf("page2 tasks = %+v, want task-oldest-match", page2)
	}
}

func TestTaskRepositoryListTasksFiltersBySheinBlockerKey(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&listingkit.Task{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := store.NewTaskRepository(db)
	ctx := listingkit.WithTenantID(context.Background(), "tenant-a")
	base := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)

	tasks := []*listingkit.Task{
		makeTaskRepoFixture("task-final-review", base.Add(3*time.Minute), []string{"shein"}, "pending_confirmation", "final_review"),
		makeTaskRepoFixture("task-category", base.Add(2*time.Minute), []string{"shein"}, "pending_confirmation", "category"),
		makeTaskRepoFixture("task-published", base.Add(1*time.Minute), []string{"shein"}, "published", ""),
	}
	for _, task := range tasks {
		if err := repo.CreateTask(ctx, task); err != nil {
			t.Fatalf("create task %s: %v", task.ID, err)
		}
	}

	items, total, err := repo.ListTasks(ctx, &listingkit.TaskListQuery{
		SheinBlockerKey: "final_review",
		Page:            1,
		PageSize:        10,
	})
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	if len(items) != 1 || items[0].ID != "task-final-review" {
		t.Fatalf("items = %+v, want task-final-review", items)
	}
}

func TestTaskRepositoryListTasksFiltersByPodPlatformBlocker(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&listingkit.Task{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := store.NewTaskRepository(db)
	ctx := listingkit.WithTenantID(context.Background(), "tenant-a")
	base := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)

	blocked := makeTaskRepoFixture("task-pod-blocked", base.Add(2*time.Minute), []string{"shein"}, "pending_confirmation", "ready")
	blocked.Result.PodExecution = &listingkit.PodExecutionSummary{
		Provider:       "sds",
		DependencyMode: "required",
		Status:         "failed_blocking",
		FailureReason:  "mockup sync timeout",
	}
	ready := makeTaskRepoFixture("task-pod-ready", base.Add(1*time.Minute), []string{"shein"}, "pending_confirmation", "ready")
	ready.Result.PodExecution = &listingkit.PodExecutionSummary{
		Provider:       "sds",
		DependencyMode: "required",
		Status:         "succeeded",
	}
	for _, task := range []*listingkit.Task{blocked, ready} {
		if err := repo.CreateTask(ctx, task); err != nil {
			t.Fatalf("create task %s: %v", task.ID, err)
		}
	}

	items, total, err := repo.ListTasks(ctx, &listingkit.TaskListQuery{
		SheinBlockerKey: "pod_platform",
		Page:            1,
		PageSize:        10,
	})
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	if len(items) != 1 || items[0].ID != "task-pod-blocked" {
		t.Fatalf("items = %+v, want task-pod-blocked", items)
	}
}

func TestTaskRepositoryListTasksFiltersBySheinWorkQueue(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&listingkit.Task{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := store.NewTaskRepository(db)
	ctx := listingkit.WithTenantID(context.Background(), "tenant-a")
	base := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)

	pending := &listingkit.Task{
		ID:        "task-generation",
		TenantID:  "tenant-a",
		Status:    listingkit.TaskStatusPending,
		Request:   &listingkit.GenerateRequest{Platforms: []string{"shein"}},
		CreatedAt: base.Add(3 * time.Minute),
		UpdatedAt: base.Add(3 * time.Minute),
	}
	ready := makeTaskRepoFixture("task-submit-ready", base.Add(2*time.Minute), []string{"shein"}, "pending_confirmation", "ready")
	repair := makeTaskRepoFixture("task-repair", base.Add(1*time.Minute), []string{"shein"}, "pending_confirmation", "final_review")

	for _, task := range []*listingkit.Task{pending, ready, repair} {
		if err := repo.CreateTask(ctx, task); err != nil {
			t.Fatalf("create task %s: %v", task.ID, err)
		}
	}

	items, total, err := repo.ListTasks(ctx, &listingkit.TaskListQuery{
		SheinWorkQueue: "generation_queue",
		Page:           1,
		PageSize:       10,
	})
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	if len(items) != 1 || items[0].ID != "task-generation" {
		t.Fatalf("items = %+v, want task-generation", items)
	}
}

func TestTaskRepositoryListTasksFiltersBySheinWarningKey(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&listingkit.Task{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := store.NewTaskRepository(db)
	ctx := listingkit.WithTenantID(context.Background(), "tenant-a")
	base := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)

	tasks := []*listingkit.Task{
		makeTaskRepoFixture("task-review", base.Add(3*time.Minute), []string{"shein"}, "pending_confirmation", "warning"),
		makeTaskRepoFixture("task-ready", base.Add(2*time.Minute), []string{"shein"}, "pending_confirmation", "ready"),
		makeTaskRepoFixture("task-repair", base.Add(1*time.Minute), []string{"shein"}, "pending_confirmation", "final_review"),
	}
	for _, task := range tasks {
		if err := repo.CreateTask(ctx, task); err != nil {
			t.Fatalf("create task %s: %v", task.ID, err)
		}
	}

	items, total, err := repo.ListTasks(ctx, &listingkit.TaskListQuery{
		SheinWarningKey: "manual_notes",
		Page:            1,
		PageSize:        10,
	})
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	if len(items) != 1 || items[0].ID != "task-review" {
		t.Fatalf("items = %+v, want task-review", items)
	}
}

func TestTaskRepositoryListTasksFiltersBySheinActionQueue(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&listingkit.Task{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := store.NewTaskRepository(db)
	ctx := listingkit.WithTenantID(context.Background(), "tenant-a")
	base := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)

	tasks := []*listingkit.Task{
		makeTaskRepoFixture("task-classification", base.Add(3*time.Minute), []string{"shein"}, "pending_confirmation", "category"),
		makeTaskRepoFixture("task-review", base.Add(2*time.Minute), []string{"shein"}, "pending_confirmation", "warning"),
		makeTaskRepoFixture("task-ready", base.Add(1*time.Minute), []string{"shein"}, "pending_confirmation", "ready"),
	}
	for _, task := range tasks {
		if err := repo.CreateTask(ctx, task); err != nil {
			t.Fatalf("create task %s: %v", task.ID, err)
		}
	}

	items, total, err := repo.ListTasks(ctx, &listingkit.TaskListQuery{
		SheinActionQueue: "classification_queue",
		Page:             1,
		PageSize:         10,
	})
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	if len(items) != 1 || items[0].ID != "task-classification" {
		t.Fatalf("items = %+v, want task-classification", items)
	}
}

func TestTaskRepositoryListTaskSummaryTasksReturnsFilteredUniverse(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&listingkit.Task{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := store.NewTaskRepository(db)
	summaryRepo, ok := repo.(interface {
		ListTaskSummaryTasks(ctx context.Context, query *listingkit.TaskListQuery) ([]listingkit.Task, error)
	})
	if !ok {
		t.Fatal("task repo does not expose ListTaskSummaryTasks")
	}
	ctx := listingkit.WithTenantID(context.Background(), "tenant-a")
	base := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)

	tasks := []*listingkit.Task{
		makeTaskRepoFixture("task-classification", base.Add(3*time.Minute), []string{"shein"}, "pending_confirmation", "category"),
		makeTaskRepoFixture("task-review", base.Add(2*time.Minute), []string{"shein"}, "pending_confirmation", "warning"),
		makeTaskRepoFixture("task-ready", base.Add(1*time.Minute), []string{"shein"}, "pending_confirmation", "ready"),
	}
	for _, task := range tasks {
		if err := repo.CreateTask(ctx, task); err != nil {
			t.Fatalf("create task %s: %v", task.ID, err)
		}
	}

	items, err := summaryRepo.ListTaskSummaryTasks(ctx, &listingkit.TaskListQuery{Platform: "shein"})
	if err != nil {
		t.Fatalf("list summary tasks: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("items = %+v, want 3 filtered tasks", items)
	}
	if items[0].ID != "task-classification" || items[1].ID != "task-review" || items[2].ID != "task-ready" {
		t.Fatalf("items order = %+v, want created_at desc filtered universe", items)
	}
}

func TestTaskRepositoryOwnerScopeFiltersTasksByUser(t *testing.T) {
	t.Cleanup(listingkit.SetOwnerScopeRequiredForTesting(true))

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&listingkit.Task{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := store.NewTaskRepository(db)
	baseCtx := listingkit.WithTenantID(context.Background(), "tenant-a")
	adminCtx := openaiclient.WithIdentity(baseCtx, openaiclient.Identity{TenantID: "tenant-a", UserID: "user-a"})

	taskA := makeTaskRepoFixture("task-user-a", time.Now().UTC(), []string{"shein"}, "published", "")
	taskA.UserID = "user-a"
	taskA.Request.UserID = "user-a"
	taskB := makeTaskRepoFixture("task-user-b", time.Now().UTC().Add(-time.Minute), []string{"shein"}, "published", "")
	taskB.UserID = "user-b"
	taskB.Request.UserID = "user-b"
	for _, task := range []*listingkit.Task{taskA, taskB} {
		if err := repo.CreateTask(baseCtx, task); err != nil {
			t.Fatalf("create task %s: %v", task.ID, err)
		}
	}

	items, total, err := repo.ListTasks(adminCtx, &listingkit.TaskListQuery{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].ID != "task-user-a" {
		t.Fatalf("owner scoped tasks = %+v total=%d, want only task-user-a", items, total)
	}

	if _, err := repo.GetTask(adminCtx, "task-user-b"); err != listingkit.ErrTaskNotFound {
		t.Fatalf("GetTask err = %v, want ErrTaskNotFound", err)
	}
}

func TestTaskRepositoryPlatformAdminBypassesOwnerScope(t *testing.T) {
	t.Cleanup(listingkit.SetOwnerScopeRequiredForTesting(true))

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&listingkit.Task{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := store.NewTaskRepository(db)
	baseCtx := listingkit.WithTenantID(context.Background(), "tenant-a")
	platformAdminCtx := listingkit.WithRequestRoles(
		openaiclient.WithIdentity(baseCtx, openaiclient.Identity{TenantID: "tenant-a", UserID: "platform-admin"}),
		[]string{"platform_admin"},
	)

	taskA := makeTaskRepoFixture("task-user-a", time.Now().UTC(), []string{"shein"}, "published", "")
	taskA.UserID = "user-a"
	taskA.Request.UserID = "user-a"
	taskB := makeTaskRepoFixture("task-user-b", time.Now().UTC().Add(-time.Minute), []string{"shein"}, "published", "")
	taskB.UserID = "user-b"
	taskB.Request.UserID = "user-b"
	for _, task := range []*listingkit.Task{taskA, taskB} {
		if err := repo.CreateTask(baseCtx, task); err != nil {
			t.Fatalf("create task %s: %v", task.ID, err)
		}
	}

	items, total, err := repo.ListTasks(platformAdminCtx, &listingkit.TaskListQuery{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("platform admin items = %+v total=%d, want both tasks", items, total)
	}

	got, err := repo.GetTask(platformAdminCtx, "task-user-b")
	if err != nil {
		t.Fatalf("GetTask err = %v, want nil", err)
	}
	if got.ID != "task-user-b" {
		t.Fatalf("GetTask id = %q, want task-user-b", got.ID)
	}
}

func makeTaskRepoFixture(id string, createdAt time.Time, platforms []string, sheinWorkflow string, blockerKey string) *listingkit.Task {
	task := &listingkit.Task{
		ID:        id,
		TenantID:  "tenant-a",
		Status:    listingkit.TaskStatusCompleted,
		Request:   &listingkit.GenerateRequest{Platforms: append([]string(nil), platforms...)},
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}
	if sheinWorkflow == "" {
		return task
	}
	productTypeID := 901
	task.Result = &listingkit.ListingKitResult{
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
		Shein: &sheinpub.Package{
			RequestDraft: &sheinpub.RequestDraft{
				ImageInfo: &sheinpub.ImageDraft{
					MainImage: "https://cdn.example.com/main.png",
					Gallery:   []string{"https://cdn.example.com/gallery.png"},
				},
				SKCList: []sheinpub.SKCRequestDraft{{
					SupplierCode: "SKC-1",
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
			SkcList: []listingkit.SheinSKCPackage{{
				SupplierCode: "SKC-1",
				SKUs: []listingkit.PlatformVariant{{
					SKU: "SKU-1",
				}},
			}},
			CategoryResolution: &sheinpub.CategoryResolution{
				Status:     "resolved",
				CategoryID: 3001,
			},
			CategoryID:     3001,
			CategoryIDList: []int{1, 2, 3001},
			ProductTypeID:  &productTypeID,
			AttributeResolution: &sheinpub.AttributeResolution{
				Status:        "resolved",
				ResolvedCount: 1,
			},
			ResolvedAttributes: []sheinpub.ResolvedAttribute{{
				Name:        "Material",
				AttributeID: 160,
			}},
			SaleAttributeResolution: &sheinpub.SaleAttributeResolution{
				Status:             "resolved",
				PrimaryAttributeID: 27,
			},
			Submission: &sheinpub.SubmissionReport{
				LastAction: "publish",
				LastStatus: sheinpub.SubmissionStatusSuccess,
				Publish: &sheinpub.SubmissionRecord{
					Action: "publish",
					Status: sheinpub.SubmissionStatusSuccess,
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
		task.Result.Shein.CategoryIDList = nil
		task.Result.Shein.ProductTypeID = nil
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
	if sheinWorkflow == "pending_confirmation" {
		task.Result.Shein.Submission = nil
	}
	return task
}

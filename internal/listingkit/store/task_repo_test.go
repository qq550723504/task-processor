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

func TestTaskRepositoryListTasksAcceptsLegacyKeyedCatalogAttributes(t *testing.T) {
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
	createdAt := time.Date(2026, 7, 5, 10, 59, 26, 0, time.UTC)
	resultJSON := `{
		"task_id": "task-legacy-keyed-attrs",
		"status": "needs_review",
		"standard_product_snapshot": {
			"catalog_product": {
				"title": "Legacy catalog product",
				"attributes": {
					"product_size": {
						"value": "[[{\"content\":\"Size\"},{\"content\":\"Bust\"}]]",
						"trace": {
							"sources": [{"type": "derived", "detail": "SDS product detail repair"}],
							"confidence": 0.99
						}
					}
				}
			}
		},
		"catalog_product": {
			"title": "Legacy catalog product",
			"attributes": {
				"product_size": {
					"value": "[[{\"content\":\"Size\"},{\"content\":\"Bust\"}]]"
				}
			}
		}
	}`
	if err := db.Exec(
		`insert into listing_kit_tasks (id, tenant_id, status, request, result, created_at, updated_at, retry_count) values (?, ?, ?, ?, ?, ?, ?, ?)`,
		"task-legacy-keyed-attrs",
		"tenant-a",
		string(listingkit.TaskStatusNeedsReview),
		`{"platforms":["shein"]}`,
		resultJSON,
		createdAt,
		createdAt,
		0,
	).Error; err != nil {
		t.Fatalf("insert legacy task: %v", err)
	}

	items, total, err := repo.ListTasks(ctx, &listingkit.TaskListQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("items=%d total=%d, want one task", len(items), total)
	}
	attrs := items[0].Result.StandardProductSnapshot.CatalogProduct.Attributes
	if len(attrs) != 1 || attrs[0].Name != "product_size" {
		t.Fatalf("catalog attributes = %+v, want keyed product_size converted", attrs)
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

func TestTaskRepositoryOwnerScopeIncludesLegacyRequestUserIDTasks(t *testing.T) {
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
	userCtx := openaiclient.WithIdentity(baseCtx, openaiclient.Identity{TenantID: "tenant-a", UserID: "legacy-user"})

	legacyTask := makeTaskRepoFixture("task-legacy", time.Now().UTC(), []string{"shein"}, "published", "")
	legacyTask.UserID = ""
	legacyTask.Request.UserID = "legacy-user"
	otherTask := makeTaskRepoFixture("task-other", time.Now().UTC().Add(-time.Minute), []string{"shein"}, "published", "")
	otherTask.UserID = ""
	otherTask.Request.UserID = "someone-else"
	for _, task := range []*listingkit.Task{legacyTask, otherTask} {
		if err := repo.CreateTask(baseCtx, task); err != nil {
			t.Fatalf("create task %s: %v", task.ID, err)
		}
	}

	items, total, err := repo.ListTasks(userCtx, &listingkit.TaskListQuery{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].ID != "task-legacy" {
		t.Fatalf("owner scoped legacy tasks = %+v total=%d, want only task-legacy", items, total)
	}
}

func TestTaskRepositoryOwnerScopeFilteredListIncludesLegacyRequestUserIDTasks(t *testing.T) {
	t.Cleanup(listingkit.SetOwnerScopeRequiredForTesting(true))

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
	baseCtx := listingkit.WithTenantID(context.Background(), "tenant-a")
	userCtx := openaiclient.WithIdentity(baseCtx, openaiclient.Identity{TenantID: "tenant-a", UserID: "legacy-user"})
	createdAt := time.Date(2026, 7, 6, 9, 0, 0, 0, time.UTC)

	visible := makeTaskRepoFixture("task-legacy-visible", createdAt, []string{"shein"}, "published", "")
	visible.UserID = ""
	visible.Request.UserID = "legacy-user"
	wrongPlatform := makeTaskRepoFixture("task-legacy-amazon", createdAt.Add(-time.Minute), []string{"amazon"}, "published", "")
	wrongPlatform.UserID = ""
	wrongPlatform.Request.UserID = "legacy-user"
	otherUser := makeTaskRepoFixture("task-legacy-other-user", createdAt.Add(-2*time.Minute), []string{"shein"}, "published", "")
	otherUser.UserID = ""
	otherUser.Request.UserID = "someone-else"
	for _, task := range []*listingkit.Task{visible, wrongPlatform, otherUser} {
		if err := db.WithContext(baseCtx).Create(task).Error; err != nil {
			t.Fatalf("create raw task %s: %v", task.ID, err)
		}
	}

	query := &listingkit.TaskListQuery{Platform: "shein", Page: 1, PageSize: 10}
	items, total, err := repo.ListTasks(userCtx, query)
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].ID != "task-legacy-visible" {
		t.Fatalf("filtered legacy items = %+v total=%d, want only task-legacy-visible", items, total)
	}

	summaryItems, err := summaryRepo.ListTaskSummaryTasks(userCtx, query)
	if err != nil {
		t.Fatalf("ListTaskSummaryTasks() error = %v", err)
	}
	if len(summaryItems) != 1 || summaryItems[0].ID != "task-legacy-visible" {
		t.Fatalf("filtered legacy summary items = %+v, want only task-legacy-visible", summaryItems)
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

func TestTaskRepositoryListSheinSourceSDSMetadataMatchesSameUserAcrossLegacyTenantMapping(t *testing.T) {
	t.Cleanup(listingkit.SetOwnerScopeRequiredForTesting(true))

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&listingkit.Task{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := store.NewTaskRepository(db)
	source, ok := repo.(interface {
		ListSheinSourceSDSMetadata(ctx context.Context, query *listingkit.SheinSourceSDSMetadataQuery) ([]listingkit.SheinSourceSDSMetadataRecord, error)
	})
	if !ok {
		t.Fatal("task repo does not expose ListSheinSourceSDSMetadata")
	}

	createCtx := listingkit.WithTenantID(context.Background(), "373211199677923496")
	sourceTask := &listingkit.Task{
		ID:       "task-source-sds",
		TenantID: "373211199677923496",
		UserID:   "373211204509761704",
		Status:   listingkit.TaskStatusCompleted,
		Request: &listingkit.GenerateRequest{
			UserID:       "373211204509761704",
			Platforms:    []string{"shein"},
			SheinStoreID: 870,
			Options: &listingkit.GenerateOptions{SDS: &listingkit.SDSSyncOptions{
				ProductName: "方形双层腰包 -（单图多拼可选）",
				ProductSKU:  "",
				Variants: []listingkit.SDSSyncVariantOption{{
					VariantSKU:      "XB0610007001",
					Price:           34.5,
					Color:           "white",
					Size:            "16x23cm",
					MockupImageURLs: []string{"https://cdn.sdspod.com/mockup/waist-bag.jpg"},
				}},
			}},
		},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := repo.CreateTask(createCtx, sourceTask); err != nil {
		t.Fatalf("create source task: %v", err)
	}

	requestCtx := openaiclient.WithIdentity(
		listingkit.WithTenantID(context.Background(), "227"),
		openaiclient.Identity{TenantID: "227", UserID: "373211204509761704"},
	)
	items, err := source.ListSheinSourceSDSMetadata(requestCtx, &listingkit.SheinSourceSDSMetadataQuery{
		StoreID:     870,
		SourceCodes: []string{"XB0610007001"},
	})
	if err != nil {
		t.Fatalf("ListSheinSourceSDSMetadata error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1: %+v", len(items), items)
	}
	if items[0].SourceCode != "XB0610007001" || items[0].Title != "方形双层腰包 -（单图多拼可选）" {
		t.Fatalf("metadata = %+v, want source code and SDS title", items[0])
	}
	if items[0].Price != 34.5 || items[0].VariantLabel != "white / 16x23cm" {
		t.Fatalf("variant metadata = %+v, want price and label", items[0])
	}
	if items[0].ImageURL != "https://cdn.sdspod.com/mockup/waist-bag.jpg" {
		t.Fatalf("image url = %q, want variant mockup image", items[0].ImageURL)
	}
}

func TestTaskRepositoryListSheinSourceSDSMetadataExpandsProductSKUToSiblingVariants(t *testing.T) {
	t.Cleanup(listingkit.SetOwnerScopeRequiredForTesting(false))

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&listingkit.Task{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := store.NewTaskRepository(db)
	source, ok := repo.(interface {
		ListSheinSourceSDSMetadata(ctx context.Context, query *listingkit.SheinSourceSDSMetadataQuery) ([]listingkit.SheinSourceSDSMetadataRecord, error)
	})
	if !ok {
		t.Fatal("task repo does not expose ListSheinSourceSDSMetadata")
	}

	createCtx := listingkit.WithTenantID(context.Background(), "373211199677923496")
	sourceTask := &listingkit.Task{
		ID:       "task-source-sds-sibling-variants",
		TenantID: "373211199677923496",
		UserID:   "373211204509761704",
		Status:   listingkit.TaskStatusCompleted,
		Request: &listingkit.GenerateRequest{
			UserID:       "373211204509761704",
			Platforms:    []string{"shein"},
			SheinStoreID: 870,
			Options: &listingkit.GenerateOptions{SDS: &listingkit.SDSSyncOptions{
				ProductName:  "帆布拉绳收纳袋双面印刷",
				ProductSKU:   "XB0603003001",
				VariantSKU:   "XB0603003001",
				VariantSize:  "12*18cm",
				VariantPrice: 8.81,
				Variants: []listingkit.SDSSyncVariantOption{{
					VariantSKU: "XB0603003002",
					Size:       "20*25cm",
					Price:      9.64,
				}},
			}},
		},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := repo.CreateTask(createCtx, sourceTask); err != nil {
		t.Fatalf("create source task: %v", err)
	}

	items, err := source.ListSheinSourceSDSMetadata(context.Background(), &listingkit.SheinSourceSDSMetadataQuery{
		StoreID:     870,
		SourceCodes: []string{"XB0603003001"},
	})
	if err != nil {
		t.Fatalf("ListSheinSourceSDSMetadata error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2: %+v", len(items), items)
	}
	if items[0].SourceCode != "XB0603003001" || items[0].VariantLabel != "12*18cm" || items[0].Price != 8.81 {
		t.Fatalf("first metadata = %+v, want 12*18cm variant", items[0])
	}
	if items[1].SourceCode != "XB0603003002" || items[1].VariantLabel != "20*25cm" || items[1].Price != 9.64 {
		t.Fatalf("second metadata = %+v, want 20*25cm sibling variant", items[1])
	}
}

func TestTaskRepositoryListSheinSourceSDSMetadataExpandsSeparateVariantTasksByFamily(t *testing.T) {
	t.Cleanup(listingkit.SetOwnerScopeRequiredForTesting(false))

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&listingkit.Task{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := store.NewTaskRepository(db)
	source, ok := repo.(interface {
		ListSheinSourceSDSMetadata(ctx context.Context, query *listingkit.SheinSourceSDSMetadataQuery) ([]listingkit.SheinSourceSDSMetadataRecord, error)
	})
	if !ok {
		t.Fatal("task repo does not expose ListSheinSourceSDSMetadata")
	}

	createCtx := listingkit.WithTenantID(context.Background(), "373211199677923496")
	for _, task := range []*listingkit.Task{
		{
			ID:       "task-source-sds-family-12",
			TenantID: "373211199677923496",
			UserID:   "373211204509761704",
			Status:   listingkit.TaskStatusCompleted,
			Request: &listingkit.GenerateRequest{
				UserID:       "373211204509761704",
				Platforms:    []string{"shein"},
				SheinStoreID: 870,
				Options: &listingkit.GenerateOptions{SDS: &listingkit.SDSSyncOptions{
					ProductName:  "帆布拉绳收纳袋双面印刷",
					VariantSKU:   "XB0603003001",
					VariantColor: "white",
					VariantSize:  "12*18cm",
					VariantPrice: 8.81,
				}},
			},
			CreatedAt: time.Now().UTC().Add(-time.Minute),
			UpdatedAt: time.Now().UTC().Add(-time.Minute),
		},
		{
			ID:       "task-source-sds-family-20",
			TenantID: "373211199677923496",
			UserID:   "373211204509761704",
			Status:   listingkit.TaskStatusCompleted,
			Request: &listingkit.GenerateRequest{
				UserID:       "373211204509761704",
				Platforms:    []string{"shein"},
				SheinStoreID: 870,
				Options: &listingkit.GenerateOptions{SDS: &listingkit.SDSSyncOptions{
					ProductName:  "帆布拉绳收纳袋双面印刷",
					VariantSKU:   "XB0603003002",
					VariantColor: "white",
					VariantSize:  "20*25cm",
					VariantPrice: 9.64,
				}},
			},
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
	} {
		if err := repo.CreateTask(createCtx, task); err != nil {
			t.Fatalf("create source task %s: %v", task.ID, err)
		}
	}

	items, err := source.ListSheinSourceSDSMetadata(context.Background(), &listingkit.SheinSourceSDSMetadataQuery{
		StoreID:     870,
		SourceCodes: []string{"XB0603003001"},
	})
	if err != nil {
		t.Fatalf("ListSheinSourceSDSMetadata error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2: %+v", len(items), items)
	}
	if items[0].SourceCode != "XB0603003001" || items[0].VariantLabel != "white / 12*18cm" || items[0].Price != 8.81 {
		t.Fatalf("first metadata = %+v, want 12*18cm family variant", items[0])
	}
	if items[1].SourceCode != "XB0603003002" || items[1].VariantLabel != "white / 20*25cm" || items[1].Price != 9.64 {
		t.Fatalf("second metadata = %+v, want 20*25cm family variant", items[1])
	}
}

func TestTaskRepositoryListSheinSourceSDSMetadataUsesRequestUserWhenOwnerScopeDisabled(t *testing.T) {
	t.Cleanup(listingkit.SetOwnerScopeRequiredForTesting(false))

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&listingkit.Task{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := store.NewTaskRepository(db)
	source, ok := repo.(interface {
		ListSheinSourceSDSMetadata(ctx context.Context, query *listingkit.SheinSourceSDSMetadataQuery) ([]listingkit.SheinSourceSDSMetadataRecord, error)
	})
	if !ok {
		t.Fatal("task repo does not expose ListSheinSourceSDSMetadata")
	}

	createCtx := listingkit.WithTenantID(context.Background(), "373211199677923496")
	sourceTask := &listingkit.Task{
		ID:       "task-source-sds-owner-scope-off",
		TenantID: "373211199677923496",
		UserID:   "373211204509761704",
		Status:   listingkit.TaskStatusCompleted,
		Request: &listingkit.GenerateRequest{
			UserID:       "373211204509761704",
			Platforms:    []string{"shein"},
			SheinStoreID: 870,
			Options: &listingkit.GenerateOptions{SDS: &listingkit.SDSSyncOptions{
				ProductName: "三折钱包",
				Variants: []listingkit.SDSSyncVariantOption{{
					VariantSKU: "XB0608018002",
					Price:      22.5,
					Color:      "black",
				}},
			}},
		},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := repo.CreateTask(createCtx, sourceTask); err != nil {
		t.Fatalf("create source task: %v", err)
	}

	requestCtx := openaiclient.WithIdentity(
		listingkit.WithTenantID(context.Background(), "227"),
		openaiclient.Identity{TenantID: "227", UserID: "373211204509761704"},
	)
	items, err := source.ListSheinSourceSDSMetadata(requestCtx, &listingkit.SheinSourceSDSMetadataQuery{
		StoreID:     870,
		SourceCodes: []string{"XB0608018002"},
	})
	if err != nil {
		t.Fatalf("ListSheinSourceSDSMetadata error = %v", err)
	}
	if len(items) != 1 || items[0].Title != "三折钱包" {
		t.Fatalf("items = %+v, want title across tenant by request user", items)
	}
}

func TestTaskRepositoryListSheinSourceSDSMetadataFallsBackToStoreSourceWhenZitadelSubjectCannotMatchLegacyUser(t *testing.T) {
	t.Cleanup(listingkit.SetOwnerScopeRequiredForTesting(false))

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&listingkit.Task{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := store.NewTaskRepository(db)
	source, ok := repo.(interface {
		ListSheinSourceSDSMetadata(ctx context.Context, query *listingkit.SheinSourceSDSMetadataQuery) ([]listingkit.SheinSourceSDSMetadataRecord, error)
	})
	if !ok {
		t.Fatal("task repo does not expose ListSheinSourceSDSMetadata")
	}

	createCtx := listingkit.WithTenantID(context.Background(), "373211199677923496")
	sourceTask := &listingkit.Task{
		ID:       "task-source-sds-zitadel-subject-fallback",
		TenantID: "373211199677923496",
		UserID:   "373211204509761704",
		Status:   listingkit.TaskStatusCompleted,
		Request: &listingkit.GenerateRequest{
			UserID:       "373211204509761704",
			Platforms:    []string{"shein"},
			SheinStoreID: 870,
			Options: &listingkit.GenerateOptions{SDS: &listingkit.SDSSyncOptions{
				ProductName: "方形双层腰包 -（单图多拼可选）",
				Variants: []listingkit.SDSSyncVariantOption{{
					VariantSKU: "XB0610007001",
					Price:      34.5,
					Color:      "white",
					Size:       "16x23cm",
				}},
			}},
		},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	otherStoreTask := &listingkit.Task{
		ID:       "task-source-sds-other-store",
		TenantID: "373211199677923496",
		UserID:   "other-user",
		Status:   listingkit.TaskStatusCompleted,
		Request: &listingkit.GenerateRequest{
			UserID:       "other-user",
			Platforms:    []string{"shein"},
			SheinStoreID: 871,
			Options: &listingkit.GenerateOptions{SDS: &listingkit.SDSSyncOptions{
				ProductName: "其他店铺标题",
				Variants: []listingkit.SDSSyncVariantOption{{
					VariantSKU: "XB0610007001",
					Price:      99,
				}},
			}},
		},
		CreatedAt: time.Now().UTC().Add(time.Minute),
		UpdatedAt: time.Now().UTC().Add(time.Minute),
	}
	for _, task := range []*listingkit.Task{sourceTask, otherStoreTask} {
		if err := repo.CreateTask(createCtx, task); err != nil {
			t.Fatalf("create source task %s: %v", task.ID, err)
		}
	}

	requestCtx := openaiclient.WithIdentity(
		listingkit.WithTenantID(context.Background(), "227"),
		openaiclient.Identity{TenantID: "227", UserID: "zitadel-subject-42"},
	)
	requestCtx = listingkit.WithRequestRoles(requestCtx, []string{"listingkit_admin"})
	items, err := source.ListSheinSourceSDSMetadata(requestCtx, &listingkit.SheinSourceSDSMetadataQuery{
		StoreID:     870,
		SourceCodes: []string{"XB0610007001"},
	})
	if err != nil {
		t.Fatalf("ListSheinSourceSDSMetadata error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1: %+v", len(items), items)
	}
	if items[0].Title != "方形双层腰包 -（单图多拼可选）" || items[0].Price != 34.5 {
		t.Fatalf("items = %+v, want same-store historical SDS metadata", items)
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

package listingkit

import (
	"context"
	"testing"
	"time"
)

type stubTaskLifecycleBaselineReadinessService struct {
	readiness *SDSBaselineReadiness
	query     *SDSBaselineReadinessQuery
	err       error
}

func (s *stubTaskLifecycleBaselineReadinessService) GetReadiness(_ context.Context, query *SDSBaselineReadinessQuery) (*SDSBaselineReadiness, error) {
	s.query = query
	return s.readiness, s.err
}

func TestTaskLifecycleServiceListTasks(t *testing.T) {
	t.Parallel()

	now := time.Now()
	repo := &stubTaskListRepo{
		tasks: []Task{
			makeTaskListFixture("task-1", now.Add(-time.Minute), SheinWorkflowStatusPendingConfirmation, ""),
			makeTaskListFixture("task-2", now.Add(-2*time.Minute), SheinWorkflowStatusPublished, ""),
		},
	}
	lifecycle := newTaskLifecycleService(taskLifecycleServiceConfig{
		repo: repo,
	})

	page, err := lifecycle.ListTasks(context.Background(), &TaskListQuery{
		Page:                1,
		PageSize:            1,
		SheinWorkflowStatus: SheinWorkflowStatusPublished,
	})
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}
	if repo.lastQuery == nil || repo.lastQuery.SheinWorkflowStatus != SheinWorkflowStatusPublished {
		t.Fatalf("repo query = %+v, want shein workflow filter propagated", repo.lastQuery)
	}
	if page.Total != 1 {
		t.Fatalf("page total = %d, want 1", page.Total)
	}
	if len(page.Items) != 1 || page.Items[0].TaskID != "task-2" {
		t.Fatalf("page items = %+v, want task-2 only", page.Items)
	}
	if page.Summary != nil {
		t.Fatalf("page summary = %+v, want nil when include_summary is false", page.Summary)
	}
	if page.Taxonomy == nil || len(page.Taxonomy.SheinWorkflowStatuses) == 0 {
		t.Fatalf("page taxonomy = %+v, want workflow taxonomy", page.Taxonomy)
	}
}

func TestTaskLifecycleServiceListTasksIncludesSummaryWhenRequested(t *testing.T) {
	t.Parallel()

	now := time.Now()
	repo := &stubTaskListRepo{
		tasks: []Task{
			makeTaskListFixture("task-1", now.Add(-time.Minute), SheinWorkflowStatusPendingConfirmation, "category"),
			makeTaskListFixture("task-2", now.Add(-2*time.Minute), SheinWorkflowStatusPublished, "warning"),
		},
	}
	lifecycle := newTaskLifecycleService(taskLifecycleServiceConfig{
		repo: repo,
	})

	page, err := lifecycle.ListTasks(context.Background(), &TaskListQuery{
		Page:           1,
		PageSize:       10,
		IncludeSummary: true,
	})
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}
	if page.Summary == nil {
		t.Fatalf("page summary = nil, want populated summary")
	}
	if page.Summary.StatusCounts[string(TaskStatusCompleted)] != 2 {
		t.Fatalf("page summary = %+v, want completed count for both tasks", page.Summary)
	}
}

func TestTaskLifecycleServiceListTasksIncludesSDSSourceMetadata(t *testing.T) {
	t.Parallel()

	now := time.Now()
	task := makeTaskListFixture("task-sds", now, SheinWorkflowStatusPendingConfirmation, "")
	task.Request.Options = &GenerateOptions{SDS: &SDSSyncOptions{
		ProductName:     "SDS 方形挂钟",
		ProductSKU:      "MG8006905",
		VariantSKU:      "MG8006905001",
		VariantColor:    "白色",
		VariantSize:     "25x25cm",
		VariantPrice:    16.6,
		ParentProductID: 238915,
		VariantID:       238916,
	}}
	repo := &stubTaskListRepo{tasks: []Task{task}}
	lifecycle := newTaskLifecycleService(taskLifecycleServiceConfig{
		repo: repo,
	})

	page, err := lifecycle.ListTasks(context.Background(), &TaskListQuery{
		Page:     1,
		PageSize: 10,
		Platform: "shein",
	})
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("page items = %+v, want 1 item", page.Items)
	}
	item := page.Items[0]
	if item.ProductName != "SDS 方形挂钟" || item.SourceProductSKU != "MG8006905" || item.SourceVariantSKU != "MG8006905001" {
		t.Fatalf("item SDS source fields = %+v, want source title and SKUs", item)
	}
	if item.SourceVariantPrice != 16.6 {
		t.Fatalf("source variant price = %v, want 16.6", item.SourceVariantPrice)
	}
	if item.VariantLabel != "白色 25x25cm MG8006905001" {
		t.Fatalf("variant label = %q, want SDS variant label", item.VariantLabel)
	}
}

func TestTaskLifecycleServiceListTasksUsesSDSVariantOptionsWhenTopLevelSKUIsEmpty(t *testing.T) {
	t.Parallel()

	now := time.Now()
	task := makeTaskListFixture("task-sds-variant", now, SheinWorkflowStatusPendingConfirmation, "")
	task.Request.Options = &GenerateOptions{SDS: &SDSSyncOptions{
		ProductName: "方形双层腰包 -（单图多拼可选）",
		Variants: []SDSSyncVariantOption{{
			VariantID:  133065,
			VariantSKU: "XB0610007001",
			Color:      "white",
			Size:       "16x23cm",
			Price:      34.5,
		}},
	}}
	repo := &stubTaskListRepo{tasks: []Task{task}}
	lifecycle := newTaskLifecycleService(taskLifecycleServiceConfig{
		repo: repo,
	})

	page, err := lifecycle.ListTasks(context.Background(), &TaskListQuery{
		Page:     1,
		PageSize: 10,
		Platform: "shein",
	})
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("page items = %+v, want 1 item", page.Items)
	}
	item := page.Items[0]
	if item.SourceVariantSKU != "XB0610007001" {
		t.Fatalf("source variant sku = %q, want variant option SKU", item.SourceVariantSKU)
	}
	if item.SourceVariantPrice != 34.5 {
		t.Fatalf("source variant price = %v, want variant option price", item.SourceVariantPrice)
	}
	if item.VariantLabel != "white 16x23cm XB0610007001" {
		t.Fatalf("variant label = %q, want variant option label", item.VariantLabel)
	}
}

func TestTaskLifecycleServiceGetSDSBaselineReadinessUsesConfiguredService(t *testing.T) {
	t.Parallel()

	baselineSvc := &stubTaskLifecycleBaselineReadinessService{
		readiness: &SDSBaselineReadiness{
			BaselineKey: "baseline-key",
			Status:      SDSBaselineStatusReady,
		},
	}
	lifecycle := newTaskLifecycleService(taskLifecycleServiceConfig{
		repo:                        NewInMemoryRepositoryForTest(),
		sdsBaselineReadinessService: baselineSvc,
	})
	query := &SDSBaselineReadinessQuery{
		TenantID:           "tenant-a",
		ParentProductID:    9001,
		PrototypeGroupID:   7001,
		VariantID:          101,
		SelectedVariantIDs: []int64{101, 102},
	}

	readiness, err := lifecycle.GetSDSBaselineReadiness(context.Background(), query)
	if err != nil {
		t.Fatalf("GetSDSBaselineReadiness() error = %v", err)
	}
	if readiness == nil || readiness.Status != SDSBaselineStatusReady {
		t.Fatalf("readiness = %+v, want ready status", readiness)
	}
	if baselineSvc.query == nil || baselineSvc.query.TenantID != "tenant-a" {
		t.Fatalf("query = %+v, want forwarded query", baselineSvc.query)
	}
}

func TestTaskLifecycleServiceGetSDSBaselineReadinessReturnsConfigurationErrorWhenMissingService(t *testing.T) {
	t.Parallel()

	lifecycle := newTaskLifecycleService(taskLifecycleServiceConfig{
		repo: NewInMemoryRepositoryForTest(),
	})

	_, err := lifecycle.GetSDSBaselineReadiness(context.Background(), &SDSBaselineReadinessQuery{
		TenantID:           "tenant-a",
		ParentProductID:    9001,
		PrototypeGroupID:   7001,
		VariantID:          101,
		SelectedVariantIDs: []int64{101},
	})
	if err == nil || err.Error() != "sds baseline readiness service is not configured" {
		t.Fatalf("GetSDSBaselineReadiness() error = %v, want configuration error", err)
	}
}

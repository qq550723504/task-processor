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

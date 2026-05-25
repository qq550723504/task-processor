package listingkit

import (
	"context"
	"testing"
	"time"
)

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
	if page.Taxonomy == nil || len(page.Taxonomy.SheinWorkflowStatuses) == 0 {
		t.Fatalf("page taxonomy = %+v, want workflow taxonomy", page.Taxonomy)
	}
}

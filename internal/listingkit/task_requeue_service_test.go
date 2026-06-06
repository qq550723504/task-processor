package listingkit

import (
	"context"
	"errors"
	"testing"
)

func TestRequeuePendingTasksRequeuesOnlyPendingTasks(t *testing.T) {
	t.Parallel()

	repo := newTaskRecoveryServiceTestRepo()
	ctx := WithTenantID(context.Background(), "tenant-requeue")
	for _, task := range []*Task{
		{
			ID:       "task-pending",
			TenantID: "tenant-requeue",
			Status:   TaskStatusPending,
			Request:  &GenerateRequest{TenantID: "tenant-requeue", Platforms: []string{"shein"}, Text: "pending"},
		},
		{
			ID:       "task-review",
			TenantID: "tenant-requeue",
			Status:   TaskStatusNeedsReview,
			Request:  &GenerateRequest{TenantID: "tenant-requeue", Platforms: []string{"shein"}, Text: "review"},
		},
		{
			ID:       "task-processing",
			TenantID: "tenant-requeue",
			Status:   TaskStatusProcessing,
			Request:  &GenerateRequest{TenantID: "tenant-requeue", Platforms: []string{"shein"}, Text: "processing"},
		},
	} {
		if err := repo.CreateTask(ctx, task); err != nil {
			t.Fatalf("CreateTask(%s) error = %v", task.ID, err)
		}
	}

	submitted := make([]string, 0, 1)
	svc := newTaskRequeueService(taskRequeueServiceConfig{
		repo: repo,
		taskSubmitter: func() TaskSubmitter {
			return taskRecoveryTestSubmitter(func(taskID string) error {
				submitted = append(submitted, taskID)
				return nil
			})
		},
	})

	result, err := svc.RequeuePendingTasks(ctx, &RequeuePendingTasksRequest{
		TaskIDs: []string{"task-pending", "task-review", "task-processing", "task-missing", "task-pending"},
	})
	if err != nil {
		t.Fatalf("RequeuePendingTasks() error = %v", err)
	}

	if len(result.RequeuedTaskIDs) != 1 || result.RequeuedTaskIDs[0] != "task-pending" {
		t.Fatalf("RequeuedTaskIDs = %v, want [task-pending]", result.RequeuedTaskIDs)
	}
	if len(submitted) != 1 || submitted[0] != "task-pending" {
		t.Fatalf("submitted = %v, want [task-pending]", submitted)
	}
	if len(result.Skipped) != 3 {
		t.Fatalf("Skipped len = %d, want 3", len(result.Skipped))
	}
}

func TestRequeuePendingTasksReportsSubmitFailures(t *testing.T) {
	t.Parallel()

	repo := newTaskRecoveryServiceTestRepo()
	ctx := WithTenantID(context.Background(), "tenant-requeue-fail")
	task := &Task{
		ID:       "task-submit-fail",
		TenantID: "tenant-requeue-fail",
		Status:   TaskStatusPending,
		Request:  &GenerateRequest{TenantID: "tenant-requeue-fail", Platforms: []string{"shein"}, Text: "pending"},
	}
	if err := repo.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	svc := newTaskRequeueService(taskRequeueServiceConfig{
		repo: repo,
		taskSubmitter: func() TaskSubmitter {
			return taskRecoveryTestSubmitter(func(taskID string) error {
				return errors.New("submit failed")
			})
		},
	})

	result, err := svc.RequeuePendingTasks(ctx, &RequeuePendingTasksRequest{
		TaskIDs: []string{task.ID},
	})
	if err != nil {
		t.Fatalf("RequeuePendingTasks() error = %v", err)
	}
	if len(result.Failed) != 1 || result.Failed[0].TaskID != task.ID {
		t.Fatalf("Failed = %+v, want one failed task", result.Failed)
	}
}

func TestRequeuePendingTasksRejectsUnavailableSubmitter(t *testing.T) {
	t.Parallel()

	svc := newTaskRequeueService(taskRequeueServiceConfig{repo: newTaskRecoveryServiceTestRepo()})
	if _, err := svc.RequeuePendingTasks(context.Background(), &RequeuePendingTasksRequest{TaskIDs: []string{"task-1"}}); !errors.Is(err, ErrTaskRequeueUnavailable) {
		t.Fatalf("RequeuePendingTasks() error = %v, want ErrTaskRequeueUnavailable", err)
	}
}

package listingkit

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"task-processor/internal/listingkit/core"
)

func TestProcessStandardProductLayerDoesNotStartAdaptationWhenBlocked(t *testing.T) {
	repo := &stubInlineTaskRepo{tasks: map[string]*Task{}}
	task := &Task{
		ID:       "task-blocked-standard",
		TenantID: "tenant-1",
		Status:   core.TaskStatusPending,
		Request:  &GenerateRequest{ProductKey: "product-1", UserID: "user-1"},
	}
	require.NoError(t, repo.CreateTask(context.Background(), task))
	client := &stubPlatformAdaptWorkflowClient{}
	svc := &service{
		repo:         repo,
		workflowDeps: workflowDependencies{},
		taskDeps: taskDependencies{
			platformAdaptWorkflowClient:  client,
			platformAdaptWorkflowEnabled: true,
		},
	}

	_, err := svc.ProcessStandardProductLayer(context.Background(), task.ID)

	require.NoError(t, err)
	require.Empty(t, client.calls)
	stored, getErr := repo.GetTask(context.Background(), task.ID)
	require.NoError(t, getErr)
	require.Equal(t, core.TaskStatusBlockedRetryable, stored.Status)
	require.NotNil(t, stored.RetryableBlock)
	require.Equal(t, standardProductReadinessBlockReason, stored.RetryableBlock.ReasonCode)
	require.True(t, stored.RetryableBlock.AutoResumeEnabled)
	require.NotNil(t, stored.RetryableBlock.NextRetryAt)
	require.NotNil(t, stored.Result)
	require.NotNil(t, stored.Result.StandardProductSnapshot)
}

func TestProcessStandardProductLayerReblocksWithExistingRetryState(t *testing.T) {
	recoveredAt := time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)
	previousBlockedAt := recoveredAt.Add(-10 * time.Minute)
	lastRetryAt := recoveredAt.Add(-2 * time.Minute)
	repo := &stubInlineTaskRepo{tasks: map[string]*Task{}}
	task := &Task{
		ID:       "task-blocked-standard-retry-state",
		TenantID: "tenant-1",
		Status:   core.TaskStatusPending,
		Request:  &GenerateRequest{ProductKey: "product-1", UserID: "user-1"},
		RetryableBlock: &RetryableBlock{
			ReasonCode:           "upstream_timeout",
			ReasonMessage:        "upstream timeout",
			BlockedAt:            previousBlockedAt,
			LastRetryAt:          &lastRetryAt,
			RetryAttempts:        2,
			MaxAutoRetryAttempts: 8,
			RecoveryScope:        "task",
			AutoResumeEnabled:    true,
		},
	}
	require.NoError(t, repo.CreateTask(context.Background(), task))

	_, err := (&service{repo: repo}).ProcessStandardProductLayer(context.Background(), task.ID)

	require.NoError(t, err)
	stored, getErr := repo.GetTask(context.Background(), task.ID)
	require.NoError(t, getErr)
	require.NotNil(t, stored.RetryableBlock)
	require.Equal(t, 3, stored.RetryableBlock.RetryAttempts)
	require.Equal(t, previousBlockedAt, stored.RetryableBlock.BlockedAt)
	require.NotNil(t, stored.RetryableBlock.LastRetryAt)
	require.NotNil(t, stored.RetryableBlock.NextRetryAt)
}

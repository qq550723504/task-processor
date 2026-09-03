package listingkit

import (
	"context"
	"testing"

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

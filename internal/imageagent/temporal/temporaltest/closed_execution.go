// Package temporaltest contains fixture orchestration for existing integration
// tests. Production composition must not import it.
package temporaltest

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	sdkclient "go.temporal.io/sdk/client"
	sdkworker "go.temporal.io/sdk/worker"
	sdkworkflow "go.temporal.io/sdk/workflow"
)

// CompleteEmptyExecution preserves the completed-execution premise of the
// HTTP approval replay test without putting SDK orchestration in HTTP code.
// It executes a real workflow, waits for successful completion, and stops its
// isolated worker; it does not replace completion with cancellation/termination.
func CompleteEmptyExecution(t *testing.T, ctx context.Context, client sdkclient.Client, workflowID string) {
	t.Helper()
	queue := "issue487-closed-" + uuid.NewString()
	w := sdkworker.New(client, queue, sdkworker.Options{})
	w.RegisterWorkflowWithOptions(func(sdkworkflow.Context) error { return nil }, sdkworkflow.RegisterOptions{Name: "Issue487CompletedReplayWorkflow"})
	require.NoError(t, w.Start())
	defer w.Stop()
	closed, err := client.ExecuteWorkflow(ctx, sdkclient.StartWorkflowOptions{ID: workflowID, TaskQueue: queue}, "Issue487CompletedReplayWorkflow")
	require.NoError(t, err)
	require.NoError(t, closed.Get(ctx, nil))
}

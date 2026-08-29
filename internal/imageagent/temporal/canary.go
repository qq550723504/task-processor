package temporal

import (
	"context"
	"fmt"
	"strings"

	sdkclient "go.temporal.io/sdk/client"
	sdkworkflow "go.temporal.io/sdk/workflow"
)

const compatibilityCanaryBuild = "image-agent-temporal-v3"

type CompatibilityCanaryAcknowledgement struct {
	Build    string
	WireMode WorkerWireMode
}

type compatibilityCanaryClient interface {
	ExecuteWorkflow(context.Context, sdkclient.StartWorkflowOptions, interface{}, ...interface{}) (sdkclient.WorkflowRun, error)
}

// ImageAgentCompatibilityCanaryWorkflow exercises only workflow polling and
// registration. It intentionally has no activities or business dependencies.
func ImageAgentCompatibilityCanaryWorkflow(sdkworkflow.Context) (CompatibilityCanaryAcknowledgement, error) {
	return CompatibilityCanaryAcknowledgement{Build: compatibilityCanaryBuild, WireMode: WorkerWireModeV3}, nil
}

func RunImageAgentCompatibilityCanary(ctx context.Context, client sdkclient.Client, taskQueue string) error {
	return runImageAgentCompatibilityCanary(ctx, client, taskQueue)
}

func runImageAgentCompatibilityCanary(ctx context.Context, client compatibilityCanaryClient, taskQueue string) error {
	if client == nil {
		return fmt.Errorf("image agent compatibility canary temporal client is required")
	}
	taskQueue = strings.TrimSpace(taskQueue)
	if taskQueue == "" {
		return fmt.Errorf("image agent compatibility canary v3 task queue is required")
	}
	workflowID, err := newTransportUpdateID("image-agent-compatibility-canary")
	if err != nil {
		return err
	}
	run, err := client.ExecuteWorkflow(ctx, sdkclient.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: taskQueue,
	}, workflowNameCompatibilityCanary)
	if err != nil {
		return fmt.Errorf("start image agent compatibility canary: %w", err)
	}
	var acknowledgement CompatibilityCanaryAcknowledgement
	if err := run.Get(ctx, &acknowledgement); err != nil {
		return fmt.Errorf("wait for image agent compatibility canary: %w", err)
	}
	if acknowledgement != (CompatibilityCanaryAcknowledgement{Build: compatibilityCanaryBuild, WireMode: WorkerWireModeV3}) {
		return fmt.Errorf("unexpected image agent compatibility canary acknowledgement: build=%q wire_mode=%q", acknowledgement.Build, acknowledgement.WireMode)
	}
	return nil
}

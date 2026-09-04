package temporal

import (
	"fmt"
	"strings"
	"time"

	sdktemporal "go.temporal.io/sdk/temporal"
	sdkworkflow "go.temporal.io/sdk/workflow"

	"task-processor/internal/listingkit"
)

func StandardProductWorkflow(ctx sdkworkflow.Context, in StandardProductWorkflowInput) (*listingkit.StandardProductSnapshot, error) {
	in = normalizeStandardProductWorkflowInput(ctx, in)
	ctx = sdkworkflow.WithActivityOptions(ctx, sdkworkflow.ActivityOptions{
		StartToCloseTimeout: 15 * time.Minute,
		RetryPolicy: &sdktemporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2,
			MaximumInterval:    30 * time.Second,
			MaximumAttempts:    3,
		},
	})
	var snapshot *listingkit.StandardProductSnapshot
	if err := sdkworkflow.ExecuteActivity(ctx, activityNameProcessStandardProduct, in).Get(ctx, &snapshot); err != nil {
		return nil, persistLayerFailure(ctx, in.TaskID, err)
	}
	return snapshot, nil
}

func PlatformAdaptWorkflow(ctx sdkworkflow.Context, in PlatformAdaptWorkflowInput) (*listingkit.ListingKitResult, error) {
	in = normalizePlatformAdaptWorkflowInput(ctx, in)
	ctx = sdkworkflow.WithActivityOptions(ctx, sdkworkflow.ActivityOptions{
		StartToCloseTimeout: 15 * time.Minute,
		RetryPolicy: &sdktemporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2,
			MaximumInterval:    30 * time.Second,
			MaximumAttempts:    3,
		},
	})
	var result *listingkit.ListingKitResult
	if err := sdkworkflow.ExecuteActivity(ctx, activityNameProcessPlatformAdapt, in).Get(ctx, &result); err != nil {
		return nil, persistLayerFailure(ctx, in.TaskID, err)
	}
	return result, nil
}

func persistLayerFailure(ctx sdkworkflow.Context, taskID string, cause error) error {
	failureCtx := sdkworkflow.WithActivityOptions(ctx, sdkworkflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
		RetryPolicy: &sdktemporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2,
			MaximumInterval:    10 * time.Second,
			MaximumAttempts:    3,
		},
	})
	persistErr := sdkworkflow.ExecuteActivity(failureCtx, activityNamePersistLayerFailure, LayerFailureInput{
		TaskID: taskID,
		Error:  cause.Error(),
	}).Get(failureCtx, nil)
	if persistErr != nil {
		return fmt.Errorf("layer processing failed: %v; persist terminal state: %w", cause, persistErr)
	}
	return cause
}

func normalizeStandardProductWorkflowInput(ctx sdkworkflow.Context, in StandardProductWorkflowInput) StandardProductWorkflowInput {
	in.TaskID = strings.TrimSpace(in.TaskID)
	in.TriggeredByUser = strings.TrimSpace(in.TriggeredByUser)
	if in.RequestedAt.IsZero() {
		in.RequestedAt = sdkworkflow.Now(ctx)
	}
	return in
}

func normalizePlatformAdaptWorkflowInput(ctx sdkworkflow.Context, in PlatformAdaptWorkflowInput) PlatformAdaptWorkflowInput {
	in.TaskID = strings.TrimSpace(in.TaskID)
	in.Platform = strings.ToLower(strings.TrimSpace(in.Platform))
	in.TriggeredByUser = strings.TrimSpace(in.TriggeredByUser)
	if in.Platform == "" {
		in.Platform = "all"
	}
	if in.RequestedAt.IsZero() {
		in.RequestedAt = sdkworkflow.Now(ctx)
	}
	return in
}

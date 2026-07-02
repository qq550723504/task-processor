package temporal

import (
	"context"
	"fmt"
	"strings"

	enumspb "go.temporal.io/api/enums/v1"
	sdkclient "go.temporal.io/sdk/client"
	sdkconverter "go.temporal.io/sdk/converter"
	sdktemporal "go.temporal.io/sdk/temporal"

	listingsubmission "task-processor/internal/listing/submission"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/core"
)

type encodedValue = sdkconverter.EncodedValue

type workflowClient interface {
	ExecuteWorkflow(ctx context.Context, options sdkclient.StartWorkflowOptions, workflow interface{}, args ...interface{}) (sdkclient.WorkflowRun, error)
	QueryWorkflow(ctx context.Context, workflowID string, runID string, queryType string, args ...interface{}) (sdkconverter.EncodedValue, error)
}

type Client struct {
	client workflowClient
}

func NewClient(client workflowClient) *Client {
	return &Client{client: client}
}

func (c *Client) StartStandardProduct(ctx context.Context, in listingkit.StandardProductWorkflowStartInput) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("temporal client is not configured")
	}
	in = listingkit.NormalizeStandardProductWorkflowStartInputForTemporal(in)
	_, err := c.client.ExecuteWorkflow(ctx, sdkclient.StartWorkflowOptions{
		ID:                                       WorkflowIDForStandardProduct(in.TaskID),
		TaskQueue:                                TaskQueueStandardProductName(),
		WorkflowIDConflictPolicy:                 enumspb.WORKFLOW_ID_CONFLICT_POLICY_FAIL,
		WorkflowIDReusePolicy:                    enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE,
		WorkflowExecutionErrorWhenAlreadyStarted: true,
	}, StandardProductWorkflow, StandardProductWorkflowInput{
		TaskID:          in.TaskID,
		RequestedAt:     in.RequestedAt,
		TriggeredByUser: in.TriggeredByUser,
	})
	return err
}

func (c *Client) StartPlatformAdaptation(ctx context.Context, in listingkit.PlatformAdaptWorkflowStartInput) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("temporal client is not configured")
	}
	in = listingkit.NormalizePlatformAdaptWorkflowStartInputForTemporal(in)
	_, err := c.client.ExecuteWorkflow(ctx, sdkclient.StartWorkflowOptions{
		ID:                                       WorkflowIDForPlatformAdapt(in.TaskID, in.Platform),
		TaskQueue:                                TaskQueuePlatformAdaptName(),
		WorkflowIDConflictPolicy:                 enumspb.WORKFLOW_ID_CONFLICT_POLICY_FAIL,
		WorkflowIDReusePolicy:                    enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE,
		WorkflowExecutionErrorWhenAlreadyStarted: true,
	}, PlatformAdaptWorkflow, PlatformAdaptWorkflowInput{
		TaskID:          in.TaskID,
		Platform:        in.Platform,
		RequestedAt:     in.RequestedAt,
		TriggeredByUser: in.TriggeredByUser,
	})
	return err
}

func (c *Client) StartSheinPublish(ctx context.Context, in listingkit.SheinPublishWorkflowStartInput) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("temporal client is not configured")
	}
	in = normalizeStartInput(in)
	_, err := c.client.ExecuteWorkflow(ctx, sdkclient.StartWorkflowOptions{
		ID:                                       WorkflowIDForSheinPublish(in.TaskID),
		TaskQueue:                                TaskQueueSheinSubmitPublishName(),
		WorkflowIDConflictPolicy:                 enumspb.WORKFLOW_ID_CONFLICT_POLICY_FAIL,
		WorkflowIDReusePolicy:                    enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE,
		WorkflowExecutionErrorWhenAlreadyStarted: true,
	}, PublishWorkflow, SheinPublishWorkflowInput{
		TaskID:          in.TaskID,
		Platform:        in.Platform,
		Action:          in.Action,
		RequestID:       in.RequestID,
		ConfirmedFinal:  in.ConfirmedFinal,
		RequestedAt:     in.RequestedAt,
		TriggeredByUser: in.TriggeredByUser,
	})
	if err != nil {
		if sdktemporal.IsWorkflowExecutionAlreadyStartedError(err) {
			state, queryErr := c.QuerySheinPublishState(ctx, in.TaskID)
			if queryErr == nil && state != nil {
				return listingsubmission.NewSubmitInProgressError(in.Platform, in.Action, state.CurrentPhase, state.RequestID, nil)
			}
			return fmt.Errorf("%w: shein publish workflow already started", core.ErrSubmitInProgress)
		}
		return err
	}
	return nil
}

func (c *Client) QuerySheinPublishState(ctx context.Context, taskID string) (*listingkit.SheinPublishWorkflowState, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("temporal client is not configured")
	}
	workflowID := WorkflowIDForSheinPublish(strings.TrimSpace(taskID))
	value, err := c.client.QueryWorkflow(ctx, workflowID, "", SheinPublishQueryCurrentState)
	if err != nil {
		return nil, err
	}
	var state SheinPublishStateQueryResult
	if err := value.Get(&state); err != nil {
		return nil, err
	}
	return &listingkit.SheinPublishWorkflowState{
		TaskID:          state.TaskID,
		Action:          state.Action,
		RequestID:       state.RequestID,
		CurrentPhase:    state.CurrentPhase,
		LastError:       state.LastError,
		StartedAt:       state.StartedAt,
		FinishedAt:      state.FinishedAt,
		WorkflowRunning: state.WorkflowRunning,
	}, nil
}

func normalizeStartInput(in listingkit.SheinPublishWorkflowStartInput) listingkit.SheinPublishWorkflowStartInput {
	in.TaskID = strings.TrimSpace(in.TaskID)
	in.Platform = strings.ToLower(strings.TrimSpace(in.Platform))
	in.Action = strings.ToLower(strings.TrimSpace(in.Action))
	in.RequestID = strings.TrimSpace(in.RequestID)
	in.TriggeredByUser = strings.TrimSpace(in.TriggeredByUser)
	if in.Platform == "" {
		in.Platform = "shein"
	}
	if in.Action == "" {
		in.Action = "publish"
	}
	return in
}

package temporal

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	sdkclient "go.temporal.io/sdk/client"
)

func TestRunImageAgentCompatibilityCanaryUsesConfiguredQueueAndExactAcknowledgement(t *testing.T) {
	client := &recordingCanaryClient{acknowledgement: CompatibilityCanaryAcknowledgement{Build: compatibilityCanaryBuild, WireMode: WorkerWireModeV3}}

	require.NoError(t, runImageAgentCompatibilityCanary(context.Background(), client, "image-agent-manual-v3-canary"))
	require.Equal(t, "image-agent-manual-v3-canary", client.options.TaskQueue)
	require.Equal(t, workflowNameCompatibilityCanary, client.workflow)
	require.True(t, strings.HasPrefix(client.options.ID, "image-agent-compatibility-canary:"))
}

func TestRunImageAgentCompatibilityCanaryRejectsWrongAcknowledgement(t *testing.T) {
	client := &recordingCanaryClient{acknowledgement: CompatibilityCanaryAcknowledgement{Build: compatibilityCanaryBuild, WireMode: WorkerWireModeV2}}

	err := runImageAgentCompatibilityCanary(context.Background(), client, TaskQueueV3)
	require.ErrorContains(t, err, "unexpected image agent compatibility canary acknowledgement")
}

type recordingCanaryClient struct {
	options         sdkclient.StartWorkflowOptions
	workflow        string
	acknowledgement CompatibilityCanaryAcknowledgement
	err             error
}

func (c *recordingCanaryClient) ExecuteWorkflow(_ context.Context, options sdkclient.StartWorkflowOptions, workflow interface{}, _ ...interface{}) (sdkclient.WorkflowRun, error) {
	c.options = options
	c.workflow = workflow.(string)
	if c.err != nil {
		return nil, c.err
	}
	return recordingCanaryRun{acknowledgement: c.acknowledgement}, nil
}

type recordingCanaryRun struct {
	acknowledgement CompatibilityCanaryAcknowledgement
	err             error
}

func (r recordingCanaryRun) GetID() string         { return "canary" }
func (r recordingCanaryRun) GetWorkflowID() string { return "canary" }
func (r recordingCanaryRun) GetRunID() string      { return "run" }
func (r recordingCanaryRun) Get(_ context.Context, target interface{}) error {
	if r.err != nil {
		return r.err
	}
	acknowledgement, ok := target.(*CompatibilityCanaryAcknowledgement)
	if !ok {
		return errors.New("unexpected canary result target")
	}
	*acknowledgement = r.acknowledgement
	return nil
}

func (r recordingCanaryRun) GetWithOptions(ctx context.Context, target interface{}, _ sdkclient.WorkflowRunGetOptions) error {
	return r.Get(ctx, target)
}

package temporal

import (
	"context"
	"github.com/stretchr/testify/require"
	sdkclient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/testsuite"
	"strings"
	"task-processor/internal/imageagent"
	"testing"
)

func TestLegacyActivityRejectsOrganizationIdentity(t *testing.T) {
	_, err := restoreActivityIdentity(context.Background(), imageagent.ExecutionIdentity{ScopeProtocol: imageagent.OrganizationScopeProtocol, TenantID: "org-b", UserID: "actor", RunID: "run", BusinessTaskID: "context"})
	require.Error(t, err)
}

func TestOrganizationClientAndWorkflowModeIsolation(t *testing.T) {
	identity := imageagent.ExecutionIdentity{ScopeProtocol: imageagent.OrganizationScopeProtocol, TenantID: "org-b", UserID: "actor", RunID: "run", BusinessTaskID: "context"}
	raw := &recordingSDKClient{}
	_, err := NewClient(raw).Resume(context.Background(), imageagent.ResumeCommand{RunID: "run", ActorID: "actor", ActionID: "resume", Identity: identity})
	require.Error(t, err)
	require.Empty(t, raw.updateOptions)
	_, err = NewOrganizationClient(raw).Resume(context.Background(), imageagent.ResumeCommand{RunID: "run", ActorID: "actor", ActionID: "resume", Identity: imageagent.ExecutionIdentity{TenantID: "org-b", UserID: "actor"}})
	require.Error(t, err)
	require.Empty(t, raw.updateOptions)
	input := EffectRecoveryWorkflowInput{RunID: "run", Identity: identity, PlanRevision: 1, Slot: imageagent.Slot{ID: "slot"}, Attempt: 1}
	require.Error(t, newRecoveryWorkflowStarter(raw, TaskQueueV3)(context.Background(), input))
	require.Empty(t, raw.workflowName)
	require.NoError(t, newRecoveryWorkflowStarter(raw, OrganizationTaskQueue)(context.Background(), input))
	require.True(t, strings.HasPrefix(raw.startOptions.ID, "organization-v1:"))
	require.Equal(t, input.Identity, raw.effectRecoveryInput.Identity)
	for _, queue := range []string{TaskQueueV3, OrganizationTaskQueue} {
		bad := identity
		if queue == OrganizationTaskQueue {
			bad.ScopeProtocol = ""
			bad.RunID = ""
		}
		for _, kind := range []string{"parent", "slot", "recovery"} {
			t.Run(queue+"/"+kind, func(t *testing.T) {
				var suite testsuite.WorkflowTestSuite
				env := suite.NewTestWorkflowEnvironment()
				env.SetStartWorkflowOptions(sdkclient.StartWorkflowOptions{TaskQueue: queue})
				switch kind {
				case "parent":
					env.ExecuteWorkflow(ImageAgentWorkflow, WorkflowInput{RunID: "run", Identity: bad})
				case "slot":
					env.ExecuteWorkflow(ImageSlotWorkflowV3, SlotWorkflowV3Input{RunID: "run", Identity: bad})
				case "recovery":
					env.ExecuteWorkflow(ImageAgentEffectRecoveryWorkflow, EffectRecoveryWorkflowInput{RunID: "run", Identity: bad})
				}
				require.Error(t, env.GetWorkflowError())
				require.Contains(t, env.GetWorkflowError().Error(), imageagent.ErrIdentityRequired.Error())
			})
		}
	}
}

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

func TestOrganizationClientRequiresCompleteBoundIdentityAtEveryIngress(t *testing.T) {
	valid := imageagent.ExecutionIdentity{ScopeProtocol: imageagent.OrganizationScopeProtocol, TenantID: "org-b", UserID: "actor", RunID: "run", BusinessTaskID: "context"}
	run := imageagent.Run{ID: "run", ScopeProtocol: valid.ScopeProtocol, TenantID: valid.TenantID, UserID: valid.UserID, BusinessTaskID: valid.BusinessTaskID, Mode: imageagent.RunModeManual, TargetPlatform: "shein", ImagePolicyContext: imageagent.ImagePolicyContext{Country: "us", Family: "default", SceneCategory: "shoes"}}
	replacement := sevenSlotPlan()
	replacement.Revision, replacement.ParentRevision, replacement.IdempotencyKey = 2, 1, "replacement"
	projection := imageagent.RunProjection{Run: run, Plan: sevenSlotPlan(), Slots: []imageagent.SlotProjection{{Slot: imageagent.Slot{ID: "slot-1", Status: imageagent.SlotStatusBlocked}, Attempt: 1, ErrorCode: "recovery_start_failed"}}}
	projection.Run.Status = imageagent.RunStatusBlocked
	projection.Run.Block = &imageagent.Block{Code: "recovery_start_failed", SlotID: "slot-1"}
	entries := map[string]func(*Client, imageagent.ExecutionIdentity) error{
		"StartManual": func(c *Client, id imageagent.ExecutionIdentity) error {
			return c.StartManual(context.Background(), imageagent.WorkflowStart{Run: run, Plan: sevenSlotPlan(), Identity: id})
		},
		"ReplacePlan": func(c *Client, id imageagent.ExecutionIdentity) error {
			return c.ReplacePlan(context.Background(), imageagent.ReplacePlanCommand{RunID: "run", ExpectedRevision: 1, Plan: replacement, ActorID: "actor", ActionID: "replace", Identity: id})
		},
		"RetrySlot": func(c *Client, id imageagent.ExecutionIdentity) error {
			return c.RetrySlot(context.Background(), imageagent.RetrySlotCommand{RunID: "run", PlanRevision: 1, SlotID: "slot-1", ActorID: "actor", ActionID: "retry", Identity: id})
		},
		"ApproveResults": func(c *Client, id imageagent.ExecutionIdentity) error {
			return c.ApproveResults(context.Background(), imageagent.ApproveResultsCommand{RunID: "run", PlanRevision: 1, ResultDigest: sevenSlotResultDigest, ActorID: "actor", ActionID: "approve", Identity: id})
		},
		"Cancel": func(c *Client, id imageagent.ExecutionIdentity) error {
			return c.Cancel(context.Background(), imageagent.CancelRunCommand{RunID: "run", PlanRevision: 1, ActorID: "actor", ActionID: "cancel", Identity: id})
		},
		"Resume": func(c *Client, id imageagent.ExecutionIdentity) error {
			_, err := c.Resume(context.Background(), imageagent.ResumeCommand{RunID: "run", ActorID: "actor", ActionID: "resume", Identity: id})
			return err
		},
		"GetProjection": func(c *Client, id imageagent.ExecutionIdentity) error {
			_, err := c.GetProjection(context.Background(), imageagent.RunScope{TenantID: "org-b", OwnerUserID: "actor", RunID: "run"}, id)
			return err
		},
		"RecoverEffect": func(c *Client, id imageagent.ExecutionIdentity) error {
			return c.RecoverEffect(context.Background(), imageagent.RecoverEffectCommand{RunID: "run", PlanRevision: 1, SlotID: "slot-1", Attempt: 1, ActionID: "recover", Identity: id, Projection: projection})
		},
	}
	mutations := map[string]func(*imageagent.ExecutionIdentity){
		"missing run":      func(id *imageagent.ExecutionIdentity) { id.RunID = "" },
		"conflicting run":  func(id *imageagent.ExecutionIdentity) { id.RunID = "other" },
		"missing context":  func(id *imageagent.ExecutionIdentity) { id.BusinessTaskID = "" },
		"missing org":      func(id *imageagent.ExecutionIdentity) { id.TenantID = "" },
		"missing actor":    func(id *imageagent.ExecutionIdentity) { id.UserID = "" },
		"legacy":           func(id *imageagent.ExecutionIdentity) { id.ScopeProtocol = "" },
		"unknown protocol": func(id *imageagent.ExecutionIdentity) { id.ScopeProtocol = "unknown" },
		"padded context":   func(id *imageagent.ExecutionIdentity) { id.BusinessTaskID = " context " },
	}
	for name, call := range entries {
		t.Run(name+"/valid", func(t *testing.T) {
			raw := &recordingSDKClient{}
			require.NoError(t, call(NewOrganizationClient(raw), valid))
			require.True(t, raw.workflowName != "" || raw.queryWorkflowID != "" || len(raw.updateOptions) == 1, "valid entry must reach the SDK")
			if name == "StartManual" {
				require.Equal(t, valid, raw.workflowInput.Identity)
			}
		})
		for defect, mutate := range mutations {
			t.Run(name+"/"+defect, func(t *testing.T) {
				bad := valid
				mutate(&bad)
				raw := &recordingSDKClient{}
				err := call(NewOrganizationClient(raw), bad)
				require.Empty(t, raw.workflowName, "rejected identity must not start a workflow")
				require.Empty(t, raw.queryWorkflowID, "rejected identity must not query a workflow")
				require.Empty(t, raw.updateOptions, "rejected identity must not send an update")
				require.Empty(t, raw.signalName)
				require.ErrorIs(t, err, imageagent.ErrIdentityRequired)
			})
		}
	}
}

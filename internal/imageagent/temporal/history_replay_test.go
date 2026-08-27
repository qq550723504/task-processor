package temporal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	historypb "go.temporal.io/api/history/v1"
	sdkconverter "go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/testsuite"
	sdkworker "go.temporal.io/sdk/worker"
	sdkworkflow "go.temporal.io/sdk/workflow"
	"google.golang.org/protobuf/encoding/protojson"

	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/store"
)

func TestReplayV2SlotInflightHistory(t *testing.T) {
	replayer := sdkworker.NewWorkflowReplayer()
	replayer.RegisterWorkflowWithOptions(ImageSlotWorkflow, sdkworkflow.RegisterOptions{Name: workflowNameImageSlot})

	require.NoError(t, replayer.ReplayWorkflowHistory(nil, readHistoryFixture(t, "v2-slot-inflight.json")))
}

func TestReplayV2AwaitingApprovalHistory(t *testing.T) {
	replayer := sdkworker.NewWorkflowReplayer()
	replayer.RegisterWorkflowWithOptions(ImageAgentWorkflow, sdkworkflow.RegisterOptions{Name: workflowNameImageAgent})
	replayer.RegisterWorkflowWithOptions(ImageSlotWorkflow, sdkworkflow.RegisterOptions{Name: workflowNameImageSlot})

	require.NoError(t, replayer.ReplayWorkflowHistory(nil, readHistoryFixture(t, "v2-awaiting-approval.json")))
}

func TestOldApprovalHistoryRetainsPlanDerivedKeyAndV2Digest(t *testing.T) {
	history := readHistoryFixture(t, "v2-awaiting-approval.json")
	var awaiting WorkflowResult
	for _, event := range history.Events {
		attributes := event.GetActivityTaskScheduledEventAttributes()
		if attributes == nil || attributes.GetActivityType().GetName() != activityPersistRunState {
			continue
		}
		var input PersistRunStateActivityInput
		require.NoError(t, sdkconverter.GetDefaultDataConverter().FromPayloads(attributes.Input, &input))
		if input.Projection.Status == imageagent.RunStatusAwaitingFinalApproval {
			awaiting = input.Projection
		}
	}
	require.NotEmpty(t, awaiting.ResultDigest)
	wantDigest, err := imageagent.ResultDigestV2(awaiting.Plan, awaiting.Slots)
	require.NoError(t, err)
	require.Equal(t, wantDigest, awaiting.ResultDigest)

	var publish PublishApprovedActivityInput
	found := false
	for _, event := range history.Events {
		attributes := event.GetActivityTaskScheduledEventAttributes()
		if attributes == nil || attributes.GetActivityType().GetName() != "imageagent.publish_approved.v2" {
			continue
		}
		require.NoError(t, sdkconverter.GetDefaultDataConverter().FromPayloads(attributes.Input, &publish))
		found = true
		break
	}
	require.True(t, found, "captured history must schedule imageagent.publish_approved.v2")
	require.Equal(t, publicationKey(publish.RunID, publish.PlanRevision), publish.IdempotencyKey)
}

type wireProbeResult struct {
	ExecuteSlot      string
	PublishApproved  string
	ApprovalActionID string
	ResultDigest     string
}

func workflowWireProbe(ctx sdkworkflow.Context) (wireProbeResult, error) {
	wire := activityWireForWorkflow(ctx)
	plan, results := wireProbePlanAndResults()
	digest, err := resultDigestForWire(plan, results, wire)
	if err != nil {
		return wireProbeResult{}, err
	}
	return wireProbeResult{
		ExecuteSlot:      wire.executeSlot,
		PublishApproved:  wire.publishApproved,
		ApprovalActionID: approvalPublicationKeyForWorkflow(ctx, "capture-action", "capture-run", 1),
		ResultDigest:     digest,
	}, nil
}

func TestNewWorkflowSelectsExecuteSlotV3(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(workflowWireProbe)
	env.ExecuteWorkflow(workflowWireProbe)
	require.NoError(t, env.GetWorkflowError())

	var got wireProbeResult
	require.NoError(t, env.GetWorkflowResult(&got))
	require.Equal(t, "imageagent.execute_slot.v3", got.ExecuteSlot)
}

func TestNewApprovalUsesActionIDV3DigestAndPublishV3(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(workflowWireProbe)
	env.ExecuteWorkflow(workflowWireProbe)
	require.NoError(t, env.GetWorkflowError())

	var got wireProbeResult
	require.NoError(t, env.GetWorkflowResult(&got))
	require.Equal(t, "imageagent.publish_approved.v3", got.PublishApproved)
	require.Equal(t, "capture-action", got.ApprovalActionID)
	plan, results := wireProbePlanAndResults()
	wantDigest, err := imageagent.ResultDigestV3(plan, slotProjections(plan, results))
	require.NoError(t, err)
	require.Equal(t, wantDigest, got.ResultDigest)
}

func wireProbePlanAndResults() (imageagent.Plan, []SlotWorkflowResult) {
	const sha = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	plan := imageagent.Plan{Revision: 1, Slots: []imageagent.Slot{{ID: "slot-1", Role: imageagent.SlotRoleScene}}}
	results := []SlotWorkflowResult{{
		Execution: imageagent.SlotExecutionResult{SlotID: "slot-1", Attempt: 1, Candidates: []imageagent.AssetCandidate{{
			AssetID: "candidate-1", DurableAsset: imageagent.DurableAssetIdentity{ObjectKey: "image-agent/public/tenant-a/capture-run/1/slot-1/1/0-" + sha + ".png", SHA256: sha},
		}}},
		Status: imageagent.SlotStatusAccepted,
	}}
	return plan, results
}

func TestWorkerRegistersFrozenV2AndV3Activities(t *testing.T) {
	activities, err := NewActivities(ActivityDependencies{
		Repository: store.NewMemoryRepository(), SlotExecutor: &identityCheckingExecutor{t: t}, Publisher: &identityCheckingPublisher{t: t},
	})
	require.NoError(t, err)
	registrar := &recordingWorkerRegistrar{}
	require.NoError(t, RegisterWorker(registrar, activities))
	require.Contains(t, registrar.activities, "imageagent.execute_slot.v2")
	require.Contains(t, registrar.activities, "imageagent.publish_approved.v2")
	require.Contains(t, registrar.activities, "imageagent.execute_slot.v3")
	require.Contains(t, registrar.activities, "imageagent.publish_approved.v3")
}

func readHistoryFixture(t *testing.T, name string) *historypb.History {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	history := &historypb.History{}
	require.NoError(t, protojson.Unmarshal(raw, history))
	return history
}

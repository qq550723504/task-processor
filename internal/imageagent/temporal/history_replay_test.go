package temporal

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	historypb "go.temporal.io/api/history/v1"
	sdkactivity "go.temporal.io/sdk/activity"
	sdkconverter "go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/testsuite"
	sdkworker "go.temporal.io/sdk/worker"
	sdkworkflow "go.temporal.io/sdk/workflow"
	"google.golang.org/protobuf/encoding/protojson"

	"task-processor/internal/imageagent"
)

func TestReplayV2SlotInflightHistory(t *testing.T) {
	replayer := sdkworker.NewWorkflowReplayer()
	replayer.RegisterWorkflowWithOptions(ImageSlotWorkflow, sdkworkflow.RegisterOptions{Name: workflowNameImageSlot})

	require.NoError(t, replayer.ReplayWorkflowHistory(nil, readHistoryFixture(t, "v2-slot-inflight.json")))
}

func TestOldV2ParentCanStartFreshSlotChildWithExactV2Registrations(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(oldV2ParentContinuationProbe)
	env.RegisterWorkflowWithOptions(ImageSlotWorkflow, sdkworkflow.RegisterOptions{Name: workflowNameImageSlot})
	env.RegisterActivityWithOptions(
		func(_ context.Context, input ExecuteSlotActivityInput) (imageagent.SlotExecutionResult, error) {
			return imageagent.SlotExecutionResult{
				SlotID: input.Slot.ID, Attempt: input.Attempt,
				Candidates: []imageagent.AssetCandidate{{AssetID: "candidate-1"}},
			}, nil
		},
		sdkactivity.RegisterOptions{Name: activityExecuteSlot},
	)
	input := SlotWorkflowInput{
		RunID: "run-old-v2-parent", Identity: imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a"},
		PlanRevision: 1, Slot: imageagent.Slot{ID: "scene-1", Role: imageagent.SlotRoleScene, IdempotencyKey: "scene-key"}, Attempt: 1,
	}

	env.ExecuteWorkflow(oldV2ParentContinuationProbe, input)

	require.NoError(t, env.GetWorkflowError())
	var result SlotWorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, imageagent.SlotStatusAccepted, result.Status)
	require.Empty(t, result.ErrorCode)
}

func oldV2ParentContinuationProbe(ctx sdkworkflow.Context, input SlotWorkflowInput) (SlotWorkflowResult, error) {
	var result SlotWorkflowResult
	err := sdkworkflow.ExecuteChildWorkflow(ctx, workflowNameImageSlot, input).Get(ctx, &result)
	return result, err
}

func TestReplayV2AwaitingApprovalHistory(t *testing.T) {
	replayer := sdkworker.NewWorkflowReplayer()
	replayer.RegisterWorkflowWithOptions(ImageAgentWorkflow, sdkworkflow.RegisterOptions{Name: workflowNameImageAgent})
	replayer.RegisterWorkflowWithOptions(ImageSlotWorkflow, sdkworkflow.RegisterOptions{Name: workflowNameImageSlot})

	require.NoError(t, replayer.ReplayWorkflowHistory(nil, readHistoryFixture(t, "v2-awaiting-approval.json")))
}

func TestReplayV2PreAtomicAwaitingApprovalHistory(t *testing.T) {
	history := readHistoryFixture(t, "v2-pre-atomic-awaiting-approval.json")
	require.NotContains(t, historyVersionChangeIDs(t, history), activityWireV2Patch)

	replayer := sdkworker.NewWorkflowReplayer()
	replayer.RegisterWorkflowWithOptions(ImageAgentWorkflow, sdkworkflow.RegisterOptions{Name: workflowNameImageAgent})
	replayer.RegisterWorkflowWithOptions(ImageSlotWorkflow, sdkworkflow.RegisterOptions{Name: workflowNameImageSlot})

	require.NoError(t, replayer.ReplayWorkflowHistory(nil, history))
}

func historyVersionChangeIDs(t *testing.T, history *historypb.History) []string {
	t.Helper()
	var changeIDs []string
	for _, event := range history.Events {
		attributes := event.GetMarkerRecordedEventAttributes()
		if attributes == nil || attributes.GetMarkerName() != "Version" {
			continue
		}
		payloads := attributes.GetDetails()["change-id"]
		if payloads == nil {
			continue
		}
		var changeID string
		require.NoError(t, sdkconverter.GetDefaultDataConverter().FromPayloads(payloads, &changeID))
		changeIDs = append(changeIDs, changeID)
	}
	return changeIDs
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

func TestTask6MarkersAreEvaluatedBeforePreAtomicSelection(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(workflowWireProbe)
	slot := env.OnGetVersion(slotExecutionWireV3Patch, sdkworkflow.DefaultVersion, 1).Return(sdkworkflow.DefaultVersion).Once()
	action := env.OnGetVersion(approvalActionIDV3Patch, sdkworkflow.DefaultVersion, 1).Return(sdkworkflow.DefaultVersion).Once()
	publication := env.OnGetVersion(approvalPublicationWireV3Patch, sdkworkflow.DefaultVersion, 1).Return(sdkworkflow.DefaultVersion).Once()
	digest := env.OnGetVersion(resultDigestV3Patch, sdkworkflow.DefaultVersion, 1).Return(sdkworkflow.DefaultVersion).Once()
	scope := env.OnGetVersion(approvalPublicationScopePatch, sdkworkflow.DefaultVersion, 1).Return(sdkworkflow.DefaultVersion).Once()
	env.OnGetVersion(activityWireV2Patch, sdkworkflow.DefaultVersion, 1).
		Return(sdkworkflow.DefaultVersion).Once().NotBefore(slot, action, publication, digest, scope)

	env.ExecuteWorkflow(workflowWireProbe)
	require.NoError(t, env.GetWorkflowError())
	require.True(t, env.AssertExpectations(t))
}

type approvalProtocolProbeResult struct {
	PublishActivity string
	IdempotencyKey  string
	ResultDigest    string
}

func approvalProtocolProbe(ctx sdkworkflow.Context) (approvalProtocolProbeResult, error) {
	ctx = imageAgentActivityContext(ctx)
	effects := newWorkflowEffectOwner(ctx)
	plan, results := wireProbePlanAndResults()
	digest, err := resultDigestForWire(plan, results, effects.activities)
	if err != nil {
		return approvalProtocolProbeResult{}, err
	}
	key := approvalPublicationKeyForWire("capture-action", "capture-run", 1, effects.activities)
	err = effects.publishApproved(ctx, PublishApprovedActivityInput{
		RunID: "capture-run", Identity: imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a"},
		PlanRevision: 1, CandidateAssetIDs: []string{"candidate-1"}, IdempotencyKey: key,
	})
	if err != nil {
		return approvalProtocolProbeResult{}, err
	}
	return approvalProtocolProbeResult{PublishActivity: effects.activities.publishApproved, IdempotencyKey: key, ResultDigest: digest}, nil
}

func TestApprovalMarkerCombinationsSelectOnlyCompleteProtocols(t *testing.T) {
	for mask := 0; mask < 8; mask++ {
		actionV3 := mask&1 != 0
		publicationV3 := mask&2 != 0
		digestV3 := mask&4 != 0
		name := fmt.Sprintf("action=%t/publication=%t/digest=%t", actionV3, publicationV3, digestV3)
		t.Run(name, func(t *testing.T) {
			var suite testsuite.WorkflowTestSuite
			env := suite.NewTestWorkflowEnvironment()
			env.RegisterWorkflow(approvalProtocolProbe)
			env.RegisterActivityWithOptions(
				func(context.Context, PublishApprovedActivityInput) error { return nil },
				sdkactivity.RegisterOptions{Name: activityPublishApproved},
			)
			env.RegisterActivityWithOptions(
				func(context.Context, PublishApprovedV3ActivityInput) error { return nil },
				sdkactivity.RegisterOptions{Name: activityPublishApprovedV3},
			)
			env.OnGetVersion(slotExecutionWireV3Patch, sdkworkflow.DefaultVersion, 1).Return(sdkworkflow.DefaultVersion).Once()
			env.OnGetVersion(approvalActionIDV3Patch, sdkworkflow.DefaultVersion, 1).Return(markerVersion(actionV3)).Once()
			env.OnGetVersion(approvalPublicationWireV3Patch, sdkworkflow.DefaultVersion, 1).Return(markerVersion(publicationV3)).Once()
			env.OnGetVersion(resultDigestV3Patch, sdkworkflow.DefaultVersion, 1).Return(markerVersion(digestV3)).Once()
			env.OnGetVersion(approvalPublicationScopePatch, sdkworkflow.DefaultVersion, 1).Return(sdkworkflow.DefaultVersion).Once()
			env.OnGetVersion(activityWireV2Patch, sdkworkflow.DefaultVersion, 1).Return(sdkworkflow.Version(1)).Once()

			allV3 := actionV3 && publicationV3 && digestV3
			wantActivity := activityPublishApproved
			wantKey := publicationKey("capture-run", 1)
			plan, results := wireProbePlanAndResults()
			wantDigest, err := imageagent.ResultDigestV2(plan, slotProjections(plan, results))
			require.NoError(t, err)
			if allV3 {
				wantActivity = activityPublishApprovedV3
				wantKey = "capture-action"
				wantDigest, err = imageagent.ResultDigestV3(plan, slotProjections(plan, results))
				require.NoError(t, err)
				env.OnActivity(wantActivity, mock.Anything, mock.MatchedBy(func(input PublishApprovedV3ActivityInput) bool {
					return input.IdempotencyKey == wantKey
				})).Return(nil).Once()
			} else {
				env.OnActivity(wantActivity, mock.Anything, mock.MatchedBy(func(input PublishApprovedActivityInput) bool {
					return input.IdempotencyKey == wantKey
				})).Return(nil).Once()
			}

			env.ExecuteWorkflow(approvalProtocolProbe)
			require.NoError(t, env.GetWorkflowError())
			var got approvalProtocolProbeResult
			require.NoError(t, env.GetWorkflowResult(&got))
			require.Equal(t, approvalProtocolProbeResult{PublishActivity: wantActivity, IdempotencyKey: wantKey, ResultDigest: wantDigest}, got)
			require.True(t, env.AssertExpectations(t))
		})
	}
}

func markerVersion(enabled bool) sdkworkflow.Version {
	if enabled {
		return sdkworkflow.Version(1)
	}
	return sdkworkflow.DefaultVersion
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
		ApprovalActionID: approvalPublicationKeyForWire("capture-action", "capture-run", 1, wire),
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

func TestNewApprovalScopesActionIDByRunAndRevisionWithV3DigestAndPublishV3(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(workflowWireProbe)
	env.ExecuteWorkflow(workflowWireProbe)
	require.NoError(t, env.GetWorkflowError())

	var got wireProbeResult
	require.NoError(t, env.GetWorkflowResult(&got))
	require.Equal(t, "imageagent.publish_approved.v3", got.PublishApproved)
	require.Equal(t, approvalActionPublicationKey("capture-action", "capture-run", 1), got.ApprovalActionID)
	require.NotEqual(t, got.ApprovalActionID, approvalActionPublicationKey("capture-action", "other-run", 1))
	require.NotEqual(t, got.ApprovalActionID, approvalActionPublicationKey("capture-action", "capture-run", 2))
	plan, results := wireProbePlanAndResults()
	wantDigest, err := imageagent.ResultDigestV3(plan, slotProjections(plan, results))
	require.NoError(t, err)
	require.Equal(t, wantDigest, got.ResultDigest)
}

func TestRunScopedApprovalPublicationKeyFitsReceiptSchema(t *testing.T) {
	key := approvalActionPublicationKey(strings.Repeat("a", imageagent.MaxActionIDLength), strings.Repeat("r", 64), imageagent.MaxJSONSafePlanRevision)
	require.LessOrEqual(t, len(key), 192)
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

func readHistoryFixture(t *testing.T, name string) *historypb.History {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	history := &historypb.History{}
	require.NoError(t, protojson.Unmarshal(raw, history))
	return history
}
